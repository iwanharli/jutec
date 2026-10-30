package handlers

import (
	"net/http"

	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

type GraphHandler struct {
	PG *store.PostgresDB
}

func NewGraphHandler(pg *store.PostgresDB) *GraphHandler {
	return &GraphHandler{PG: pg}
}

// GET /network/graph
func (h *GraphHandler) GetGraph(c *gin.Context) {
	graph, err := h.PG.GetNetworkGraph(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, graph)
}
