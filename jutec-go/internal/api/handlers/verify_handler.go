package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"jutec-go/internal/store"

	"github.com/gin-gonic/gin"
)

type VerifyHandler struct{ PG *store.PostgresDB }

// Domain institusi: subdomainnya sering dibajak judol, jadi tidak boleh diblacklist utuh
var reInstitution = regexp.MustCompile(`(?i)\.(go|gov|mil|ac|edu|sch|desa|or)\.[a-z]{2}$|\.(gov|edu|mil)$`)

// POST /sites/:id/verify {"status":"judol|bukan_judol|reset","note":"…","blacklist":false}
func (h *VerifyHandler) Verify(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	var body struct {
		Status    string `json:"status"`
		Note      string `json:"note"`
		Blacklist bool   `json:"blacklist"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body tidak valid"})
		return
	}
	actor := ActorOf(c)
	if actor == "anonymous" {
		c.JSON(http.StatusForbidden, gin.H{"error": "isi nama penyidik dulu (tombol nama di kanan atas) — keputusan verifikasi harus tercatat atas nama seseorang"})
		return
	}
	if err := h.PG.VerifySite(c, id, body.Status, strings.TrimSpace(body.Note), actor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp := gin.H{"ok": true, "status": body.Status}
	if body.Blacklist && body.Status == "bukan_judol" {
		var raw string
		h.PG.Pool.QueryRow(c, `SELECT url FROM sites WHERE id=$1`, id).Scan(&raw)
		u, _ := url.Parse(raw)
		host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
		if reInstitution.MatchString(host) {
			resp["blacklist"] = "dilewati: domain institusi (.go/.gov/.ac/.edu) — subdomainnya sering dibajak judol"
		} else if d := parentDomain(host); d != "" {
			h.PG.Pool.Exec(c, `INSERT INTO domain_blacklist (domain, category) SELECT $1, 'verified'
				WHERE NOT EXISTS (SELECT 1 FROM domain_blacklist WHERE domain = $1)`, d)
			resp["blacklist"] = d
		}
	}
	c.Set("audit_target", fmt.Sprintf("site:%d", id))
	c.JSON(http.StatusOK, resp)
}

// GET /sites/:id/verifications
func (h *VerifyHandler) History(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	list, _ := h.PG.SiteVerificationHistory(c, id)
	c.JSON(http.StatusOK, gin.H{"history": list})
}

// GET /verification/queue?priority=High&sort=weak|confidence|recent&limit=&offset=
func (h *VerifyHandler) Queue(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	list, total, err := h.PG.VerifyQueue(c, c.Query("priority"), c.Query("sort"), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": list, "total": total, "stats": h.PG.VerifyStats(c)})
}

func parentDomain(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return ""
	}
	if l := len(parts); l >= 3 && len(parts[l-2]) <= 3 && len(parts[l-1]) == 2 {
		return strings.Join(parts[l-3:], ".")
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// GET /sites/:id/reachability — riwayat perubahan status keterjangkauan/blokir
func (h *VerifyHandler) Reach(c *gin.Context) {
	id, ok := siteIDParam(c)
	if !ok {
		return
	}
	list, _ := h.PG.ReachHistory(c, id)
	c.JSON(http.StatusOK, gin.H{"history": list})
}
