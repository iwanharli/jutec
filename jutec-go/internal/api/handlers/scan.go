package handlers

import (
	"errors"
	"jutec-go/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ScanHandler struct {
	ScanSvc *service.ScanService
}

func NewScanHandler(svc *service.ScanService) *ScanHandler {
	return &ScanHandler{ScanSvc: svc}
}

func (h *ScanHandler) AnalyzeSite(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	result, err := h.ScanSvc.AnalyzeURL(c.Request.Context(), url, 0)
	if errors.Is(err, service.ErrInvalidURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

type BulkRequest struct {
	URLs []string `json:"urls" binding:"required"`
}

func (h *ScanHandler) AnalyzeBulk(c *gin.Context) {
	var req BulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := h.ScanSvc.AnalyzeBulk(c.Request.Context(), req.URLs, 0)
	c.JSON(http.StatusOK, result)
}
