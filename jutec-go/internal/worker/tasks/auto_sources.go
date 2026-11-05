package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/net/publicsuffix"
	"io/fs"
	"jutec-go/internal/wayback"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"jutec-go/internal/commoncrawl"
	"jutec-go/internal/trustpositif"

	"github.com/hibiken/asynq"
)

const (
	TypeAutoTrustPositif = "auto:trustpositif" // cocokkan semua situs dengan daftar blokir TrustPositif
	TypeAutoCommonCrawl  = "auto:commoncrawl"  // cari halaman judol susupan di arsip Common Crawl
	TypeAutoTakeover     = "auto:takeover"     // domain bekas situs sah yang dibeli operator (riwayat Wayback)
)

func (h *AutoHandler) TrustPositif(ctx context.Context, _ *asynq.Task) error {
	byHost, err := h.PG.SiteHosts(ctx)
	if err != nil || len(byHost) == 0 {
		return err
	}
	hosts := make(map[string]bool, len(byHost))
	for k := range byHost {
		hosts[k] = true
	}
	listed, total, err := trustpositif.Match(ctx, trustpositif.DefaultURL, hosts)
	if err != nil {
		log.Printf("❌ [Auto] TrustPositif: %v", err)
		return err
	}
	var ids []int
	for hst := range listed {
		ids = append(ids, byHost[hst]...)
	}
	n, err := h.PG.SaveTrustPositif(ctx, ids)
	log.Printf("🛡️  [Auto] TrustPositif: %d domain di daftar · %d situs kita terdaftar (%d baru)", total, len(ids), n)
	h.PG.Audit(ctx, autoActor, "", "auto.trustpositif.checked", "", 200, map[string]any{"listed": len(ids), "new": n, "list_size": total})
	return err
}

// Pola domain institusi/organisasi yang ditelusuri bergiliran; regex URL halaman judol.
var ccPatterns = []string{"*.go.id", "*.ac.id", "*.sch.id", "*.desa.id", "*.or.id", "*.ponpes.id"}

const ccRegex = `.*(slot|gacor|togel|toto|maxwin|casino|kasino|judi|sbobet|scatter|zeus|mahjong|bandar).*`

type ccCursor struct {
	Index   string `json:"index"`
	Pattern int    `json:"pattern"`
	Page    int    `json:"page"`
}

// CommonCrawl: tiap putaran membaca maks. 6 halaman indeks (±10 dtk/halaman), lanjut dari posisi
// terakhir. Host baru yang punya halaman judol diantrekan scan (URL halamannya, bukan beranda).
func (h *AutoHandler) CommonCrawl(ctx context.Context, _ *asynq.Task) error {
	var cur ccCursor
	json.Unmarshal([]byte(h.PG.GetSetting(ctx, "cc_cursor")), &cur)
	latest, err := commoncrawl.LatestIndex(ctx)
	if err != nil {
		return nil
	}
	if cur.Index != latest { // koleksi baru → mulai dari awal
		cur = ccCursor{Index: latest}
	}
	if cur.Pattern >= len(ccPatterns) {
		return nil // koleksi ini sudah selesai ditelusuri
	}

	pagesByHost := map[string]string{}
	for i := 0; i < 6 && cur.Pattern < len(ccPatterns); i++ {
		recs, err := commoncrawl.Query(ctx, cur.Index, ccPatterns[cur.Pattern], ccRegex, cur.Page)
		if errors.Is(err, commoncrawl.ErrEnd) {
			cur.Pattern, cur.Page = cur.Pattern+1, 0
			continue
		}
		if err != nil {
			log.Printf("⚠️  [Auto] Common Crawl %s hlm %d: %v", ccPatterns[cur.Pattern], cur.Page, err)
			break // coba lagi putaran berikutnya
		}
		for _, r := range recs {
			if r.Status != "200" {
				continue
			}
			if u, err := url.Parse(r.URL); err == nil {
				hst := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
				if _, ok := pagesByHost[hst]; !ok && u.Path != "" && u.Path != "/" {
					pagesByHost[hst] = r.URL
				}
			}
		}
		cur.Page++
		time.Sleep(2 * time.Second)
	}
	b, _ := json.Marshal(cur)
	h.PG.SetSetting(ctx, "cc_cursor", string(b))

	hosts := make([]string, 0, len(pagesByHost))
	for k := range pagesByHost {
		hosts = append(hosts, k)
	}
	queued := 0
	for _, hst := range h.PG.UnknownHosts(ctx, hosts) {
		if queued >= 30 || h.Client == nil {
			break
		}
		payload, _ := json.Marshal(ScanURLPayload{URL: pagesByHost[hst]})
		if _, err := h.Client.Enqueue(asynq.NewTask(TypeScanURL, payload), asynq.Queue("default"), asynq.MaxRetry(1), asynq.Unique(24*time.Hour)); err == nil {
			queued++
		}
	}
	if len(hosts) > 0 {
		log.Printf("🗄️  [Auto] Common Crawl %s: %d domain berhalaman judol, %d baru diantrekan", cur.Index, len(hosts), queued)
		h.PG.Audit(ctx, autoActor, "", "auto.commoncrawl.found", "", 200, map[string]any{"hosts": len(hosts), "queued": queued})
	}
	return nil
}

