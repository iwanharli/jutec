package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"jutec-go/internal/crawler"
	"jutec-go/internal/probeio"
	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

// Mode Pantau: jendela Chrome terlihat yang dikendalikan analis. Sesi disimpan di memori per probe_id.
var (
	attendedMu  sync.Mutex
	attendedSes = map[int]*crawler.AttendedSession{}
)

func init() {
	// Tutup sesi yang ditinggalkan > 20 menit agar jendela & proses Chrome tidak menumpuk
	go func() {
		for range time.Tick(2 * time.Minute) {
			attendedMu.Lock()
			for id, s := range attendedSes {
				if s.Expired(20 * time.Minute) {
					s.Close()
					delete(attendedSes, id)
				}
			}
			attendedMu.Unlock()
		}
	}()
}

// POST /sites/:id/form-probe/attended — buka jendela Chrome, isi data dummy, tunggu analis.
func (h *FormProbeHandler) AttendedStart(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	var body struct {
		ProfileID int  `json:"profile_id"`
		Force     bool `json:"force"`
	}
	c.ShouldBindJSON(&body)
	prof, err := h.PG.GetProbeProfile(ctx, body.ProfileID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Belum ada data dummy."})
		return
	}
	d, err := h.PG.GetSiteDetail(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "situs tidak ditemukan"})
		return
	}
	pageURL, action, _, _ := store.PickAnyRegisterForm(mustJSON(d.Site.AuthSurface))
	if pageURL == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Tidak ada halaman pendaftaran yang terbaca di situs ini."})
		return
	}
	username := randUser(prof.UsernamePrefix)
	pid, err := h.PG.CreateFormProbe(ctx, id, pageURL, username, ActorOf(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.PG.SetFormProbeProfile(ctx, pid, prof.ID)

	profile := crawler.ProbeProfile{Username: username, Password: prof.Password, FullName: prof.FullName, Email: prof.Email, Phone: prof.Phone}
	json.Unmarshal([]byte(h.PG.GetSetting(ctx, "probe_accounts")), &profile.Accounts)
	var own []crawler.ProbeAccount
	if json.Unmarshal(prof.Accounts, &own) == nil {
		profile.Accounts = append(profile.Accounts, own...)
	}

	// Buka jendela (bisa lambat — tunggu sampai terisi)
	sess, err := crawler.NewCrawler().OpenAttended(pageURL, action, profile)
	if err != nil {
		h.PG.FinishFormProbe(ctx, pid, "error", "", "", err.Error(), "", map[string]any{}, []any{})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	attendedMu.Lock()
	attendedSes[pid] = sess
	attendedMu.Unlock()
	c.Set("audit_target", fmt.Sprintf("site:%d", id))
	c.JSON(http.StatusOK, gin.H{"probe_id": pid, "username": username, "filled": sess.Filled, "page_url": pageURL})
}

// POST /form-probes/:pid/attended/capture — ambil keadaan jendela saat ini, simpan, tutup.
func (h *FormProbeHandler) AttendedCapture(c *gin.Context) {
	pid, _ := strconv.Atoi(c.Param("pid"))
	attendedMu.Lock()
	sess := attendedSes[pid]
	delete(attendedSes, pid)
	attendedMu.Unlock()
	if sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sesi pantau tidak ada atau sudah ditutup"})
		return
	}
	pr, _ := h.PG.GetFormProbe(c.Request.Context(), pid)
	res := sess.Capture(crawler.NewCrawler())
	sess.Close()
	siteID := 0
	if pr != nil {
		siteID = pr.SiteID
	}
	probeio.Save(c.Request.Context(), h.PG, h.BaseDir, pid, siteID, res)
	c.JSON(http.StatusOK, gin.H{"status": res.Status, "outcome": res.Outcome, "message": res.Message})
}

// POST /form-probes/:pid/attended/cancel — tutup jendela tanpa menyimpan.
func (h *FormProbeHandler) AttendedCancel(c *gin.Context) {
	pid, _ := strconv.Atoi(c.Param("pid"))
	attendedMu.Lock()
	sess := attendedSes[pid]
	delete(attendedSes, pid)
	attendedMu.Unlock()
	if sess != nil {
		sess.Close()
	}
	h.PG.FinishFormProbe(c.Request.Context(), pid, "skipped", "", "", "dibatalkan analis (Mode Pantau)", "", map[string]any{}, []any{})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
