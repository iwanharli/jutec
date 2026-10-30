package handlers

import (
	"net/http"

	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	PG *store.PostgresDB
}

func NewSettingsHandler(pg *store.PostgresDB) *SettingsHandler {
	return &SettingsHandler{PG: pg}
}

// GET /settings
func (h *SettingsHandler) List(c *gin.Context) {
	settings, err := h.PG.GetAllSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// PATCH /settings/:key  body: {"value":"3"}
func (h *SettingsHandler) Update(c *gin.Context) {
	key := c.Param("key")
	var body struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.PG.SetSetting(c.Request.Context(), key, body.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "value": body.Value})
}
