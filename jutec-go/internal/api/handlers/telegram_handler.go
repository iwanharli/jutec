package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"jutec-go/internal/service"
	"jutec-go/internal/store"
	"jutec-go/internal/telegram"

	"github.com/gin-gonic/gin"
)

type TelegramHandler struct {
	Svc *service.TelegramService
	PG  *store.PostgresDB
}

func NewTelegramHandler(svc *service.TelegramService, pg *store.PostgresDB) *TelegramHandler {
	return &TelegramHandler{Svc: svc, PG: pg}
}

// POST /telegram/resolve?username=nagahokiofficial
func (h *TelegramHandler) Resolve(c *gin.Context) {
	username := strings.TrimPrefix(c.Query("username"), "@")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}

	ch, err := h.Svc.ResolveAndSave(c.Request.Context(), username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ch)
}

// POST /telegram/scrape?username=nagahokiofficial&limit=50
func (h *TelegramHandler) Scrape(c *gin.Context) {
	username := strings.TrimPrefix(c.Query("username"), "@")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	count, links, err := h.Svc.ScrapeMessages(c.Request.Context(), username, limit)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, telegram.ErrNotLoggedIn) {
			status = http.StatusServiceUnavailable
		} else if errors.Is(err, telegram.ErrNotChannel) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	if links == nil {
		links = []string{}
	}
	c.JSON(http.StatusOK, gin.H{
		"username":        username,
		"artifacts_found": count,
		"links":           links,
	})
}

// POST /telegram/resolve-pending — resolve semua username dari artifacts yang belum di-resolve
func (h *TelegramHandler) ResolvePending(c *gin.Context) {
	resolved, err := h.Svc.ResolveAllPending(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"resolved": resolved})
}

// GET /telegram/pending-count
func (h *TelegramHandler) PendingCount(c *gin.Context) {
	usernames, err := h.PG.GetUnresolvedTelegramUsernames(c.Request.Context(), 9999)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(usernames)})
}

// GET /telegram/channels
func (h *TelegramHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	channels, total, err := h.PG.ListTelegramChannels(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"total":    total,
		"channels": channels,
	})
}

// GET /telegram/messages?username=…&limit=20 — pesan terbaru yang tersimpan dari channel
func (h *TelegramHandler) Messages(c *gin.Context) {
	username := strings.TrimPrefix(c.Query("username"), "@")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	msgs, err := h.PG.TelegramRecentMessages(c.Request.Context(), username, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if msgs == nil {
		msgs = []map[string]any{}
	}
	c.JSON(http.StatusOK, gin.H{"messages": msgs})
}
