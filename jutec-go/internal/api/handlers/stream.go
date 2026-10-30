package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"jutec-go/internal/api/middleware"
	"jutec-go/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ProgressEvent adalah format pesan progress yang dikirim ke client.
type ProgressEvent struct {
	Type      string      `json:"type"`    // step, result, error, done
	Step      string      `json:"step"`    // crawling, whois, scoring, dll
	Message   string      `json:"message"` // deskripsi singkat
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"ts"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// Hanya dashboard lokal (localhost port apa pun) yang boleh membuka WS; request tanpa Origin (curl/CLI) tetap diizinkan.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || middleware.LocalOrigin.MatchString(origin)
	},
}

type StreamHandler struct {
	ScanSvc *service.ScanService
}

func NewStreamHandler(svc *service.ScanService) *StreamHandler {
	return &StreamHandler{ScanSvc: svc}
}

// WS godoc
// GET /ws/analyze?url=https://...&depth=0
// Upgrade ke WebSocket, stream progress scan, kirim result di akhir.
func (h *StreamHandler) AnalyzeWS(c *gin.Context) {
	rawURL := c.Query("url")
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter required"})
		return
	}
	depth := 0
	if c.Query("depth") == "1" {
		depth = 1
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}
	defer conn.Close()

	send := func(evType, step, message string, data interface{}) {
		evt := ProgressEvent{
			Type:      evType,
			Step:      step,
			Message:   message,
			Data:      data,
			Timestamp: time.Now().UnixMilli(),
		}
		b, _ := json.Marshal(evt)
		if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
			log.Printf("[WS] Write error: %v", err)
		}
	}

	send("start", "init", fmt.Sprintf("Memulai analisis: %s", rawURL), nil)

	ctx := c.Request.Context()
	result, err := h.ScanSvc.AnalyzeURLStreaming(ctx, rawURL, depth, func(step, msg string, data interface{}) {
		send("step", step, msg, data)
	})

	if err != nil {
		send("error", "failed", err.Error(), nil)
		return
	}

	send("done", "complete", "Analisis selesai", result)
}

// SSE godoc
// GET /sse/analyze?url=https://...&depth=0
// Server-Sent Events fallback — stream progress sebagai text/event-stream.
func (h *StreamHandler) AnalyzeSSE(c *gin.Context) {
	rawURL := c.Query("url")
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter required"})
		return
	}
	depth := 0
	if c.Query("depth") == "1" {
		depth = 1
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	send := func(evType, step, message string, data interface{}) {
		evt := ProgressEvent{
			Type:      evType,
			Step:      step,
			Message:   message,
			Data:      data,
			Timestamp: time.Now().UnixMilli(),
		}
		b, _ := json.Marshal(evt)
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(b))
		flusher.Flush()
	}

	send("start", "init", fmt.Sprintf("Memulai analisis: %s", rawURL), nil)

	ctx := c.Request.Context()
	result, err := h.ScanSvc.AnalyzeURLStreaming(ctx, rawURL, depth, func(step, msg string, data interface{}) {
		send("step", step, msg, data)
	})

	if err != nil {
		send("error", "failed", err.Error(), nil)
		return
	}

	send("done", "complete", "Analisis selesai", result)
}
