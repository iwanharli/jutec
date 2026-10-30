package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jutec-go/internal/service"
	"jutec-go/internal/store"
	"jutec-go/internal/worker/tasks"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

type VflHandler struct {
	PG    *store.PostgresDB
	Tasks *asynq.Client
}

// POST /leak/vfl {"phone":"628..."} — antrekan pengecekan berantai. Hanya nomor yang tercatat
// sebagai artefak situs judol yang diterima.
func (h *VflHandler) Check(c *gin.Context) {
	var body struct {
		Phone string `json:"phone"`
	}
	c.ShouldBindJSON(&body)
	phone := strings.TrimPrefix(strings.TrimSpace(body.Phone), "+")
	if strings.HasPrefix(phone, "0") {
		phone = "62" + phone[1:]
	}
	if !phonePattern.MatchString(phone) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format nomor tidak valid"})
		return
	}
	var artifactID int
	h.PG.Pool.QueryRow(c, `
		SELECT a.id FROM artifacts a JOIN site_artifacts sa ON sa.artifact_id = a.id
		JOIN sites s ON s.id = sa.site_id AND s.is_gambling
		WHERE regexp_replace(a.value, '^\+', '') = $1 LIMIT 1`, phone).Scan(&artifactID)
	if artifactID == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "nomor ini tidak tercatat sebagai artefak situs judol"})
		return
	}
	if h.Tasks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "worker tidak aktif"})
		return
	}
	payload, _ := json.Marshal(tasks.VflPayload{ArtifactID: artifactID, Phone: phone})
	if _, err := h.Tasks.Enqueue(asynq.NewTask(tasks.TypeVflChain, payload, asynq.MaxRetry(0)),
		asynq.Queue("enrichment"), asynq.Unique(10*time.Minute)); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "pengecekan nomor ini sudah dalam antrean"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"queued": true, "artifact_id": artifactID,
		"message": "Pengecekan berantai diantrekan (nomor → NIK → nomor lain). Butuh ±1–3 menit."})
}

// GET /leak/vfl/chain?artifact_id=… — log audit rantai pengecekan satu artefak.
func (h *VflHandler) Chain(c *gin.Context) {
	id, _ := strconv.Atoi(c.Query("artifact_id"))
	if id == 0 && c.Query("phone") != "" {
		h.PG.Pool.QueryRow(c, `SELECT id FROM artifacts WHERE regexp_replace(value, '^\+', '') = $1 LIMIT 1`,
			strings.TrimPrefix(c.Query("phone"), "+")).Scan(&id)
	}
	list, err := h.PG.VflChain(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"queries": list})
}

// GET /leak/vfl/status — pemakaian kuota harian.
func (h *VflHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"today": h.PG.VflQueriesToday(c), "daily_cap": service.VflDailyCap})
}
