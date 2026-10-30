package handlers

import (
	"strings"
	"net/http"
	"strconv"

	"context"
	"jutec-go/internal/service"
	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

type DiscoveryHandler struct {
	DiscoverySvc *service.DiscoveryService
	PG           *store.PostgresDB
	Enricher     *service.AutoEnricher
}

func NewDiscoveryHandler(svc *service.DiscoveryService, pg *store.PostgresDB) *DiscoveryHandler {
	return &DiscoveryHandler{DiscoverySvc: svc, PG: pg}
}

// GET /discovery/cache/stats — statistik cache (entries, TTL, per source)
func (h *DiscoveryHandler) CacheStats(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not connected"})
		return
	}
	stats, err := h.PG.GetDiscoveryCacheStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// DELETE /discovery/cache — hapus semua cache yang expired, atau semua jika ?force=true
func (h *DiscoveryHandler) CachePurge(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not connected"})
		return
	}
	ctx := c.Request.Context()

	var deleted int64
	if c.Query("force") == "true" {
		tag, err := h.PG.Pool.Exec(ctx, `DELETE FROM discovery_cache`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		deleted = tag.RowsAffected()
	} else {
		var err error
		deleted, err = h.PG.PurgeExpiredCache(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cache dibersihkan",
		"deleted": deleted,
	})
}

func (h *DiscoveryHandler) DiscoverGoogle(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	urls, err := h.DiscoverySvc.SearchByKeyword(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":       query,
		"found_count": len(urls),
		"results":     urls,
	})
}

type SweepRequest struct {
	Keywords    []string `json:"keywords"`
	UseDefaults bool     `json:"use_defaults"`
	AutoAnalyze bool     `json:"auto_analyze"`
}

func (h *DiscoveryHandler) DiscoverCerts(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	domains, err := h.DiscoverySvc.DiscoverCerts(keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"keyword":     keyword,
		"found_count": len(domains),
		"results":     domains,
	})
}

func (h *DiscoveryHandler) Sweep(c *gin.Context) {
	var req SweepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	keywords := req.Keywords
	if req.UseDefaults {
		keywords = append(keywords, h.DiscoverySvc.LoadDefaultKeywords(c.Request.Context())...)
	}

	// Deduplicate keywords
	seen := make(map[string]bool)
	var unique []string
	for _, kw := range keywords {
		if !seen[kw] {
			seen[kw] = true
			unique = append(unique, kw)
		}
	}

	if len(unique) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide at least one keyword, or set use_defaults: true"})
		return
	}

	result, err := h.DiscoverySvc.Sweep(c.Request.Context(), unique, req.AutoAnalyze)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

type DeepSweepRequest struct {
	Keywords    []string `json:"keywords"`
	UseDefaults bool     `json:"use_defaults"`
	MaxDepth    int      `json:"max_depth"`
	MaxURLs     int      `json:"max_urls"`
}

func (h *DiscoveryHandler) DeepSweep(c *gin.Context) {
	// Support both GET (domain-based) and POST (keyword-based)
	if c.Request.Method == http.MethodGet {
		domain := c.Query("domain")
		if domain == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "domain parameter is required for GET request"})
			return
		}

		maxDepth, _ := strconv.Atoi(c.DefaultQuery("depth", "3"))
		maxURLs, _ := strconv.Atoi(c.DefaultQuery("limit", "500"))

		// Domain awal dianalisis langsung di Depth 0, lalu link/tracking/NS-nya diikuti di Depth 1+.
		// (Sebelumnya domain dipakai sebagai keyword Serper sehingga domain itu sendiri tak pernah di-crawl.)
		domain = strings.TrimSpace(domain)
		if !strings.Contains(domain, "://") {
			domain = "https://" + domain
		}
		cfg := service.DeepSweepConfig{
			MaxDepth: maxDepth,
			MaxURLs:  maxURLs,
			SeedURLs: []string{domain},
		}

		result, err := h.DiscoverySvc.DeepSweep(c.Request.Context(), nil, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
		return
	}

	// POST: Original keyword-based logic
	var req DeepSweepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	keywords := req.Keywords
	if req.UseDefaults {
		keywords = append(keywords, h.DiscoverySvc.LoadDefaultKeywords(c.Request.Context())...)
	}

	// Deduplicate keywords
	seen := make(map[string]bool)
	var unique []string
	for _, kw := range keywords {
		if !seen[kw] {
			seen[kw] = true
			unique = append(unique, kw)
		}
	}

	if len(unique) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provide at least one keyword, or set use_defaults: true"})
		return
	}

	cfg := service.DeepSweepConfig{
		MaxDepth: req.MaxDepth,
		MaxURLs:  req.MaxURLs,
	}

	result, err := h.DiscoverySvc.DeepSweep(c.Request.Context(), unique, cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.Enricher != nil {
		go h.Enricher.Run(context.Background())
	}

	c.JSON(http.StatusOK, result)
}
