package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"jutec-go/internal/hackertarget"
	"jutec-go/internal/store"
	"jutec-go/internal/tron"
	"jutec-go/internal/worker/tasks"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

// ExternalIntelHandler: pengayaan dari sumber eksternal — reverse analytics (HackerTarget) & jejak USDT (TronGrid).
type ExternalIntelHandler struct {
	PG    *store.PostgresDB
	HT    *hackertarget.Client
	Tron  *tron.Client
	Tasks *asynq.Client
}

const (
	reverseRefreshAfter = 7 * 24 * time.Hour // hasil reverse lookup jarang berubah; hemat kuota harian
	cryptoCacheFor      = 6 * time.Hour
	cryptoMaxPages      = 5 // ≤1.000 transfer terbaru per alamat
)

// GET /sites/:id/reverse-analytics — hasil tersimpan, tanpa memakai kuota.
func (h *ExternalIntelHandler) ReverseAnalytics(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	res, err := h.PG.ReverseAnalyticsForSite(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": res})
}

// POST /sites/:id/reverse-analytics?queue=1 — tanya HackerTarget untuk ID yang belum dicek 7 hari terakhir,
// lalu (opsional) antrekan analisis domain yang belum ada di Jutec.
func (h *ExternalIntelHandler) RefreshReverseAnalytics(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	ids, err := h.PG.SiteAnalyticsIDs(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "situs ini tidak punya ID Google Analytics / GTM / AdSense"})
		return
	}
	asked, quota := 0, false
	for _, t := range ids {
		if quota || (c.Query("force") != "1" && h.PG.ReverseAnalyticsFetchedWithin(ctx, t.Value, reverseRefreshAfter)) {
			continue
		}
		domains, err := h.HT.ReverseAnalytics(t.Value)
		if errors.Is(err, hackertarget.ErrQuota) {
			quota = true // jangan simpan sebagai galat ID — coba lagi besok
			continue
		}
		asked++
		h.PG.SaveReverseAnalytics(ctx, t.Value, domains, err)
	}

	res, err := h.PG.ReverseAnalyticsForSite(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	queued := []string{}
	if c.Query("queue") == "1" && h.Tasks != nil {
		seen := map[string]bool{}
		for _, r := range res {
			for _, d := range r.Domains {
				if d.Site != nil || seen[d.Domain] || len(queued) >= 50 {
					continue
				}
				seen[d.Domain] = true
				payload, _ := json.Marshal(map[string]string{"url": "https://" + d.Domain})
				if _, err := h.Tasks.Enqueue(asynq.NewTask(tasks.TypeScanURL, payload, asynq.MaxRetry(1)),
					asynq.Queue("default"), asynq.Unique(24*time.Hour)); err == nil {
					queued = append(queued, d.Domain)
				}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"results": res, "asked": asked, "quota_exceeded": quota, "queued": queued})
}

type cryptoEntry struct {
	Address string          `json:"address"`
	Profile json.RawMessage `json:"profile"`
	Error   string          `json:"error,omitempty"`
	// Lawan transaksi yang juga tercantum sebagai dompet di situs lain di Jutec → kemungkinan operator sama
	LinkedSites map[string][]store.SiteRef `json:"linked_sites"`
}

// GET /sites/:id/crypto?refresh=1 — profil transaksi USDT-TRC20 setiap alamat situs.
func (h *ExternalIntelHandler) Crypto(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	addrs, err := h.PG.SiteTRONAddresses(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := []cryptoEntry{}
	for _, a := range addrs {
		e := cryptoEntry{Address: a, LinkedSites: map[string][]store.SiteRef{}}
		maxAge := cryptoCacheFor
		if c.Query("refresh") == "1" {
			maxAge = 0
		}
		if raw := h.PG.CachedCryptoProfile(ctx, a, maxAge); raw != nil {
			e.Profile = raw
		} else if p, err := h.Tron.USDTProfile(a, cryptoMaxPages); err != nil {
			e.Error = err.Error()
			if raw := h.PG.CachedCryptoProfile(ctx, a, 365*24*time.Hour); raw != nil {
				e.Profile = raw // pakai data lama daripada kosong
			}
		} else {
			h.PG.SaveCryptoProfile(ctx, a, p)
			e.Profile, _ = json.Marshal(p)
		}
		if e.Profile != nil {
			var p tron.Profile
			json.Unmarshal(e.Profile, &p)
			var cps []string
			for _, x := range append(p.TopSenders, p.TopReceivers...) {
				cps = append(cps, x.Address)
			}
			if linked, err := h.PG.SitesByCryptoAddress(ctx, cps, id); err == nil {
				e.LinkedSites = linked
			}
		}
		out = append(out, e)
	}
	c.JSON(http.StatusOK, gin.H{"wallets": out})
}
