package handlers

import (
	"context"
	"io/fs"
	"path/filepath"
	"syscall"
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
	sys := gin.H{"redis": h.Inspector != nil, "queues": []queueStat{}, "servers": 0, "disk": diskInfo()}
	if h.PG != nil {
		ctx := context.Background()
		sys["daily_sweep"] = gin.H{
			"enabled":  h.PG.GetSettingBool(ctx, "sweep_daily_enabled", false),
			"hour":     h.PG.GetSettingInt(ctx, "sweep_daily_hour", 2),
			"keywords": h.PG.GetSettingInt(ctx, "sweep_daily_keywords", 20),
			"last":     h.PG.GetSetting(ctx, "sweep_daily_last"),
		}
	}
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
			// Daftar jadwal otomatis (jenis tugas, interval, kapan berikutnya) untuk ditampilkan apa adanya
			list := []gin.H{}
			for _, e := range entries {
				item := gin.H{"type": e.Task.Type(), "spec": e.Spec, "next_at": e.Next}
				if !e.Prev.IsZero() {
					item["prev_at"] = e.Prev
				}
				list = append(list, item)
			}
			sys["scheduler"] = gin.H{"jobs": len(entries), "next_at": next, "last_at": prev, "entries": list}
		}
	}
	// Kunci API terpasang (tanpa memperlihatkan nilainya). Tanpa kunci, sumber tsb nonaktif atau berkuota gratis kecil.
	keys := []gin.H{}
	for _, d := range store.APIKeyDefs {
		if d.Group == "telegram" {
			continue
		}
		keys = append(keys, gin.H{"name": d.Label, "set": os.Getenv(d.Env) != "", "required": d.Required, "free_tier": d.FreeTier})
	}
	sys["api_keys"] = keys
	return sys
}

// diskInfo: ruang disk tempat bukti forensik disimpan + ukuran folder bukti — penyebab utama pemakaian disk.
func diskInfo() gin.H {
	dir := os.Getenv("EVIDENCE_DIR")
	if dir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, ".jutec", "evidence")
		}
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return nil
	}
	var evidence int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, e := d.Info(); e == nil {
				evidence += fi.Size()
			}
		}
		return nil
	})
	const gb = 1 << 30
	return gin.H{
		"free_gb":     float64(st.Bavail*uint64(st.Bsize)) / gb,
		"total_gb":    float64(st.Blocks*uint64(st.Bsize)) / gb,
		"evidence_mb": float64(evidence) / (1 << 20),
		"low":         st.Bavail*uint64(st.Bsize) < 5*gb, // <5 GB: scan & bukti forensik bisa gagal
	}
}
