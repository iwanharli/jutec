package handlers

import (
	"net/http"
	"strconv"

	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

type OperatorHandler struct {
	PG *store.PostgresDB
}

func NewOperatorHandler(pg *store.PostgresDB) *OperatorHandler {
	return &OperatorHandler{PG: pg}
}

// POST /operators/discover — jalankan clustering otomatis
func (h *OperatorHandler) Discover(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not connected"})
		return
	}
	total, err := h.PG.DiscoverOperators(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":         "Operator discovery selesai",
		"total_operators": total,
	})
}

// GET /operators?platform=AsiaSigma&limit=20&offset=0
func (h *OperatorHandler) List(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not connected"})
		return
	}
	platform := c.Query("platform")
	limit, _  := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 { limit = 20 }
	if limit > 100 { limit = 100 }
	if offset < 0 { offset = 0 }

	result, err := h.PG.ListOperators(c.Request.Context(), platform, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"total":    result.Total,
		"limit":    limit,
		"offset":   offset,
		"operators": result.Operators,
	})
}

// GET /operators/:id
func (h *OperatorHandler) GetByID(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not connected"})
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	op, err := h.PG.GetOperatorByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "operator not found"})
		return
	}
	c.JSON(http.StatusOK, op)
}
