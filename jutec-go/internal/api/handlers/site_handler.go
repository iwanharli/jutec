package handlers

import (
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"strconv"
	"time"

	"jutec-go/internal/service"
	"jutec-go/internal/store"
	"jutec-go/internal/wayback"
	"jutec-go/internal/worker/tasks"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
)

type SiteHandler struct {
	PG     *store.PostgresDB
	Scan   *service.ScanService
	Tasks  *asynq.Client             // antrean background (analisis lanjutan)
	Mirror *tasks.MirrorWatchHandler // pencarian mirror on-demand
}

func NewSiteHandler(pg *store.PostgresDB) *SiteHandler {
	return &SiteHandler{PG: pg}
}

// GET /sites/lookup?url=... → {"id": 123}
func (h *SiteHandler) Lookup(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}
	id, err := h.PG.GetSiteIDByURL(c.Request.Context(), url)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "situs belum pernah di-scan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// GET /sites/:id
func (h *SiteHandler) Detail(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	d, err := h.PG.GetSiteDetail(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "situs tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, d)
}

// GET /sites/:id/identities — identitas pemilik nomor kontak situs dari data bocoran tersimpan.
func (h *SiteHandler) Identities(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	list, err := h.PG.GetSiteIdentities(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"identities": list})
}

// GET /sites/:id/relations?per_attr=15&max_shared=40
func (h *SiteHandler) Relations(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	perAttr, _ := strconv.Atoi(c.DefaultQuery("per_attr", "15"))
	if perAttr <= 0 || perAttr > 100 {
		perAttr = 15
	}
	maxShared, _ := strconv.Atoi(c.DefaultQuery("max_shared", "40"))
	if maxShared <= 0 {
		maxShared = 40
	}
	rel, err := h.PG.GetSiteRelations(c.Request.Context(), id, perAttr, maxShared)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "situs tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rel)
}

func siteIDParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

// POST /sites/:id/enrich — antrekan URLScan + VirusTotal untuk situs yang sudah di-scan
func (h *SiteHandler) Enrich(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	d, err := h.PG.GetSiteDetail(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "situs tidak ditemukan"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.Scan.EnqueueEnrichment(id, d.Site.URL); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": "enrichment diantrekan", "site_id": id})
}

// siteURL mengambil URL situs; menulis 404 bila tidak ada.
func (h *SiteHandler) siteURL(c *gin.Context, id int) (string, bool) {
	var u string
	if err := h.PG.Pool.QueryRow(c.Request.Context(), `SELECT url FROM sites WHERE id = $1`, id).Scan(&u); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "situs tidak ditemukan"})
		return "", false
	}
	return u, true
}

// GET /sites/:id/changes — perubahan artefak/link/tracking antar scan
func (h *SiteHandler) Changes(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	ch, err := h.PG.SiteChanges(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"changes": ch})
}

// GET /sites/:id/apk — hasil analisis statis APK
func (h *SiteHandler) APK(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	list, err := h.PG.APKAnalysesForSite(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []map[string]any{}
	}
	c.JSON(http.StatusOK, gin.H{"apks": list})
}

// POST /sites/:id/deep — antrekan analisis lanjutan (QRIS, OCR, APK) sekarang
func (h *SiteHandler) Deep(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	if h.Tasks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "antrean background (Redis) tidak aktif"})
		return
	}
	payload := []byte(fmt.Sprintf(`{"site_id":%d}`, id))
	if _, err := h.Tasks.Enqueue(asynq.NewTask(tasks.TypeDeepSite, payload, asynq.MaxRetry(1)),
		asynq.Queue("low"), asynq.Timeout(15*time.Minute)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": "analisis lanjutan diantrekan"})
}

// GET /sites/:id/wayback — snapshot yang sudah diambil
func (h *SiteHandler) Wayback(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	snaps, err := h.PG.WaybackSnapshots(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"snapshots": snaps})
}

// POST /sites/:id/wayback — ambil hingga 6 snapshot dari Wayback Machine & simpan artefak lamanya
func (h *SiteHandler) FetchWayback(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	raw, ok := h.siteURL(c, id)
	if !ok {
		return
	}
	u, _ := neturl.Parse(raw)
	snaps, err := wayback.Fetch(u.Hostname(), 6)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	historic := h.PG.SaveWaybackResult(ctx, id, snaps)
	all, _ := h.PG.WaybackSnapshots(ctx, id)
	c.JSON(http.StatusOK, gin.H{"fetched": len(snaps), "artifacts": historic, "snapshots": all})
}

// POST /sites/:id/mirrors — cari domain mirror brand situs ini di CT Logs & antrekan analisisnya
func (h *SiteHandler) Mirrors(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	if h.Mirror == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "antrean background (Redis) tidak aktif"})
		return
	}
	raw, ok := h.siteURL(c, id)
	if !ok {
		return
	}
	u, _ := neturl.Parse(raw)
	brand := c.DefaultQuery("brand", tasks.BrandOf(u.Hostname()))
	if brand == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nama brand tidak bisa ditentukan dari domain ini"})
		return
	}
	queued, err := h.Mirror.WatchBrand(c.Request.Context(), brand, 25)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if queued == nil {
		queued = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"brand": brand, "queued": queued})
}

// GET /sites/:id/telegram — status setiap kontak Telegram situs (valid / tidak ditemukan / tertunda) + detail
func (h *SiteHandler) Telegram(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	list, err := h.PG.TelegramStatusForSite(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []map[string]any{}
	}
	c.JSON(http.StatusOK, gin.H{"accounts": list})
}
