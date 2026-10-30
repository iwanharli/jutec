package tasks

import (
	"context"
	"errors"
	"encoding/json"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"

	"jutec-go/internal/crawler"
	"jutec-go/internal/discovery"
	"jutec-go/internal/service"
	"jutec-go/internal/store"
	"jutec-go/internal/telegram"

	"github.com/hibiken/asynq"
)

const (
	TypeMirrorWatch     = "mirror:watch"     // cari domain mirror brand judol via CT Logs (terjadwal)
	TypeTelegramResolve = "telegram:resolve" // resolve username Telegram tertunda (terjadwal)
	TypeTelegramScrape  = "telegram:scrape"  // baca pesan channel/grup publik & antrekan domain baru (terjadwal)
)

// ── Mirror watch ─────────────────────────────────────────────────────

type MirrorWatchHandler struct {
	PG     *store.PostgresDB
	CT     *discovery.CertScraper
	Client *asynq.Client
}

var (
	reDigitsTail = regexp.MustCompile(`^(.*\d)[a-z]{0,4}$`) // indo303a → indo303
	commonWords  = map[string]bool{"slot": true, "casino": true, "login": true, "daftar": true, "games": true, "mobile": true, "official": true}
)

// BrandOf menurunkan kata kunci brand dari host: "m.indo303a.site" → "indo303".
func BrandOf(host string) string {
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	label := strings.Split(host, ".")[0]
	if label == "m" || label == "mobile" {
		parts := strings.Split(host, ".")
		if len(parts) > 2 {
			label = parts[1]
		}
	}
	if m := reDigitsTail.FindStringSubmatch(label); m != nil {
		label = m[1]
	}
	if len(label) < 5 || commonWords[label] || strings.Contains(label, "-") && len(label) > 30 {
		return ""
	}
	return label
}

// WatchBrand mencari domain baru yang memuat brand di CT Logs lalu mengantrekan analisisnya.
func (h *MirrorWatchHandler) WatchBrand(ctx context.Context, brand string, limit int) ([]string, error) {
	domains, err := h.CT.Search(brand)
	if err != nil {
		return nil, err
	}
	var queued []string
	for _, d := range domains {
		d = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(d)), "*.")
		if !strings.Contains(d, brand) || strings.ContainsAny(d, " *") {
			continue
		}
		u := "https://" + d
		if h.PG.ScannedWithin(ctx, u, 30*24*time.Hour) || h.PG.GetSiteID(ctx, u) > 0 {
			continue
		}
		payload, _ := json.Marshal(map[string]string{"url": u})
		if _, err := h.Client.Enqueue(asynq.NewTask(TypeScanURL, payload, asynq.MaxRetry(1)), asynq.Queue("default"),
			asynq.Unique(24*time.Hour)); err == nil {
			queued = append(queued, u)
		}
		if len(queued) >= limit {
			break
		}
	}
	return queued, nil
}

func (h *MirrorWatchHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	rows, err := h.PG.Pool.Query(ctx, `
		SELECT url FROM sites WHERE is_gambling AND priority = 'High' AND last_scanned_at > NOW() - INTERVAL '30 days'
		ORDER BY confidence DESC, last_scanned_at DESC LIMIT 300`)
	if err != nil {
		return nil
	}
	brands := map[string]bool{}
	var order []string
	for rows.Next() {
		var raw string
		if rows.Scan(&raw) != nil {
			continue
		}
		if u, err := url.Parse(raw); err == nil {
			if b := BrandOf(u.Hostname()); b != "" && !brands[b] {
				brands[b] = true
				order = append(order, b)
			}
		}
	}
	rows.Close()

	total := 0
	for i, b := range order {
		if i == 15 || total >= 40 { // batasi beban crt.sh & antrean per putaran
			break
		}
		q, err := h.WatchBrand(ctx, b, 10)
		if err != nil {
			log.Printf("  ⚠️  [Mirror] %s: %v", b, err)
			continue
		}
		if len(q) > 0 {
			log.Printf("  🪞 [Mirror] brand %q: %d domain baru diantrekan %v", b, len(q), q)
		}
		total += len(q)
		time.Sleep(3 * time.Second) // sopan terhadap crt.sh
	}
	log.Printf("🪞 [Mirror] Selesai: %d brand diperiksa, %d domain baru diantrekan", min(len(order), 15), total)
	return nil
}

// ── Telegram ─────────────────────────────────────────────────────────

type TelegramResolveHandler struct {
	Svc *service.TelegramService
}

