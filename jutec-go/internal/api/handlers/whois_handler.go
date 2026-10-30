package handlers

import (
	"net/http"
	"strconv"

	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

type WHOISHandler struct {
	PG *store.PostgresDB
}

func NewWHOISHandler(pg *store.PostgresDB) *WHOISHandler {
	return &WHOISHandler{PG: pg}
}

// GET /whois/reverse?registrar=Namecheap&tld=.top&is_private=true&max_age_days=90&min_risk=0.5&limit=20&offset=0
func (h *WHOISHandler) Reverse(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not connected"})
		return
	}

	filter := store.ReverseWHOISFilter{
		Registrar: c.Query("registrar"),
		TLD:       c.Query("tld"),
	}

	if v := c.Query("is_private"); v != "" {
		b := v == "true"
		filter.IsPrivate = &b
	}
	if v := c.Query("max_age_days"); v != "" {
		filter.MaxAgeDays, _ = strconv.Atoi(v)
	}
	if v := c.Query("min_risk"); v != "" {
		filter.MinRisk, _ = strconv.ParseFloat(v, 64)
	}

	limit, _  := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0  { limit = 20 }
	if limit > 200 { limit = 200 }
	if offset < 0  { offset = 0 }

	result, err := h.PG.ReverseWHOIS(c.Request.Context(), filter, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":   result.Total,
		"limit":   limit,
		"offset":  offset,
		"filter":  filter,
		"results": result.Results,
	})
}
