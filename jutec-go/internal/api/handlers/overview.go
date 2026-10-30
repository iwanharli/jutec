package handlers

import (
	"net/http"
	"os"
	"time"

	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

// OverviewHandler menyajikan ringkasan dashboard + kesehatan sistem (antrean worker & kunci API).
type OverviewHandler struct {
	PG        *store.PostgresDB
	Inspector *asynq.Inspector // nil bila Redis tidak dikonfigurasi
}

type queueStat struct {
	Name      string `json:"name"`
	Pending   int    `json:"pending"`
	Active    int    `json:"active"`
	Scheduled int    `json:"scheduled"`
	Retry     int    `json:"retry"`
	Failed    int    `json:"failed"` // diarsipkan setelah gagal berulang
	Processed int    `json:"processed_today"`
	FailedDay int    `json:"failed_today"`
	Paused    bool   `json:"paused"`
}

// GET /data/overview
func (h *OverviewHandler) Get(c *gin.Context) {
	o, err := h.PG.GetOverview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"overview": o, "system": h.system()})
}

func (h *OverviewHandler) system() gin.H {
	sys := gin.H{"redis": h.Inspector != nil, "queues": []queueStat{}, "servers": 0}
	if h.Inspector != nil {
		names, err := h.Inspector.Queues()
		if err != nil {
			sys["redis"] = false
			sys["error"] = err.Error()
		}
		qs := []queueStat{}
		for _, n := range names {
			if info, err := h.Inspector.GetQueueInfo(n); err == nil {
				qs = append(qs, queueStat{Name: n, Pending: info.Pending, Active: info.Active, Scheduled: info.Scheduled,
					Retry: info.Retry, Failed: info.Archived, Processed: info.Processed, FailedDay: info.Failed, Paused: info.Paused})
			}
		}
		sys["queues"] = qs
		if srv, err := h.Inspector.Servers(); err == nil {
			sys["servers"] = len(srv)
		}
		// Jadwal otomatis berikutnya & terakhir
		if entries, err := h.Inspector.SchedulerEntries(); err == nil {
			var next, prev *time.Time
			for _, e := range entries {
				if next == nil || e.Next.Before(*next) {
					t := e.Next
					next = &t
				}
				if !e.Prev.IsZero() && (prev == nil || e.Prev.After(*prev)) {
					t := e.Prev
					prev = &t
				}
			}
			sys["scheduler"] = gin.H{"jobs": len(entries), "next_at": next, "last_at": prev}
		}
	}
	// Kunci API terpasang (tanpa memperlihatkan nilainya). Tanpa kunci, sumber tsb nonaktif atau berkuota gratis kecil.
	sys["api_keys"] = []gin.H{
		{"name": "Serper (Google)", "set": os.Getenv("SERPER_API_KEY") != "", "required": true},
		{"name": "SerpAPI (cadangan Google)", "set": os.Getenv("SERPAPI_API_KEY") != ""},
		{"name": "URLScan", "set": os.Getenv("URLSCAN_API_KEY") != ""},
		{"name": "VirusTotal", "set": os.Getenv("VIRUSTOTAL_API_KEY") != ""},
		{"name": "LeakOSINT", "set": os.Getenv("LEAKOSINT_TOKEN") != ""},
		{"name": "HackerTarget", "set": os.Getenv("HACKERTARGET_API_KEY") != "", "free_tier": true},
		{"name": "TronGrid", "set": os.Getenv("TRONGRID_API_KEY") != "", "free_tier": true},
	}
	return sys
}
