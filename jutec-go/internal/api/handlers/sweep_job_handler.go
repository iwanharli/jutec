package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"jutec-go/internal/service"
	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

type SweepJobHandler struct {
	DiscoverySvc *service.DiscoveryService
	PG           *store.PostgresDB
	Enricher     *service.AutoEnricher
}

func NewSweepJobHandler(svc *service.DiscoveryService, pg *store.PostgresDB) *SweepJobHandler {
	return &SweepJobHandler{DiscoverySvc: svc, PG: pg}
}

type StartJobRequest struct {
	Keywords    []string `json:"keywords"`
	UseDefaults bool     `json:"use_defaults"`
	MaxDepth    int      `json:"max_depth"`
	MaxURLs     int      `json:"max_urls"`
}

// POST /sweep/start — mulai sweep sebagai background job
func (h *SweepJobHandler) Start(c *gin.Context) {
	var req StartJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	keywords := req.Keywords
	if req.UseDefaults {
		keywords = append(keywords, h.DiscoverySvc.LoadDefaultKeywords(c.Request.Context())...)
	}

	// Deduplicate
	seen := map[string]bool{}
	var unique []string
	for _, kw := range keywords {
		if !seen[kw] {
			seen[kw] = true
			unique = append(unique, kw)
		}
	}
	if len(unique) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide at least one keyword or set use_defaults: true"})
		return
	}

	jobID, activeID, err := h.launch(c.Request.Context(), unique, service.DeepSweepConfig{MaxDepth: req.MaxDepth, MaxURLs: req.MaxURLs})
	if activeID != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "job already running or paused", "job_id": activeID})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job_id": jobID, "status": "running"})
}

// launch memulai Deep Sweep sebagai job latar (dipakai tombol manual & jadwal harian).
// activeID terisi bila sudah ada job yang berjalan/dijeda — job baru tidak dibuat.
func (h *SweepJobHandler) launch(ctx context.Context, keywords []string, cfg service.DeepSweepConfig) (jobID, activeID string, err error) {
	if active, e := h.PG.GetActiveSweepJob(ctx); e == nil && active != nil {
		return "", active.ID, nil
	}
	jobID = fmt.Sprintf("sweep-%d", time.Now().UnixMilli())
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 3
	}
	if cfg.MaxURLs <= 0 {
		cfg.MaxURLs = 500
	}
	h.PG.CreateSweepJob(ctx, jobID, keywords, map[string]any{"max_depth": cfg.MaxDepth, "max_urls": cfg.MaxURLs})

	jobCtx, cancelFn := context.WithCancel(context.Background())
	service.GlobalJobManager.Register2(jobID, cancelFn)
	go func() {
		defer service.GlobalJobManager.Remove(jobID)
		_, err := h.DiscoverySvc.DeepSweepJob(jobCtx, jobID, keywords, cfg)
		status, errMsg := "done", ""
		if err != nil {
			status, errMsg = "error", err.Error()
		}
		bgCtx := context.Background()
		h.PG.FinishSweepJob(bgCtx, jobID, status, errMsg)
		if status == "done" && h.Enricher != nil {
			h.Enricher.Run(bgCtx)
		}
	}()
	return jobID, "", nil
}

// POST /sweep/:id/pause
func (h *SweepJobHandler) Pause(c *gin.Context) {
	id := c.Param("id")
	job, ok := service.GlobalJobManager.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found or already finished"})
		return
	}
	job.Pause()
	h.PG.UpdateSweepJobStatus(c.Request.Context(), id, "paused")
	c.JSON(http.StatusOK, gin.H{"job_id": id, "status": "paused"})
}

// POST /sweep/:id/resume
func (h *SweepJobHandler) Resume(c *gin.Context) {
	id := c.Param("id")
	job, ok := service.GlobalJobManager.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found or already finished"})
		return
	}
	job.Resume()
	h.PG.UpdateSweepJobStatus(c.Request.Context(), id, "running")
	c.JSON(http.StatusOK, gin.H{"job_id": id, "status": "running"})
}

