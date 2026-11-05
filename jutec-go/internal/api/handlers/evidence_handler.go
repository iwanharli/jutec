package handlers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"jutec-go/internal/evidence"
	"jutec-go/internal/store"
	"jutec-go/internal/worker/tasks"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

type EvidenceHandler struct {
	PG      *store.PostgresDB
	Tasks   *asynq.Client
	BaseDir string
}

var reActor = regexp.MustCompile(`[^\p{L}\p{N} ._@-]`)

// ActorOf: nama pengguna dari header X-Jutec-User. Ini PENANDA, bukan autentikasi.
func ActorOf(c *gin.Context) string {
	raw := c.GetHeader("X-Jutec-User")
	if raw == "" {
		raw = c.Query("by") // <img src> tak bisa mengirim header
	}
	a := strings.TrimSpace(reActor.ReplaceAllString(raw, ""))
	if len(a) > 64 {
		a = a[:64]
	}
	if a == "" {
		return "anonymous"
	}
	return a
}

// POST /sites/:id/evidence — antrekan pengambilan bukti forensik.
func (h *EvidenceHandler) Request(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	var url string
	if h.PG.Pool.QueryRow(c, `SELECT url FROM sites WHERE id = $1`, id).Scan(&url) != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "situs tidak ditemukan"})
		return
	}
	if h.Tasks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "worker tidak aktif"})
		return
	}
	actor := ActorOf(c)
	capID, err := h.PG.CreateEvidenceCapture(c, id, url, "", actor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	payload, _ := json.Marshal(tasks.EvidencePayload{CaptureID: capID, SiteID: id, URL: url, RequestedBy: actor})
	if _, err := h.Tasks.Enqueue(asynq.NewTask(tasks.TypeEvidenceCapture, payload, asynq.MaxRetry(0), asynq.Timeout(4*time.Minute)),
		asynq.Queue("default")); err != nil {
		h.PG.FailEvidenceCapture(c, capID, "gagal diantrekan: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Set("audit_target", fmt.Sprintf("evidence:%d", capID))
	c.JSON(http.StatusAccepted, gin.H{"capture_id": capID, "message": "Pengambilan bukti diantrekan (±20–60 detik)."})
}

// GET /sites/:id/evidence
func (h *EvidenceHandler) List(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	list, err := h.PG.ListEvidence(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"captures": list})
}

func (h *EvidenceHandler) capture(c *gin.Context) (*store.EvidenceCapture, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id tidak valid"})
		return nil, false
	}
	ec, err := h.PG.GetEvidence(c, id)
	if err != nil || ec == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "bukti tidak ditemukan"})
		return nil, false
	}
	if ec.Status != "done" {
		c.JSON(http.StatusConflict, gin.H{"error": "pengambilan bukti belum selesai / gagal"})
		return nil, false
	}
	// Pastikan folder tetap di dalam BaseDir (cegah path traversal)
	if rel, err := filepath.Rel(h.BaseDir, ec.Dir); err != nil || strings.HasPrefix(rel, "..") {
		c.JSON(http.StatusForbidden, gin.H{"error": "lokasi bukti di luar folder bukti"})
		return nil, false
	}
	return ec, true
}

// GET /evidence/:id/verify — hitung ulang hash setiap file & cocokkan dengan catatan.
func (h *EvidenceHandler) Verify(c *gin.Context) {
	ec, ok := h.capture(c)
	if !ok {
		return
	}
	type fileCheck struct {
		Name     string `json:"name"`
		Expected string `json:"expected"`
		Actual   string `json:"actual"`
		OK       bool   `json:"ok"`
		Error    string `json:"error,omitempty"`
	}
	all := true
	checks := []fileCheck{}
	for _, f := range ec.Files {
		sum, size, err := evidence.HashFile(filepath.Join(ec.Dir, f.Name))
		fc := fileCheck{Name: f.Name, Expected: f.SHA256, Actual: sum, OK: err == nil && sum == f.SHA256 && size == f.Size}
		if err != nil {
			fc.Error = "file hilang / tidak terbaca"
		}
		all = all && fc.OK
		checks = append(checks, fc)
	}
	manifestOK := false
	for _, f := range ec.Files {
		if f.Name == "manifest.json" {
			manifestOK = f.SHA256 == ec.ManifestSHA256
		}
	}
	c.Set("audit_target", fmt.Sprintf("evidence:%d", ec.ID))
	c.JSON(http.StatusOK, gin.H{"capture_id": ec.ID, "intact": all && manifestOK, "manifest_ok": manifestOK, "files": checks, "checked_at": time.Now().UTC()})
}

