package handlers

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"jutec-go/internal/ipintel"
	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

type IPHandler struct {
	PG         *store.PostgresDB
	URLScanKey string
}

const ipIntelTTL = 7 * 24 * time.Hour

// intel: dari cache bila masih segar, selain itu ambil ulang & simpan.
func (h *IPHandler) intel(ctx context.Context, ip string, refresh bool) (*ipintel.Intel, error) {
	if !refresh {
		if in, at, err := h.PG.GetIPIntel(ctx, ip); err == nil && time.Since(at) < ipIntelTTL {
			return in, nil
		}
	}
	in, err := ipintel.Lookup(ctx, ip)
	if err != nil {
		return nil, err
	}
	h.PG.SaveIPIntel(ctx, in)
	return in, nil
}

type ipRow struct {
	IP          string          `json:"ip"`
	Intel       *ipintel.Intel  `json:"intel"`
	Error       string          `json:"error,omitempty"`
	SharedSites int             `json:"shared_sites"`
	Sites       []store.SiteRef `json:"sites"`
}

// intelMany: data beberapa IP sekaligus (paralel terbatas), plus situs lain di IP yang sama.
func (h *IPHandler) intelMany(ctx context.Context, ips []string, excludeSite int, refresh bool) []ipRow {
	out := make([]ipRow, len(ips))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, ip := range ips {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, ip string) {
			defer wg.Done()
			defer func() { <-sem }()
			r := ipRow{IP: ip, Sites: []store.SiteRef{}}
			if in, err := h.intel(ctx, ip, refresh); err != nil {
				r.Error = err.Error()
			} else {
				r.Intel = in
			}
			r.Sites, r.SharedSites, _ = h.PG.SitesOnIP(ctx, ip, excludeSite, 10)
			out[i] = r
		}(i, ip)
	}
	wg.Wait()
	return out
}

// GET /ips/:ip?refresh=1
func (h *IPHandler) Get(c *gin.Context) {
	ip := strings.TrimSpace(c.Param("ip"))
	if net.ParseIP(ip) == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "alamat IP tidak valid"})
		return
	}
	c.Set("audit_target", "ip:"+ip)
	rows := h.intelMany(c.Request.Context(), []string{ip}, 0, c.Query("refresh") == "1")
	r := rows[0]
	if r.Intel == nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": r.Error})
		return
	}
	sites, total, _ := h.PG.SitesOnIP(c, ip, 0, 100)
	history, _ := h.PG.IPIntelHistory(c, ip)
	c.JSON(http.StatusOK, gin.H{"intel": r.Intel, "sites": sites, "sites_total": total, "history": history})
}

// GET /sites/:id/infra — IP situs + intel-nya + hasil pencarian IP asli terakhir.
func (h *IPHandler) SiteInfra(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	ips, err := h.PG.SiteIPs(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(ips) > 8 {
		ips = ips[:8]
	}
	resp := gin.H{"ips": h.intelMany(c.Request.Context(), ips, id, c.Query("refresh") == "1"), "origin": nil, "origin_ips": []ipRow{}}
	if rep, err := h.PG.GetSiteOrigin(c, id); err == nil {
		resp["origin"] = rep
		resp["origin_ips"] = h.intelMany(c.Request.Context(), candidateIPs(rep), id, false)
	}
	c.JSON(http.StatusOK, resp)
}

// POST /sites/:id/origin — cari kandidat IP asli di balik CDN (±20–60 detik).
func (h *IPHandler) FindOrigin(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	var raw string
	if h.PG.Pool.QueryRow(c, `SELECT url FROM sites WHERE id = $1`, id).Scan(&raw) != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "situs tidak ditemukan"})
		return
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL situs tidak valid"})
		return
	}
	if net.ParseIP(u.Hostname()) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "situs diakses langsung lewat IP — tidak ada CDN untuk ditembus"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
	defer cancel()
	rep, err := ipintel.FindOrigin(ctx, u.Hostname(), h.URLScanKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	h.PG.SaveSiteOrigin(c, id, rep)
	c.JSON(http.StatusOK, gin.H{"origin": rep, "origin_ips": h.intelMany(ctx, candidateIPs(rep), id, false)})
}

func candidateIPs(rep *ipintel.OriginReport) []string {
	var ips []string
	for _, cnd := range rep.Candidates {
		ips = append(ips, cnd.IP)
		if len(ips) == 8 {
			break
		}
	}
	return ips
}
