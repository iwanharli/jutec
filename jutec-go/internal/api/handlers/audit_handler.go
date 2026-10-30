package handlers

import (
	"net/http"
	"strconv"

	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

type AuditHandler struct{ PG *store.PostgresDB }

// GET /audit?q=&target=&limit=&offset=
func (h *AuditHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	list, total, err := h.PG.ListAudit(c, c.Query("q"), c.Query("action"), c.Query("target"), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": list, "total": total})
}

// GET /audit/verify — periksa keutuhan seluruh rantai hash.
func (h *AuditHandler) Verify(c *gin.Context) {
	n, broken, err := h.PG.VerifyAuditChain(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"checked": n, "intact": broken == 0, "broken_at": broken})
}
