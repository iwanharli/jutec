package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	neturl "net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

// FormProbeHandler: Uji Form Terkendali — dijalankan manual oleh analis, satu situs per uji.
type FormProbeHandler struct {
	PG      *store.PostgresDB
	Tasks   *asynq.Client
	BaseDir string
}

// POST /sites/:id/form-probe — antrekan uji daftar + login dengan data dummy dari Pengaturan.
func (h *FormProbeHandler) Start(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	if h.Tasks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "antrean latar belakang tidak aktif"})
		return
	}
	var body struct {
		ProfileID int  `json:"profile_id"`
		Force     bool `json:"force"` // uji ulang walau profil sudah berhasil di backend ini
	}
	c.ShouldBindJSON(&body)
	prof, err := h.PG.GetProbeProfile(ctx, body.ProfileID) // 0 = identitas bawaan
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Belum ada data dummy. Tambahkan identitas di halaman Form Daftar & Login → Data Dummy."})
		return
	}
	d, err := h.PG.GetSiteDetail(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "situs tidak ditemukan"})
		return
	}
	pageURL, action, login, skipped := store.PickRegisterForm(mustJSON(d.Site.AuthSurface))
	if pageURL == "" {
		msg := "Tidak ada form pendaftaran yang terbaca di situs ini."
		if skipped {
			msg = "Form pendaftaran memakai captcha/OTP — dilewati karena tidak bisa diotomasi."
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
		return
	}
	if !body.Force && h.PG.ProfileProbedFormHosts(ctx, prof.ID)[formHost(pageURL)] {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Identitas “%s” sudah berhasil mendaftar di backend ini (%s). Pakai identitas lain, atau “Uji ulang (paksa)”.", prof.Label, formHost(pageURL)), "can_force": true})
		return
	}
	username := fmt.Sprintf("%s%06d", prof.UsernamePrefix, rand.Intn(1000000))
	by := ActorOf(c)
	pid, err := h.PG.CreateFormProbe(ctx, id, pageURL, username, by)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.PG.SetFormProbeProfile(ctx, pid, prof.ID)
	payload, _ := json.Marshal(map[string]any{"probe_id": pid, "action": action, "login_url": login})
	if _, err := h.Tasks.Enqueue(asynq.NewTask("form:probe", payload, asynq.MaxRetry(0)), asynq.Queue("default"), asynq.Timeout(5*60*1e9)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Set("audit_target", fmt.Sprintf("site:%d", id))
	c.JSON(http.StatusAccepted, gin.H{"probe_id": pid, "username": username, "page_url": pageURL, "profile": prof.Label})
}

// GET /sites/:id/form-probes
func (h *FormProbeHandler) List(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	r, err := h.PG.ListFormProbes(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"probes": r})
}

// GET /form-probes/:pid — detail satu uji (respons, platform, artefak, berkas)
func (h *FormProbeHandler) Detail(c *gin.Context) {
	pid, _ := strconv.Atoi(c.Param("pid"))
	p, err := h.PG.GetFormProbe(c.Request.Context(), pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tidak ada"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// GET /form-probes/:pid/files/:name — berkas bukti uji (hanya yang tercatat).
func (h *FormProbeHandler) File(c *gin.Context) {
	pid, _ := strconv.Atoi(c.Param("pid"))
	p, err := h.PG.GetFormProbe(c.Request.Context(), pid)
	if err != nil || p.Dir == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "tidak ada"})
		return
	}
	var files []struct {
		Name string `json:"name"`
	}
	json.Unmarshal(p.Files, &files)
	name := c.Param("name")
	for _, f := range files {
		if f.Name == name {
			c.File(filepath.Join(p.Dir, name))
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "berkas tidak tercatat"})
}

