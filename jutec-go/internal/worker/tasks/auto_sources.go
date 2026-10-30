package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"strings"
	"time"

	"jutec-go/internal/commoncrawl"
	"jutec-go/internal/trustpositif"

	"github.com/hibiken/asynq"
)

const (
	TypeAutoTrustPositif = "auto:trustpositif" // cocokkan semua situs dengan daftar blokir TrustPositif
	TypeAutoCommonCrawl  = "auto:commoncrawl"  // cari halaman judol susupan di arsip Common Crawl
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
