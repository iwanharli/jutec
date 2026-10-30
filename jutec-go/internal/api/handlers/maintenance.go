package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"jutec-go/internal/crawler"
	"jutec-go/internal/store"
	"jutec-go/internal/worker/tasks"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

// MaintenanceHandler: pemeliharaan data dari UI — membersihkan artefak palsu dan scan ulang dengan engine terbaru,
// agar tidak perlu menjalankan apa pun lewat terminal.
type MaintenanceHandler struct {
	PG    *store.PostgresDB
	Tasks *asynq.Client

	mu     sync.Mutex
	rescan *rescanJob
}

type rescanJob struct {
	Scope     string    `json:"scope"`
	StartedAt time.Time `json:"started_at"`
	By        string    `json:"by"`
	Total     int       `json:"total"`
	Queued    int       `json:"queued"`
	WasJudol  int `json:"was_judol"`
	ids       []int
	judolIDs  []int
}

// GET /maintenance/summary
func (h *MaintenanceHandler) Summary(c *gin.Context) {
	entries, urls := h.PG.SearchCacheStats(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"rescan_counts": h.PG.RescanCounts(c.Request.Context()),
		"search_cache":  gin.H{"entries": entries, "urls": urls},
		"worker":        h.Tasks != nil,
	})
}

// GET /maintenance/artifacts/fake — pratinjau artefak Bank yang menurut aturan terbaru bukan rekening
func (h *MaintenanceHandler) FakeArtifacts(c *gin.Context) {
	all, err := h.PG.BankArtifactsForReview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := []store.FakeArtifact{}
	for _, a := range all {
		if r := crawler.AccountRejectReason(a.Value); r != "" {
			a.Reason = r
			out = append(out, a)
		}
	}
	c.JSON(http.StatusOK, gin.H{"artifacts": out, "checked": len(all)})
}

// POST /maintenance/artifacts/clean {ids} — hanya id yang memang gagal aturan yang dihapus (dicadangkan dulu)
func (h *MaintenanceHandler) CleanArtifacts(c *gin.Context) {
	var req struct {
		IDs []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids wajib diisi"})
		return
	}
	all, err := h.PG.BankArtifactsForReview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	allowed := map[int]bool{}
	for _, a := range all {
		if crawler.AccountRejectReason(a.Value) != "" {
			allowed[a.ID] = true
		}
	}
	var ids []int
	for _, id := range req.IDs {
		if allowed[id] {
			ids = append(ids, id)
		}
	}
	n, err := h.PG.DeleteArtifactsWithBackup(c.Request.Context(), ids, ActorOf(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"removed": n, "backup_table": "artifact_cleanup_backup"})
}

// POST /maintenance/rescan {scope} — antrekan scan ulang lewat worker
func (h *MaintenanceHandler) StartRescan(c *gin.Context) {
	if h.Tasks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "worker background (Redis) tidak aktif"})
		return
	}
	var req struct {
		Scope string `json:"scope"`
	}
	c.ShouldBindJSON(&req)
	if _, ok := store.RescanScopes[req.Scope]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cakupan tidak dikenal"})
		return
	}
	h.mu.Lock()
	if j := h.rescan; j != nil {
		done, _, _ := h.PG.RescanProgress(c.Request.Context(), j.ids, j.judolIDs, j.StartedAt)
		if done < j.Queued && time.Since(j.StartedAt) < 3*time.Hour {
			h.mu.Unlock()
			c.JSON(http.StatusConflict, gin.H{"error": "scan ulang sebelumnya masih berjalan"})
			return
		}
	}
	h.mu.Unlock()

	targets, err := h.PG.RescanTargets(c.Request.Context(), req.Scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	job := &rescanJob{Scope: req.Scope, StartedAt: time.Now(), By: ActorOf(c), Total: len(targets)}
	for _, t := range targets {
		payload, _ := json.Marshal(tasks.ScanURLPayload{URL: t.URL})
		// MaxRetry 1: situs yang gagal tidak diulang puluhan kali; kegagalan tidak lagi menimpa vonis lama
		if _, err := h.Tasks.Enqueue(asynq.NewTask(tasks.TypeScanURL, payload), asynq.Queue("default"), asynq.MaxRetry(1)); err == nil {
			job.Queued++
			job.ids = append(job.ids, t.ID)
			if t.IsGambling {
				job.WasJudol++
				job.judolIDs = append(job.judolIDs, t.ID)
			}
		}
	}
	h.mu.Lock()
	h.rescan = job
	h.mu.Unlock()
	h.PG.Audit(c.Request.Context(), job.By, c.ClientIP(), "maintenance.rescan", "scope:"+req.Scope, 200, map[string]any{"queued": job.Queued})
	c.JSON(http.StatusOK, gin.H{"queued": job.Queued, "total": job.Total})
}

// GET /maintenance/rescan — progres scan ulang terakhir
func (h *MaintenanceHandler) RescanStatus(c *gin.Context) {
	h.mu.Lock()
	j := h.rescan
	h.mu.Unlock()
	if j == nil {
		c.JSON(http.StatusOK, gin.H{"job": nil})
		return
	}
	done, failed, corrected := h.PG.RescanProgress(c.Request.Context(), j.ids, j.judolIDs, j.StartedAt)
	c.JSON(http.StatusOK, gin.H{"job": j, "done": done, "failed": failed, "corrected": corrected,
		"running": done < j.Queued && time.Since(j.StartedAt) < 3*time.Hour})
}

// POST /maintenance/cache/clear — kosongkan cache pencarian Google
func (h *MaintenanceHandler) ClearCache(c *gin.Context) {
	n, err := h.PG.ClearSearchCache(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"removed": n})
}
