package handlers

import (
	"jutec-go/internal/store"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type BlacklistHandler struct {
	PG *store.PostgresDB
}

func NewBlacklistHandler(pg *store.PostgresDB) *BlacklistHandler {
	return &BlacklistHandler{PG: pg}
}

func (h *BlacklistHandler) dbRequired(c *gin.Context) bool {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return false
	}
	return true
}

// GET /blacklist?active=true
func (h *BlacklistHandler) List(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	activeOnly := c.Query("active") != "false"
	domains, err := h.PG.ListBlacklistDomains(c.Request.Context(), activeOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(domains), "domains": domains})
}

// POST /blacklist
func (h *BlacklistHandler) Create(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	var req struct {
		Domain   string `json:"domain" binding:"required"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	d, err := h.PG.CreateBlacklistDomain(c.Request.Context(), req.Domain, req.Category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, d)
}

// PUT /blacklist/:id
func (h *BlacklistHandler) Update(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		IsActive *bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	if err := h.PG.UpdateBlacklistDomain(c.Request.Context(), id, isActive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// DELETE /blacklist/:id
func (h *BlacklistHandler) Delete(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.PG.DeleteBlacklistDomain(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