// GET /data/probe-accounts?q= — akun dummy yang berhasil terdaftar
func (h *FormProbeHandler) Accounts(c *gin.Context) {
	r, err := h.PG.ProbeAccounts(c.Request.Context(), c.Query("q"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": r})
}

// ── Identitas dummy tersimpan ──

// GET /probe-profiles
func (h *FormProbeHandler) Profiles(c *gin.Context) {
	r, err := h.PG.ListProbeProfiles(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": r})
}

// POST /probe-profiles · PUT /probe-profiles/:pid
func (h *FormProbeHandler) SaveProfile(c *gin.Context) {
	var p store.ProbeProfileRow
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if c.Param("pid") != "" {
		p.ID, _ = strconv.Atoi(c.Param("pid"))
	} else {
		p.ID = 0
	}
	if p.UsernamePrefix == "" || p.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Awalan username & password wajib diisi"})
		return
	}
	if p.Label == "" {
		p.Label = "Identitas " + p.UsernamePrefix
	}
	id, err := h.PG.SaveProbeProfile(c.Request.Context(), &p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Set("audit_target", fmt.Sprintf("probe_profile:%d", id))
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// POST /probe-profiles/:pid/default
func (h *FormProbeHandler) DefaultProfile(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("pid"))
	if err := h.PG.SetDefaultProbeProfile(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DELETE /probe-profiles/:pid
func (h *FormProbeHandler) DeleteProfile(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("pid"))
	if err := h.PG.DeleteProbeProfile(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Set("audit_target", fmt.Sprintf("probe_profile:%d", id))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func formHost(raw string) string {
	u, err := neturl.Parse(raw)
	if err != nil {
		return raw
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

func randUser(prefix string) string { return fmt.Sprintf("%s%06d", prefix, rand.Intn(1000000)) }

// batchTargets: kandidat uji massal (situs judol valid dengan form daftar tanpa captcha/OTP).
func (h *FormProbeHandler) batchTargets(ctx context.Context, profileID int) ([]store.BatchProbeTarget, error) {
	cands, err := h.PG.BatchProbeCandidates(ctx, profileID)
	if err != nil {
		return nil, err
	}
	probed := h.PG.ProfileProbedFormHosts(ctx, profileID)
	seen := map[string]bool{}
	var out []store.BatchProbeTarget
	for _, c := range cands {
		page, action, login, _ := store.PickRegisterForm(c.Auth)
		if page == "" {
			continue
		}
		// Dedup per host halaman pendaftaran: banyak domain judol berbagi satu backend daftar
		host := formHost(page)
		if probed[host] || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, store.BatchProbeTarget{SiteID: c.ID, URL: c.URL, PageURL: page, Action: action, Login: login})
	}
	return out, nil
}

// captchaTargets: situs judol valid yang form daftarnya butuh captcha/OTP (dilewati uji otomatis) —
// ditampilkan agar analis mengerjakannya manual lewat Mode Pantau. Dedup per host daftar.
func (h *FormProbeHandler) captchaTargets(ctx context.Context, profileID int) ([]store.BatchProbeTarget, error) {
	cands, err := h.PG.BatchProbeCandidates(ctx, profileID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []store.BatchProbeTarget
	for _, c := range cands {
		auto, _, _, _ := store.PickRegisterForm(c.Auth)
		page, action, login, hasCaptcha := store.PickAnyRegisterForm(c.Auth)
		if auto != "" || page == "" || !hasCaptcha { // sudah tercakup uji otomatis, atau tanpa captcha
			continue
		}
		host := formHost(page)
		if seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, store.BatchProbeTarget{SiteID: c.ID, URL: c.URL, PageURL: page, Action: action, Login: login})
	}
	return out, nil
}

// GET /form-probe-batch/preview?cooldown_days=3 — berapa situs yang akan diuji
func (h *FormProbeHandler) BatchPreview(c *gin.Context) {
	ctx := c.Request.Context()
	prof, err := h.PG.GetProbeProfile(ctx, atoiDefault(c.Query("profile_id"), 0))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"total": 0, "sample": []any{}})
		return
	}
	full, err := h.batchTargets(ctx, prof.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sample := full
	if len(sample) > 12 {
		sample = sample[:12]
	}
	captcha, _ := h.captchaTargets(ctx, prof.ID)
	c.JSON(http.StatusOK, gin.H{"total": len(full), "sample": sample, "captcha": captcha, "profile_id": prof.ID})
}

// POST /form-probe-batch — jalankan uji massal
// body: {profile_id, limit, delay_sec, cooldown_days}
func (h *FormProbeHandler) BatchStart(c *gin.Context) {
	if h.Tasks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "antrean latar belakang tidak aktif"})
		return
	}
	var body struct {
		ProfileID int `json:"profile_id"`
		DelaySec  int `json:"delay_sec"`
	}
	c.ShouldBindJSON(&body)
	ctx := c.Request.Context()
	if b, err := h.PG.LatestBatch(ctx); err == nil && b.Status == "running" {
		c.JSON(http.StatusConflict, gin.H{"error": "Masih ada uji massal berjalan.", "batch_id": b.ID})
		return
	}
	prof, err := h.PG.GetProbeProfile(ctx, body.ProfileID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Belum ada data dummy."})
		return
	}
	if body.DelaySec < 10 {
		body.DelaySec = 25
	}
	targets, err := h.batchTargets(ctx, prof.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Pengaman teknis: maksimal 200 situs sekali jalan
	if len(targets) > 200 {
		targets = targets[:200]
	}
	if len(targets) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Tidak ada situs yang perlu diuji."})
		return
	}
	by := ActorOf(c)
	batchID, err := h.PG.CreateBatch(ctx, prof.ID, len(targets), body.DelaySec, by)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	queued := 0
	for i, t := range targets {
		pid, err := h.PG.CreateBatchProbe(ctx, batchID, t.SiteID, prof.ID, t.PageURL, randUser(prof.UsernamePrefix), by)
		if err != nil {
			continue
		}
		payload, _ := json.Marshal(map[string]any{"probe_id": pid, "action": t.Action, "login_url": t.Login})
		// Antre pelan: 1 situs per delay_sec detik
		if _, err := h.Tasks.Enqueue(asynq.NewTask("form:probe", payload, asynq.MaxRetry(0)),
			asynq.Queue("default"), asynq.ProcessIn(time.Duration(i*body.DelaySec)*time.Second), asynq.Timeout(5*60*1e9)); err == nil {
			queued++
		}
	}
	c.Set("audit_target", fmt.Sprintf("probe_batch:%d", batchID))
	c.JSON(http.StatusAccepted, gin.H{"batch_id": batchID, "queued": queued, "delay_sec": body.DelaySec, "profile": prof.Label})
}

// GET /form-probe-batch/:bid
func (h *FormProbeHandler) BatchStatus(c *gin.Context) {
	bid, _ := strconv.Atoi(c.Param("bid"))
	b, err := h.PG.BatchStatus(c.Request.Context(), bid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tidak ada"})
		return
	}
	c.JSON(http.StatusOK, b)
}

// GET /form-probe-batch/latest
func (h *FormProbeHandler) BatchLatest(c *gin.Context) {
	b, err := h.PG.LatestBatch(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"batch": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"batch": b})
}

// POST /form-probe-batch/:bid/stop
func (h *FormProbeHandler) BatchStop(c *gin.Context) {
	bid, _ := strconv.Atoi(c.Param("bid"))
	if err := h.PG.StopBatch(c.Request.Context(), bid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Set("audit_target", fmt.Sprintf("probe_batch:%d", bid))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func atoiDefault(s string, d int) int {
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return d
}
