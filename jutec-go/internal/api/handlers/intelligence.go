package handlers

import (
	"jutec-go/internal/store"
	"net/http"

	"github.com/gin-gonic/gin"
)

type IntelligenceHandler struct {
	DB *store.Neo4jDB
	PG *store.PostgresDB
}

func NewIntelligenceHandler(database *store.Neo4jDB, pg *store.PostgresDB) *IntelligenceHandler {
	return &IntelligenceHandler{DB: database, PG: pg}
}

// GetNetworkSummary returns top NS and top artifacts for the discovery dashboard
func (h *IntelligenceHandler) GetNetworkSummary(c *gin.Context) {
	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Postgres database not connected"})
		return
	}

	topNS, err := h.PG.GetTopSharedNameservers(c.Request.Context(), 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	topArtifacts, err := h.PG.GetTopArtifacts(c.Request.Context(), "", 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"top_ns":        topNS,
		"top_artifacts": topArtifacts,
	})
}

// Lookup finds sites connected to an artifact using PostgreSQL.
// type=Contact|Bank|Crypto|E-Wallet|etc, v=search value (partial match).
func (h *IntelligenceHandler) Lookup(c *gin.Context) {
	attrType := c.Query("type")
	attrValue := c.Query("v")

	if attrValue == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "v parameter is required"})
		return
	}

	if h.PG == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not connected"})
		return
	}

	sites, err := h.PG.GetSitesByLookup(c.Request.Context(), attrType, attrValue)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if sites == nil {
		sites = []map[string]interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{
		"sites": sites,
		"count": len(sites),
	})
}

func (h *IntelligenceHandler) ReverseLookup(c *gin.Context) {
	attrType := c.Query("type")
	attrValue := c.Query("value")

	if attrType == "" || attrValue == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type and value parameters are required"})
		return
	}

	if h.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Neo4j database not connected"})
		return
	}

	sites, err := h.DB.GetConnectedSites(c.Request.Context(), attrType, attrValue)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"search": gin.H{
			"type":  attrType,
			"value": attrValue,
		},
		"connected_sites": sites,
		"count":           len(sites),
	})
}