// GET /evidence/:id/files/:name — unduh satu file (hanya yang tercatat).
func (h *EvidenceHandler) File(c *gin.Context) {
	ec, ok := h.capture(c)
	if !ok {
		return
	}
	name := c.Param("name")
	// Bukti baru menyimpan screenshot sebagai JPEG; tautan lama "screenshot-*.png" tetap berlaku
	alt := ""
	if strings.HasPrefix(name, "screenshot-") {
		if strings.HasSuffix(name, ".png") {
			alt = strings.TrimSuffix(name, ".png") + ".jpg"
		} else if strings.HasSuffix(name, ".jpg") {
			alt = strings.TrimSuffix(name, ".jpg") + ".png"
		}
	}
	for _, f := range ec.Files {
		if f.Name == name || (alt != "" && f.Name == alt) {
			name = f.Name
			c.Set("audit_target", fmt.Sprintf("evidence:%d", ec.ID))
			inline := (strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg")) && c.Query("download") == ""
			if !inline {
				c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="bukti-%d-%s"`, ec.ID, name))
			}
			c.Header("X-Content-SHA256", f.SHA256)
			c.File(filepath.Join(ec.Dir, f.Name))
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "file tidak ada di catatan bukti"})
}

// GET /evidence/:id/zip — paket bukti: semua file + SHA256SUMS + KETERANGAN.txt.
func (h *EvidenceHandler) Zip(c *gin.Context) {
	ec, ok := h.capture(c)
	if !ok {
		return
	}
	c.Set("audit_target", fmt.Sprintf("evidence:%d", ec.ID))
	host := strings.TrimPrefix(strings.TrimPrefix(ec.RequestedURL, "https://"), "http://")
	host = regexp.MustCompile(`[^a-zA-Z0-9.-]`).ReplaceAllString(host, "_")
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="bukti-%d-%s.zip"`, ec.ID, host))
	zw := zip.NewWriter(c.Writer)
	defer zw.Close()

	var sums strings.Builder
	for _, f := range ec.Files {
		src, err := os.Open(filepath.Join(ec.Dir, f.Name))
		if err != nil {
			continue
		}
		w, _ := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: derefTime(ec.CapturedAt)})
		io.Copy(w, src)
		src.Close()
		fmt.Fprintf(&sums, "%s  %s\n", f.SHA256, f.Name)
	}
	w, _ := zw.Create("SHA256SUMS")
	io.WriteString(w, sums.String())

	w, _ = zw.Create("KETERANGAN.txt")
	fmt.Fprintf(w, `PAKET BUKTI ELEKTRONIK — Jutec
=================================
ID pengambilan   : %d
ID situs         : %d
URL diminta      : %s
URL akhir        : %s
Status HTTP      : %d
IP server        : %s
Rantai redirect  : %s
User-Agent       : %s
Waktu ambil (UTC): %s
Diminta oleh     : %s
Hash manifest    : %s

Cara memeriksa keutuhan:
  1. Ekstrak paket ini.
  2. Jalankan:  sha256sum -c SHA256SUMS   (macOS: shasum -a 256 -c SHA256SUMS)
  3. Semua file harus berstatus OK. Hash manifest.json harus sama dengan "Hash manifest" di atas,
     yang juga tercatat di log audit aplikasi.

manifest.json berisi daftar hash setiap file pada saat diambil, sehingga perubahan pada file
mana pun akan terdeteksi.
`, ec.ID, ec.SiteID, ec.RequestedURL, ec.FinalURL, ec.HTTPStatus, ec.ServerIP, strings.Join(ec.Redirects, " → "),
		ec.UserAgent, derefTime(ec.CapturedAt).Format(time.RFC3339), ec.RequestedBy, ec.ManifestSHA256)
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
