// backfillowner mengisi identifier pengelola (reCAPTCHA, Firebase, bot Telegram, Cloudflare Analytics, …)
// untuk situs judol yang masih aktif tanpa menunggu scan ulang. Halaman & file JS situs diunduh biasa
// (tanpa Chrome), hasil disimpan seperti scan, lalu klaster operator dibangun ulang.
//
//	go run ./cmd/backfillowner            # semua situs judol aktif
//	go run ./cmd/backfillowner -limit 50  # sebagian
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"jutec-go/internal/config"
	"jutec-go/internal/crawler"
	"jutec-go/internal/netutil"
	"jutec-go/internal/store"

	"github.com/joho/godotenv"
)

// Platform yang sudah ditangkap scan biasa — dilewati agar fokus ke identifier baru
var existing = map[string]bool{"GoogleTag": true, "GoogleTagManager": true, "GoogleAnalytics": true, "FacebookPixel": true,
	"LiveChat": true, "TawkTo": true, "Crisp": true, "Hotjar": true, "AsiaSigma": true, "GigaGaming": true}

func main() {
	limit := flag.Int("limit", 0, "batasi jumlah situs (0 = semua)")
	workers := flag.Int("workers", 6, "jumlah unduhan paralel")
	flag.Parse()
	godotenv.Load()
	pg, err := store.NewPostgresConnection(config.LoadConfig().PostgresURL)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	q := `SELECT id, url FROM sites WHERE is_gambling AND COALESCE(status,'') <> 'unreachable' AND offline_since IS NULL
	      ORDER BY priority = 'High' DESC, last_scanned_at DESC NULLS LAST`
	if *limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", *limit)
	}
	rows, err := pg.Pool.Query(ctx, q)
	if err != nil {
		log.Fatal(err)
	}
	type site struct {
		id  int
		url string
	}
	var sites []site
	for rows.Next() {
		var s site
		rows.Scan(&s.id, &s.url)
		sites = append(sites, s)
	}
	rows.Close()
	log.Printf("Mengisi identifier pengelola untuk %d situs judol aktif…", len(sites))

	client := netutil.HTTPClient(15 * time.Second) // DoH: tembus blokir DNS ISP
	var done, reached, withIDs, saved int64
	var mu sync.Mutex
	perPlatform := map[string]int{}
	jobs := make(chan site)
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range jobs {
				ids := fetchOwnerIDs(client, s.url)
				if ids != nil {
					atomic.AddInt64(&reached, 1)
				}
				var maps []map[string]string
				for _, t := range ids {
					if existing[t.Platform] {
						continue
					}
					maps = append(maps, map[string]string{"platform": t.Platform, "value": t.Value})
					mu.Lock()
					perPlatform[t.Platform]++
					mu.Unlock()
				}
				if len(maps) > 0 {
					pg.SaveTrackingIDs(ctx, s.id, maps)
					atomic.AddInt64(&withIDs, 1)
					atomic.AddInt64(&saved, int64(len(maps)))
				}
				if n := atomic.AddInt64(&done, 1); n%50 == 0 {
					log.Printf("  %d/%d situs · %d terjangkau · %d punya identifier baru", n, len(sites), atomic.LoadInt64(&reached), atomic.LoadInt64(&withIDs))
				}
			}
		}()
	}
	for _, s := range sites {
		jobs <- s
	}
	close(jobs)
	wg.Wait()

	log.Printf("Selesai: %d situs diperiksa, %d terjangkau, %d punya identifier baru (%d identifier)", done, reached, withIDs, saved)
	keys := make([]string, 0, len(perPlatform))
	for k := range perPlatform {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return perPlatform[keys[i]] > perPlatform[keys[j]] })
	for _, k := range keys {
		log.Printf("  %-20s %d", k, perPlatform[k])
	}

	// Identifier yang dipakai lebih dari satu situs = akun pengelola yang sama
	shared, _ := pg.Pool.Query(ctx, `
		SELECT t.platform, t.value, COUNT(DISTINCT st.site_id) n, string_agg(DISTINCT s.url, ' · ')
		FROM tracking_ids t JOIN site_tracking st ON st.tracking_id = t.id JOIN sites s ON s.id = st.site_id AND s.is_gambling
		WHERE t.platform IN ('ReCaptcha','GoogleAPIKey','FirebaseProject','AdSense','TelegramBot','TelegramChat','OneSignal','Sentry',
		                     'CloudflareAnalytics','Clarity','YandexMetrica','TikTokPixel','Jivo','Tidio','Zendesk','Smartsupp')
		GROUP BY 1, 2 HAVING COUNT(DISTINCT st.site_id) > 1 ORDER BY n DESC LIMIT 25`)
	log.Println("Identifier yang menghubungkan beberapa situs:")
	for shared.Next() {
		var p, v, urls string
		var n int
		shared.Scan(&p, &v, &n, &urls)
		if len(urls) > 160 {
			urls = urls[:160] + "…"
		}
		log.Printf("  %-18s %-42s %2d situs  %s", p, v, n, urls)
	}
	shared.Close()

	if n, err := pg.DiscoverOperators(ctx); err == nil {
		log.Printf("Klaster operator dibangun ulang: %d operator", n)
	}
	pg.Audit(ctx, "system (backfillowner)", "", "owner_ids.backfilled", "", 200,
		map[string]any{"sites": done, "reached": reached, "with_ids": withIDs, "ids": saved})
}

// fetchOwnerIDs: nil bila situs tak terjangkau, slice (mungkin kosong) bila terjangkau.
func fetchOwnerIDs(client *http.Client, url string) []crawler.TrackingID {
	req, err := http.NewRequest("GET", strings.TrimRight(url, `\`), nil)
	if err != nil {
		return nil // URL rusak di database (mis. diakhiri "\") — lewati
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13; SM-A536E) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil
	}
	html := string(body)
	out := crawler.ExtractOwnerIDs(html)
	for _, t := range crawler.ScanOwnerScripts(html, resp.Request.URL.String()) {
		out = append(out, t)
	}
	if out == nil {
		out = []crawler.TrackingID{}
	}
	return out
}