// POST /sweep/:id/cancel
func (h *SweepJobHandler) Cancel(c *gin.Context) {
	id := c.Param("id")
	job, ok := service.GlobalJobManager.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found or already finished"})
		return
	}
	job.Cancel()
	h.PG.FinishSweepJob(c.Request.Context(), id, "cancelled", "")
	c.JSON(http.StatusOK, gin.H{"job_id": id, "status": "cancelled"})
}

// GET /sweep/:id
func (h *SweepJobHandler) Status(c *gin.Context) {
	id := c.Param("id")
	job, err := h.PG.GetSweepJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	// Log & fase hanya ada selama job hidup di memori server
	since, _ := strconv.Atoi(c.Query("since"))
	resp := gin.H{"job": job, "live": nil, "events": []service.JobEvent{}}
	if st, ok := service.GlobalJobManager.Get(job.ID); ok {
		live, events, _ := st.Snapshot(since)
		resp["live"], resp["events"] = live, events
	}
	c.JSON(http.StatusOK, resp)
}

// GET /sweep/active
func (h *SweepJobHandler) Active(c *gin.Context) {
	job, err := h.PG.GetActiveSweepJob(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"job": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": job})
}

// StartDailyScheduler: cek tiap 10 menit — pada jam yang diatur (sweep_daily_hour), jalankan satu Deep Sweep
// per hari dengan N keyword bawaan yang dirotasi (sweep_daily_offset), sehingga seluruh daftar tercakup bergiliran.
func (h *SweepJobHandler) StartDailyScheduler() {
	go func() {
		for {
			h.dailyTick(context.Background())
			time.Sleep(10 * time.Minute)
		}
	}()
}

func (h *SweepJobHandler) dailyTick(ctx context.Context) {
	if !h.PG.GetSettingBool(ctx, "sweep_daily_enabled", false) {
		return
	}
	now := time.Now()
	today := now.Format("2006-01-02")
	if now.Hour() != h.PG.GetSettingInt(ctx, "sweep_daily_hour", 2) || h.PG.GetSetting(ctx, "sweep_daily_last") == today {
		return
	}
	all := h.DiscoverySvc.LoadDefaultKeywords(ctx)
	if len(all) == 0 {
		return
	}
	n := h.PG.GetSettingInt(ctx, "sweep_daily_keywords", 20)
	if n > len(all) {
		n = len(all)
	}
	off := h.PG.GetSettingInt(ctx, "sweep_daily_offset", 0) % len(all)
	if off < 0 {
		off = 0
	}
	kws := make([]string, 0, n)
	for i := 0; i < n; i++ {
		kws = append(kws, all[(off+i)%len(all)])
	}
	cfg := service.DeepSweepConfig{MaxDepth: h.PG.GetSettingInt(ctx, "sweep_daily_depth", 2), MaxURLs: h.PG.GetSettingInt(ctx, "sweep_daily_max_urls", 300)}
	jobID, activeID, err := h.launch(ctx, kws, cfg)
	if activeID != "" {
		log.Printf("⏭️  [Sweep Harian] dilewati — masih ada sweep %s berjalan", activeID)
		return // coba lagi di cek berikutnya (masih jam yang sama)
	}
	if err != nil {
		log.Printf("⚠️  [Sweep Harian] gagal mulai: %v", err)
		return
	}
	h.PG.SetSetting(ctx, "sweep_daily_last", today)
	h.PG.SetSetting(ctx, "sweep_daily_offset", strconv.Itoa((off+n)%len(all)))
	h.PG.Audit(ctx, "otomatis", "", "sweep.daily.started", jobID, 200, map[string]any{"keywords": n, "offset": off})
	log.Printf("🌙 [Sweep Harian] %s dimulai — %d keyword (mulai #%d dari %d)", jobID, n, off+1, len(all))
}
