package handlers

import (
	"encoding/csv"
	"fmt"
	"jutec-go/internal/store"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type DataHandler struct {
	PG *store.PostgresDB
}

func NewDataHandler(pg *store.PostgresDB) *DataHandler {
	return &DataHandler{PG: pg}
}

func (h *DataHandler) GetSummary(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return
	}

	summary, err := h.PG.GetDashboardSummary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *DataHandler) GetSites(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return
	}

	priority := c.Query("priority")
	isGamblingStr := c.Query("is_gambling")
	q := c.Query("q")
	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}

	offsetStr := c.DefaultQuery("offset", "0")
	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}

	csvOut := c.Query("format") == "csv"
	if limit > 200 && !csvOut {
		limit = 200
	}
	if csvOut {
		limit, offset = 10000, 0
	}
	f := store.SiteFilter{
		Priority: priority, Query: q, Limit: limit, Offset: offset,
		Status:        c.Query("status"),
		Lifecycle:     c.Query("lifecycle"),
		Category:      c.Query("category"),
		TP:            c.Query("tp"),
		JudolOnly:     c.Query("judol_only") == "1",
		GroupByDomain: c.Query("group") == "domain" && c.Query("format") != "csv",
		ArtifactType:  c.Query("artifact_type"),
		HasApp:        c.Query("has_app") == "true",
		Sort:          c.DefaultQuery("sort", "last_scanned_at"),
		Desc:          c.DefaultQuery("order", "desc") != "asc",
	}
	if isGamblingStr != "" {
		val := isGamblingStr == "true"
		f.IsGambling = &val
	}
	f.MinConf, _ = strconv.ParseFloat(c.Query("min_conf"), 64)
	f.MaxConf, _ = strconv.ParseFloat(c.Query("max_conf"), 64)
	f.ScannedDays, _ = strconv.Atoi(c.Query("scanned_days"))

	sites, total, err := h.PG.GetRecentSites(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if csvOut {
		writeSitesCSV(c, sites)
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": sites, "total": total, "category_counts": h.PG.SiteCategoryCounts(c.Request.Context(), f.Lifecycle)})
}

func (h *DataHandler) GetApps(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}

	apps, err := h.PG.GetTopApps(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, apps)
}

// GET /data/pending-summary — jumlah item pending untuk semua resolve jobs
func (h *DataHandler) PendingSummary(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not connected"})
		return
	}
	ctx := c.Request.Context()

	tgUsernames, _ := h.PG.GetUnresolvedTelegramUsernames(ctx, 9999)
	leakArtifacts, _ := h.PG.GetPhoneArtifactsNotEnriched(ctx)

	var opCount int
	h.PG.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM operators`).Scan(&opCount)

	total := len(tgUsernames) + len(leakArtifacts)
	c.JSON(http.StatusOK, gin.H{
		"telegram": len(tgUsernames),
		"leak":     len(leakArtifacts),
		"total":    total,
	})
}

// GET /data/tracking?platform=GA&limit=50
func (h *DataHandler) GetTracking(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return
	}
	platform := c.Query("platform")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	results, err := h.PG.GetTopSharedTracking(c.Request.Context(), platform, limit, c.Query("include_inactive") == "1")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(results), "results": results})
}

// GET /data/operators?platform=&limit=50&offset=0
func (h *DataHandler) GetOperators(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return
	}
	platform := c.Query("platform")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	result, err := h.PG.ListOperators(c.Request.Context(), platform, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// GET /data/links?q=&type=external|iframe&limit=10&offset=0
func (h *DataHandler) GetLinks(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return
	}
	q := c.Query("q")
	linkType := c.Query("type")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	results, total, err := h.PG.GetExternalDomainsPaged(c.Request.Context(), q, linkType, limit, offset, c.Query("include_inactive") == "1")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "count": len(results), "results": results})
}

// GET /data/kominfo?q=&limit=10&offset=0
func (h *DataHandler) GetKomInfo(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return
	}
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	results, total, err := h.PG.ListLeakKomInfo(c.Request.Context(), q, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "count": len(results), "results": results})
}

// GET /data/dukcapil?q=&limit=10&offset=0
func (h *DataHandler) GetDukcapil(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return
	}
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	results, total, err := h.PG.ListLeakDukcapil(c.Request.Context(), q, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "count": len(results), "results": results})
}

// GET /data/telegram?limit=50&offset=0
func (h *DataHandler) GetTelegram(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	channels, total, err := h.PG.ListTelegramChannels(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "count": len(channels), "results": channels})
}

// writeSitesCSV: ekspor daftar situs sesuai filter (mis. judol yang belum ada di TrustPositif untuk diajukan).
func writeSitesCSV(c *gin.Context, sites []store.RecentSite) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="jutec-situs-%s.csv"`, time.Now().Format("20060102-1504")))
	c.Writer.WriteString("\ufeff") // BOM agar Excel membaca UTF-8
	w := csv.NewWriter(c.Writer)
	w.Write([]string{"id", "url", "domain", "jenis", "prioritas", "confidence", "status", "trustpositif", "ditemukan", "scan_terakhir"})
	for _, s := range sites {
		tp := "belum dicek"
		if s.TPListed != nil {
			tp = map[bool]string{true: "terdaftar", false: "belum terdaftar"}[*s.TPListed]
		}
		host := s.URL
		if u, err := url.Parse(s.URL); err == nil {
			host = u.Hostname()
		}
		w.Write([]string{strconv.Itoa(s.ID), s.URL, host, s.Category, s.Priority, fmt.Sprintf("%.2f", s.Confidence), s.Status, tp, s.CreatedAt, s.LastScannedAt})
	}
	w.Flush()
}
