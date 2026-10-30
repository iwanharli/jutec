package handlers

import (
	"jutec-go/internal/store"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type NetworkHandler struct {
	PG *store.PostgresDB
}

func NewNetworkHandler(pg *store.PostgresDB) *NetworkHandler {
	return &NetworkHandler{PG: pg}
}

func (h *NetworkHandler) dbRequired(c *gin.Context) bool {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return false
	}
	return true
}

// --- Tracking ID ---

// GET /network/tracking/lookup?platform=LiveChat&value=19348776
func (h *NetworkHandler) TrackingLookup(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	platform := c.Query("platform")
	value := c.Query("value")
	if platform == "" || value == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform and value parameters are required"})
		return
	}

	results, err := h.PG.GetSitesByTracking(c.Request.Context(), platform, value)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"search":  gin.H{"platform": platform, "value": value},
		"count":   len(results),
		"results": results,
	})
}

// GET /network/tracking/top?platform=GoogleAnalytics&limit=20
func (h *NetworkHandler) TrackingTop(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	platform := c.Query("platform")
	limit := parseLimit(c.Query("limit"), 20)

	results, err := h.PG.GetTopSharedTracking(c.Request.Context(), platform, limit, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"filter":  gin.H{"platform": platform, "limit": limit},
		"count":   len(results),
		"results": results,
	})
}

// --- IP / Subnet ---

// GET /network/ip/lookup?ip=103.21.186.5
func (h *NetworkHandler) IPLookup(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	ip := c.Query("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip parameter is required"})
		return
	}

	results, err := h.PG.GetSitesByIP(c.Request.Context(), ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ip":      ip,
		"count":   len(results),
		"results": results,
	})
}

// GET /network/ip/subnet?subnet=103.21.186
func (h *NetworkHandler) SubnetLookup(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	subnet := c.Query("subnet")
	if subnet == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subnet parameter is required (e.g. 103.21.186)"})
		return
	}

	results, err := h.PG.GetSitesBySubnet(c.Request.Context(), subnet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"subnet":  subnet + ".0/24",
		"count":   len(results),
		"results": results,
	})
}

// GET /network/ip/top?limit=20
func (h *NetworkHandler) SubnetTop(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	limit := parseLimit(c.Query("limit"), 20)

	results, err := h.PG.GetTopSharedSubnets(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"count":   len(results),
		"results": results,
	})
}

// --- SimHash / Template ---

// GET /network/template/similar?simhash=abc123&threshold=8
func (h *NetworkHandler) TemplateSimilar(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	simhash := c.Query("simhash")
	if simhash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "simhash parameter is required"})
		return
	}
	threshold := 8
	if t := c.Query("threshold"); t != "" {
		if n, err := strconv.Atoi(t); err == nil && n >= 0 && n <= 32 {
			threshold = n
		}
	}

	results, err := h.PG.GetSimilarTemplates(c.Request.Context(), simhash, threshold)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"simhash":   simhash,
		"threshold": threshold,
		"count":     len(results),
		"results":   results,
	})
}

// --- Nameserver ---

// GET /network/ns/lookup?host=ns1.cloudflare.com
func (h *NetworkHandler) NSLookup(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	host := c.Query("host")
	if host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host parameter is required"})
		return
	}
	results, err := h.PG.GetSitesByNameserver(c.Request.Context(), host)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"host": host, "count": len(results), "results": results})
}

// GET /network/ns/top?limit=20
func (h *NetworkHandler) NSTop(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	results, err := h.PG.GetTopSharedNameservers(c.Request.Context(), parseLimit(c.Query("limit"), 20))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(results), "results": results})
}

// --- SSL Certificate ---

// GET /network/ssl/lookup?thumbprint=abc123
func (h *NetworkHandler) SSLLookup(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	thumb := c.Query("thumbprint")
	if thumb == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "thumbprint parameter is required"})
		return
	}
	results, err := h.PG.GetSitesBySSLCert(c.Request.Context(), thumb)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"thumbprint": thumb, "count": len(results), "results": results})
}

// GET /network/ssl/top?limit=20
func (h *NetworkHandler) SSLTop(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	results, err := h.PG.GetTopSharedSSLCerts(c.Request.Context(), parseLimit(c.Query("limit"), 20))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(results), "results": results})
}

// --- TXT Token ---

// GET /network/txt/lookup?service=GoogleVerification&token=abc123
func (h *NetworkHandler) TXTLookup(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	service := c.Query("service")
	token := c.Query("token")
	if service == "" || token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service and token parameters are required"})
		return
	}

	results, err := h.PG.GetSitesByTXTToken(c.Request.Context(), service, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"search":  gin.H{"service": service, "token": token},
		"count":   len(results),
		"results": results,
	})
}

// GET /network/txt/top?service=GoogleVerification&limit=20
func (h *NetworkHandler) TXTTop(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	service := c.Query("service")
	limit := parseLimit(c.Query("limit"), 20)

	results, err := h.PG.GetTopSharedTXTTokens(c.Request.Context(), service, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"filter":  gin.H{"service": service, "limit": limit},
		"count":   len(results),
		"results": results,
	})
}

func parseLimit(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 100 {
		return def
	}
	return n
}
