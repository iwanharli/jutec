package handlers

import (
	"errors"
	"jutec-go/internal/store"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type KeywordHandler struct {
	PG *store.PostgresDB
}

func NewKeywordHandler(pg *store.PostgresDB) *KeywordHandler {
	return &KeywordHandler{PG: pg}
}

func (h *KeywordHandler) dbRequired(c *gin.Context) bool {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return false
	}
	return true
}

// --- Detection Keywords ---

func (h *KeywordHandler) ListDetection(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	activeOnly := c.Query("active") != "false"
	keywords, err := h.PG.ListDetectionKeywords(c.Request.Context(), activeOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(keywords), "keywords": keywords})
}

func (h *KeywordHandler) CreateDetection(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	var req struct {
		Keyword string  `json:"keyword" binding:"required"`
		Weight  float64 `json:"weight"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	if req.Keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "keyword tidak boleh kosong"})
		return
	}
	if req.Weight == 0 {
		req.Weight = 0.2
	}
	if req.Weight < 0 || req.Weight > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "weight harus antara 0 dan 1"})
		return
	}
	kw, err := h.PG.CreateDetectionKeyword(c.Request.Context(), req.Keyword, req.Weight)
	if err != nil {
		writeDBError(c, err)
		return
	}
	c.JSON(http.StatusCreated, kw)
}

func (h *KeywordHandler) UpdateDetection(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Weight   *float64 `json:"weight"`
		IsActive *bool    `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	if req.Weight != nil && (*req.Weight < 0 || *req.Weight > 1) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "weight harus antara 0 dan 1"})
		return
	}
	if err := h.PG.UpdateDetectionKeyword(c.Request.Context(), id, req.Weight, isActive); err != nil {
		writeDBError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *KeywordHandler) DeleteDetection(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.PG.DeleteDetectionKeyword(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// --- Search Keywords ---

func (h *KeywordHandler) ListSearch(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	activeOnly := c.Query("active") != "false"
	keywords, err := h.PG.ListSearchKeywords(c.Request.Context(), activeOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(keywords), "keywords": keywords})
}

func (h *KeywordHandler) CreateSearch(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	var req struct {
		Keyword string `json:"keyword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	if req.Keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "keyword tidak boleh kosong"})
		return
	}
	kw, err := h.PG.CreateSearchKeyword(c.Request.Context(), req.Keyword)
	if err != nil {
		writeDBError(c, err)
		return
	}
	c.JSON(http.StatusCreated, kw)
}

func (h *KeywordHandler) UpdateSearch(c *gin.Context) {
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
	if err := h.PG.UpdateSearchKeyword(c.Request.Context(), id, isActive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *KeywordHandler) DeleteSearch(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.PG.DeleteSearchKeyword(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// writeDBError memetakan error DB ke status HTTP: duplikat → 409, tidak ditemukan → 404.
func writeDBError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		c.JSON(http.StatusConflict, gin.H{"error": "data sudah ada"})
	case errors.Is(err, pgx.ErrNoRows):
		c.JSON(http.StatusNotFound, gin.H{"error": "data tidak ditemukan"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// GET /keywords/detection/stats — ketepatan tiap kata kunci menurut situs yang sudah diverifikasi
func (h *KeywordHandler) DetectionStats(c *gin.Context) {
	res, err := h.PG.KeywordStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}