// TakeoverCheck: riwayat Wayback domain induk situs judol — domain bekas situs sah yang dibeli operator.
// Maks. 6 domain per putaran (±8 snapshot/domain, sopan terhadap archive.org).
func (h *AutoHandler) TakeoverCheck(ctx context.Context, _ *asynq.Task) error {
	targets, err := h.PG.TakeoverTargets(ctx, 6)
	if err != nil || len(targets) == 0 {
		return nil
	}
	found := 0
	for id, raw := range targets {
		dom := takeoverDomain(raw)
		var res any
		if dom == "" {
			res = map[string]any{"detected": false, "checked_at": time.Now(), "note": "nama domain bertema judol"}
		} else if t, err := wayback.CheckTakeover(dom); err != nil {
			log.Printf("⚠️  [Auto] riwayat domain %s: %v", dom, err)
			continue
		} else {
			res = t
			if t.Detected {
				found++
				log.Printf("🏚️  [Auto] Domain bekas: %s — dulu %q, judol sejak %s", dom, t.LegitTitle, t.JudolSince)
			}
		}
		h.PG.SaveTakeover(ctx, id, res)
	}
	if found > 0 {
		h.PG.Audit(ctx, autoActor, "", "auto.takeover.found", "", 200, map[string]any{"checked": len(targets), "found": found})
	}
	return nil
}

var reJudolName = regexp.MustCompile(`(?i)slot|gacor|toto|togel|bet|casino|judi|poker|4d|88|303|777|maxwin|hoki|qq|domino|lotto|rtp|macau|sgp|hk`)

// takeoverDomain: domain induk yang layak dicek; "" bila namanya sendiri bertema judol (sejak awal milik operator).
func takeoverDomain(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	root, err := publicsuffix.EffectiveTLDPlusOne(strings.TrimPrefix(strings.ToLower(u.Hostname()), "www."))
	if err != nil || reJudolName.MatchString(strings.Split(root, ".")[0]) {
		return ""
	}
	return root
}

const TypeAutoEvidenceRetention = "auto:evidence-retention"

// EvidenceRetention: hapus file bukti situs yang lama tidak aktif sesuai Pengaturan (bawaan 0 = nonaktif).
func (h *AutoHandler) EvidenceRetention(ctx context.Context, _ *asynq.Task) error {
	days := h.PG.GetSettingInt(ctx, "evidence_retention_days", 0)
	if days <= 0 {
		return nil
	}
	items, err := h.PG.EvidenceRetentionCandidates(ctx, days, 200)
	if err != nil || len(items) == 0 {
		return err
	}
	root := os.Getenv("EVIDENCE_DIR")
	if root == "" {
		if home, err := os.UserHomeDir(); err == nil {
			root = filepath.Join(home, ".jutec", "evidence")
		}
	}
	n := 0
	for _, it := range items {
		// Pengaman: hanya hapus folder di dalam direktori bukti
		if rel, err := filepath.Rel(root, it.Dir); err != nil || strings.HasPrefix(rel, "..") || rel == "." {
			continue
		}
		filepath.WalkDir(it.Dir, func(p string, d fs.DirEntry, err error) error {
			if err == nil {
				os.Chmod(p, 0o700) // file bukti dibuat baca-saja
			}
			return nil
		})
		if err := os.RemoveAll(it.Dir); err != nil {
			log.Printf("⚠️  [Retensi] gagal hapus %s: %v", it.Dir, err)
			continue
		}
		h.PG.MarkEvidencePurged(ctx, it.ID, days)
		n++
	}
	log.Printf("🧹 [Auto] Retensi bukti: %d berkas bukti dihapus (situs tidak aktif > %d hari)", n, days)
	h.PG.Audit(ctx, autoActor, "", "auto.evidence.purged", "", 200, map[string]any{"count": n, "days": days})
	return nil
}
