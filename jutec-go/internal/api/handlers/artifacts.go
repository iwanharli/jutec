package handlers

import (
	"jutec-go/internal/store"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ArtifactHandler struct {
	PG *store.PostgresDB
}

func NewArtifactHandler(pg *store.PostgresDB) *ArtifactHandler {
	return &ArtifactHandler{PG: pg}
}

func (h *ArtifactHandler) dbRequired(c *gin.Context) bool {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return false
	}
	return true
}

// LookupByValue mencari semua situs yang menggunakan artefak yang sama.
// GET /artifacts/lookup?type=Bank&value=1234567890
func (h *ArtifactHandler) LookupByValue(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	artifactType := c.Query("type")
	value := c.Query("value")
	if artifactType == "" || value == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type and value parameters are required"})
		return
	}

	results, err := h.PG.GetSitesByArtifact(c.Request.Context(), artifactType, value)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"search":  gin.H{"type": artifactType, "value": value},
		"count":   len(results),
		"results": results,
	})
}

// TopShared mengembalikan artefak yang dipakai paling banyak situs judol.
// GET /data/artifacts?type=Bank&name=BCA&limit=20
func (h *ArtifactHandler) TopShared(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}

	artifactType := c.Query("type")
	artifactName := c.Query("name")
	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	results, err := h.PG.GetTopArtifactsFiltered(c.Request.Context(), artifactType, artifactName, limit, c.Query("include_inactive") == "1", c.Query("shared") == "1")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count":   len(results),
		"results": results,
	})
}

// GetFilters mengembalikan distinct type + name dari tabel artifacts.
// GET /data/artifacts/filters
func (h *ArtifactHandler) GetFilters(c *gin.Context) {
	if !h.dbRequired(c) {
		return
	}
	filters, err := h.PG.GetArtifactFilters(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"filters": filters})
}