func (h *TelegramResolveHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	n, err := h.Svc.ResolveAllPending(ctx)
	if err != nil {
		log.Printf("⚠️  [Telegram] Resolve terjadwal gagal (cek sesi/API Telegram): %v", err)
		return nil
	}
	log.Printf("✈️  [Telegram] %d username di-resolve", n)
	return nil
}

// TelegramScrapeHandler membaca channel/grup publik (tanpa bergabung) yang paling lama tak dibaca,
// lalu mengantrekan analisis domain baru yang diumumkan di pesannya (link mirror/daftar terbaru).
type TelegramScrapeHandler struct {
	Svc    *service.TelegramService
	PG     *store.PostgresDB
	Client *asynq.Client
}

// Domain platform umum — bukan situs operator
var reSocialHost = regexp.MustCompile(`(?i)(^|\.)(t\.me|telegram\.(me|org|dog)|wa\.me|whatsapp\.com|google\.|youtube\.com|youtu\.be|instagram\.com|facebook\.com|fb\.me|tiktok\.com|twitter\.com|x\.com|threads\.net|linktr\.ee)$`)

func (h *TelegramScrapeHandler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	channels, err := h.PG.TelegramChannelsToScrape(ctx, 10)
	if err != nil || len(channels) == 0 {
		return nil
	}
	queued := 0
	for _, ch := range channels {
		_, links, err := h.Svc.ScrapeMessages(ctx, ch, 50)
		if err != nil {
			if errors.Is(err, telegram.ErrNotLoggedIn) || telegram.IsFlood(err) {
				log.Printf("⚠️  [Telegram] Pembacaan dihentikan: %v", err)
				break
			}
			log.Printf("⚠️  [Telegram] @%s: %v", ch, err)
			continue
		}
		seen := map[string]bool{}
		for _, l := range links {
			u, err := url.Parse(l)
			if err != nil || u.Hostname() == "" {
				continue
			}
			host := strings.ToLower(u.Hostname())
			if reSocialHost.MatchString(host) || seen[host] {
				continue
			}
			seen[host] = true
			target := l // shortener/link daftar dipertahankan utuh — crawler mengikuti redirect-nya
			if !crawlerIsShortener(l) {
				target = u.Scheme + "://" + u.Host
			}
			if h.PG.ScannedWithin(ctx, target, 7*24*time.Hour) || queued >= 15 {
				continue
			}
			payload, _ := json.Marshal(map[string]string{"url": target})
			if _, err := h.Client.Enqueue(asynq.NewTask(TypeScanURL, payload, asynq.MaxRetry(1)), asynq.Queue("default"),
				asynq.Unique(24*time.Hour)); err == nil {
				queued++
				log.Printf("  📨 [Telegram] @%s mengumumkan %s → diantrekan", ch, target)
			}
		}
		time.Sleep(3 * time.Second) // hindari FLOOD_WAIT
	}
	log.Printf("📨 [Telegram] %d channel dibaca, %d domain baru diantrekan", len(channels), queued)
	return nil
}

func crawlerIsShortener(u string) bool { return crawler.IsShortenerURL(u) }

// ── VflOSINT (bot Telegram) ──────────────────────────────────────────

const (
	TypeVflChain   = "leak:vfl"         // cek satu nomor judol berantai (nomor → NIK → nomor lain)
	TypeVflPending = "leak:vfl-pending" // cek otomatis nomor judol yang belum pernah dicek (terjadwal)
)

type VflPayload struct {
	ArtifactID int    `json:"artifact_id"`
	Phone      string `json:"phone"`
}

type VflHandler struct {
	Svc *service.VflService
}

func (h *VflHandler) ProcessChain(ctx context.Context, t *asynq.Task) error {
	var p VflPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil
	}
	res, err := h.Svc.Chain(ctx, p.ArtifactID, p.Phone)
	if err != nil {
		log.Printf("⚠️  [VflOSINT] rantai %s gagal: %v", p.Phone, err)
		return nil
	}
	log.Printf("🕵️  [VflOSINT] rantai %s selesai: %d kueri, %d data", p.Phone, res.Queries, len(res.Records))
	return nil
}

func (h *VflHandler) ProcessPending(ctx context.Context, _ *asynq.Task) error {
	n, err := h.Svc.RunPending(ctx, 5)
	if err != nil {
		log.Printf("⚠️  [VflOSINT] pengecekan terjadwal berhenti: %v", err)
	}
	log.Printf("🕵️  [VflOSINT] %d nomor judol dicek (terjadwal)", n)
	return nil
}
