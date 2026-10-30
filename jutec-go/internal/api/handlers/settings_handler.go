package handlers

import (
	"log"
	"net/http"
	"os"
	"syscall"
	"time"

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

// GET /settings/api-keys — status kunci API (nilai disamarkan)
func (h *SettingsHandler) APIKeys(c *gin.Context) {
	c.JSON(http.StatusOK, h.PG.APIKeyStatuses(c.Request.Context()))
}

// PUT /settings/api-keys/:env  body: {"value":"..."} — kosong = hapus (kembali ke .env)
func (h *SettingsHandler) SetAPIKey(c *gin.Context) {
	env := c.Param("env")
	if store.APIKeyDefByEnv(env) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "kunci tidak dikenal"})
		return
	}
	var body struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.PG.SetAPIKey(c.Request.Context(), env, body.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Set("audit_target", "apikey:"+env)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /settings/apply — terapkan kunci baru: proses backend memulai ulang dirinya (±5 detik).
// Tugas latar yang sedang berjalan akan diulang otomatis oleh antrean.
func (h *SettingsHandler) Apply(c *gin.Context) {
	exe, err := os.Executable()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"restarting": true})
	go func() {
		time.Sleep(500 * time.Millisecond)
		log.Println("🔄 Menerapkan pengaturan baru: memulai ulang backend…")
		if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
			log.Printf("❌ Gagal memulai ulang: %v", err)
		}
	}()
}
