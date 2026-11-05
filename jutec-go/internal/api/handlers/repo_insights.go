package handlers

import (
	"net/http"
	"strconv"

	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

// RepoInsightsHandler: tab lintas situs di Repository (infrastruktur, dompet, susupan, respawn, referral, banner).
type RepoInsightsHandler struct {
	PG *store.PostgresDB
}

func qInt(c *gin.Context, k string, def, max int) int {
	n, err := strconv.Atoi(c.Query(k))
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

func (h *RepoInsightsHandler) respond(c *gin.Context, v any, err error) {
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": v})
}

// GET /data/shared?kind=ip|asn|ns|ssl|fingerprint|referral|image&q=&min=2&limit=200
func (h *RepoInsightsHandler) Shared(c *gin.Context) {
	r, err := h.PG.SharedAttributes(c.Request.Context(), c.Query("kind"), c.Query("q"), qInt(c, "min", 2, 1000), qInt(c, "limit", 200, 1000))
	h.respond(c, r, err)
}

// GET /data/wallets?q=
func (h *RepoInsightsHandler) Wallets(c *gin.Context) {
	r, err := h.PG.Wallets(c.Request.Context(), c.Query("q"), qInt(c, "limit", 300, 2000))
	h.respond(c, r, err)
}

// GET /data/injected?q=
func (h *RepoInsightsHandler) Injected(c *gin.Context) {
	r, err := h.PG.InjectedSites(c.Request.Context(), c.Query("q"), qInt(c, "limit", 500, 5000))
	h.respond(c, r, err)
}

// GET /data/respawns
func (h *RepoInsightsHandler) Respawns(c *gin.Context) {
	r, err := h.PG.Respawns(c.Request.Context(), qInt(c, "limit", 300, 2000))
	h.respond(c, r, err)
}

// GET /data/takeovers — domain bekas situs sah yang kini dipakai judol
func (h *RepoInsightsHandler) Takeovers(c *gin.Context) {
	r, err := h.PG.Takeovers(c.Request.Context(), qInt(c, "limit", 300, 2000))
	h.respond(c, r, err)
}
