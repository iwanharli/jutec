package handlers

import (
	"context"
	"fmt"
	"net/http"
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
		if !seen[kw] { seen[kw] = true; unique = append(unique, kw) }
	}
	if len(unique) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide at least one keyword or set use_defaults: true"})
		return
	}

	// Cek apakah sudah ada job yang running/paused
	if active, err := h.PG.GetActiveSweepJob(c.Request.Context()); err == nil && active != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":  "job already running or paused",
			"job_id": active.ID,
			"status": active.Status,
		})
		return
	}

	jobID := fmt.Sprintf("sweep-%d", time.Now().UnixMilli())
	cfg := service.DeepSweepConfig{MaxDepth: req.MaxDepth, MaxURLs: req.MaxURLs}
	if cfg.MaxDepth <= 0 { cfg.MaxDepth = 3 }
	if cfg.MaxURLs  <= 0 { cfg.MaxURLs  = 500 }

	// Simpan ke DB
	h.PG.CreateSweepJob(c.Request.Context(), jobID, unique, map[string]any{
		"max_depth": cfg.MaxDepth,
		"max_urls":  cfg.MaxURLs,
	})

	// Buat job context + state
	jobCtx, cancelFn := context.WithCancel(context.Background())
	jobState := service.GlobalJobManager.Register2(jobID, cancelFn)

	// Jalankan di goroutine
	go func() {
		defer service.GlobalJobManager.Remove(jobID)
		_, err := h.DiscoverySvc.DeepSweepJob(jobCtx, jobID, unique, cfg)
		status := "done"
		errMsg := ""
		if err != nil {
			status = "error"
			errMsg = err.Error()
		}
		bgCtx := context.Background()
		h.PG.FinishSweepJob(bgCtx, jobID, status, errMsg)
		if status == "done" && h.Enricher != nil {
			h.Enricher.Run(bgCtx)
		}
	}()

	_ = jobState
	c.JSON(http.StatusOK, gin.H{"job_id": jobID, "status": "running"})
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
	c.JSON(http.StatusOK, job)
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
