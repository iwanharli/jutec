package service

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"jutec-go/internal/discovery"
	"jutec-go/internal/store"
)

type SweepResult struct {
	Keywords      []string    `json:"keywords"`
	TotalFound    int         `json:"total_found"`
	NewDomains    int         `json:"new_domains"`
	Analyzed      int         `json:"analyzed,omitempty"`
	GamblingFound int         `json:"gambling_found,omitempty"`
	Domains       []string    `json:"domains"`
	BulkResult    *BulkResult `json:"bulk_result,omitempty"`
}

type DiscoveryService struct {
	Scraper     *discovery.GoogleScraper
	CertScraper *discovery.CertScraper
	ScanSvc     *ScanService
	PG          *store.PostgresDB
}

func NewDiscoveryService(serperKey string, serperPages int, scanSvc *ScanService, pg *store.PostgresDB) *DiscoveryService {
	return &DiscoveryService{
		Scraper:     discovery.NewGoogleScraper(serperKey, serperPages),
		CertScraper: discovery.NewCertScraper(),
		ScanSvc:     scanSvc,
		PG:          pg,
	}
}

func (s *DiscoveryService) SearchByKeyword(keyword string) ([]string, error) {
	return s.Scraper.Search(keyword)
}

// LoadDefaultKeywords mengambil search keywords aktif dari DB.
// Dipakai handler saat use_defaults: true.
func (s *DiscoveryService) LoadDefaultKeywords(ctx context.Context) []string {
	if s.PG == nil {
		return nil
	}
	rows, err := s.PG.ListSearchKeywords(ctx, true)
	if err != nil {
		return nil
	}
	kws := make([]string, len(rows))
	for i, r := range rows {
		kws[i] = r.Keyword
	}
	return kws
}

// Sweep runs parallel Google searches for all keywords, deduplicates results,
// filters out already-scanned domains, and optionally auto-analyzes new ones.
func (s *DiscoveryService) Sweep(ctx context.Context, keywords []string, autoAnalyze bool) (*SweepResult, error) {
	allDomains := s.collectDomains(ctx, keywords)

	var allURLs []string
	for domain := range allDomains {
		allURLs = append(allURLs, domain)
	}

	newURLs := s.filterNewDomains(ctx, allURLs)

	res := &SweepResult{
		Keywords:   keywords,
		TotalFound: len(allURLs),
		NewDomains: len(newURLs),
		Domains:    newURLs,
	}

	if autoAnalyze && len(newURLs) > 0 {
		bulk := s.ScanSvc.AnalyzeBulk(ctx, newURLs, 0)
		res.Analyzed = bulk.Processed
		res.GamblingFound = bulk.GamblingFound
		res.BulkResult = bulk
	}

	return res, nil
}

// --- Deep Sweep: Recursive Crawl ---

// DeepSweepConfig mengontrol perilaku recursive deep crawl.
type DeepSweepConfig struct {
	MaxDepth int `json:"max_depth"` // max kedalaman rekursi (default: 3)
	MaxURLs  int `json:"max_urls"`  // max total URL yang diproses (default: 500)
	// SeedURLs: titik awal langsung (mode "domain awal"). Jika diisi, Depth 0 menganalisis URL ini
	// — termasuk yang sudah pernah di-scan — alih-alih mencari keyword di Serper.
	SeedURLs []string `json:"seed_urls,omitempty"`
}

// DepthStats menampilkan statistik per kedalaman.
type DepthStats struct {
	Depth         int      `json:"depth"`
	URLsFound     int      `json:"urls_found"`
	NewURLs       int      `json:"new_urls"`
	Analyzed      int      `json:"analyzed"`
	GamblingFound int      `json:"gambling_found"`
	Domains       []string `json:"domains"`
}

// DeepSweepResult adalah hasil lengkap deep crawl.
type DeepSweepResult struct {
	Keywords       []string     `json:"keywords"`
	DepthReached   int          `json:"depth_reached"`
	TotalFound     int          `json:"total_found"`
	TotalAnalyzed  int          `json:"total_analyzed"`
	TotalGambling  int          `json:"total_gambling"`
	TotalSkipped   int          `json:"total_skipped"`
	PerDepth       []DepthStats `json:"per_depth"`
	TimeTaken      string       `json:"time_taken"`
}

// DeepSweep menjalankan recursive crawl:
// Depth 0: Serper results → bulk analyze
// Depth 1+: Extract external links & iframes dari situs judol → analyze → repeat
// Berhenti saat max_depth tercapai, max_urls tercapai, atau tidak ada URL baru.
func (s *DiscoveryService) DeepSweep(ctx context.Context, keywords []string, cfg DeepSweepConfig) (*DeepSweepResult, error) {
	start := time.Now()

	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 10
	}
	if cfg.MaxURLs <= 0 {
		cfg.MaxURLs = 5000
	}

	// Load domain blacklist dari DB (sekali di awal)
	var blacklist []string
	if s.PG != nil {
		blacklist = s.PG.GetActiveBlacklistPatterns(ctx)
		log.Printf("🛡️  [DeepSweep] Loaded %d domain blacklists", len(blacklist))
	}

	// Global tracking
	globalSeen := make(map[string]bool) // semua URL yang sudah diproses/dilihat
	totalAnalyzed := 0
	totalGambling := 0
	totalSkipped := 0
	var perDepth []DepthStats

	// ── Depth 0: Serper discovery ──
	var seedURLs, newSeedURLs []string
	if len(cfg.SeedURLs) > 0 {
		// Mode domain awal: crawl domain itu sendiri (dipaksa, walau sudah ada di DB)
		log.Printf("🚀 [DeepSweep] Starting Depth 0 | Seed: %v", cfg.SeedURLs)
		for _, u := range cfg.SeedURLs {
			if !globalSeen[u] {
				globalSeen[u] = true
				seedURLs = append(seedURLs, u)
			}
		}
		newSeedURLs = seedURLs
	} else {
		log.Printf("🚀 [DeepSweep] Starting Depth 0 | Keywords: %d", len(keywords))
		allDomains := s.collectDomains(ctx, keywords)
		for domain := range allDomains {
			if !globalSeen[domain] {
				globalSeen[domain] = true
				seedURLs = append(seedURLs, domain)
			}
		}
		newSeedURLs = s.filterNewDomains(ctx, seedURLs)
		totalSkipped += len(seedURLs) - len(newSeedURLs)
	}

	// Cap jumlah URL depth 0
	if len(newSeedURLs) > cfg.MaxURLs {
		newSeedURLs = newSeedURLs[:cfg.MaxURLs]
	}

	depth0Stats := DepthStats{
		Depth:   0,
		URLsFound: len(seedURLs),
		NewURLs: len(newSeedURLs),
		Domains: newSeedURLs,
	}

	if len(newSeedURLs) > 0 {
		log.Printf("📊 [DeepSweep] Depth 0: Analyzing %d new URLs", len(newSeedURLs))
		bulk := s.ScanSvc.AnalyzeBulk(ctx, newSeedURLs, 0)
		depth0Stats.Analyzed = bulk.Processed
		depth0Stats.GamblingFound = bulk.GamblingFound
		totalAnalyzed += bulk.Processed
		totalGambling += bulk.GamblingFound

		// ── Multi-source enrichment setelah Depth 0 ──
		// Jalankan semua sumber discovery secara paralel untuk efisiensi

		var (
			ctURLs          []string
			trackingURLs    []string
			nsURLs          []string
			alternateURLs   []string
			storedLinkURLs  []string
		)

		var enrichWG sync.WaitGroup
		enrichWG.Add(5)

		go func() {
			defer enrichWG.Done()
			ctURLs = s.discoverViaCT(bulk, globalSeen, blacklist)
			if len(ctURLs) > 0 {
				log.Printf("🔐 [DeepSweep] CT enrichment: +%d domain baru dari crt.sh", len(ctURLs))
			}
		}()
		go func() {
			defer enrichWG.Done()
			trackingURLs = s.discoverViaTracking(ctx, bulk, globalSeen)
		}()
		go func() {
			defer enrichWG.Done()
			nsURLs = s.discoverViaNS(ctx, bulk, globalSeen)
		}()
		go func() {
			defer enrichWG.Done()
			alternateURLs = s.discoverViaAlternate(bulk, globalSeen, blacklist)
		}()
		go func() {
			defer enrichWG.Done()
			storedLinkURLs = s.discoverViaStoredLinks(ctx, globalSeen, blacklist, 2000)
		}()

		enrichWG.Wait()

		// Kumpulkan external links dari situs judol untuk depth berikutnya
		nextURLs := s.extractFollowURLs(bulk, globalSeen, blacklist)
		nextURLs = append(nextURLs, ctURLs...)
		nextURLs = append(nextURLs, trackingURLs...)
		nextURLs = append(nextURLs, nsURLs...)
		nextURLs = append(nextURLs, alternateURLs...)
		nextURLs = append(nextURLs, storedLinkURLs...)

		log.Printf("📦 [DeepSweep] Depth 1 pool: %d URL (links:%d ct:%d tracking:%d ns:%d alt:%d stored:%d)",
			len(nextURLs),
			len(nextURLs)-len(ctURLs)-len(trackingURLs)-len(nsURLs)-len(alternateURLs)-len(storedLinkURLs),
			len(ctURLs), len(trackingURLs), len(nsURLs), len(alternateURLs), len(storedLinkURLs))

		perDepth = append(perDepth, depth0Stats)

		// ── Depth 1 .. N: Recursive follow ──
		currentURLs := nextURLs
		for depth := 1; depth <= cfg.MaxDepth; depth++ {
			if len(currentURLs) == 0 || totalAnalyzed >= cfg.MaxURLs {
				log.Printf("🛑 [DeepSweep] Stopping at depth %d: no new URLs or max reached", depth)
				break
			}

			// Cap per-depth
			remaining := cfg.MaxURLs - totalAnalyzed
			if remaining <= 0 {
				break
			}
			if len(currentURLs) > remaining {
				currentURLs = currentURLs[:remaining]
			}

			log.Printf("📊 [DeepSweep] Depth %d: Analyzing %d URLs", depth, len(currentURLs))

			depthStats := DepthStats{
				Depth:     depth,
				URLsFound: len(currentURLs),
				NewURLs:   len(currentURLs),
				Domains:   currentURLs,
			}

			bulk := s.ScanSvc.AnalyzeBulk(ctx, currentURLs, depth)
			depthStats.Analyzed = bulk.Processed
			depthStats.GamblingFound = bulk.GamblingFound
			totalAnalyzed += bulk.Processed
			totalGambling += bulk.GamblingFound

			perDepth = append(perDepth, depthStats)

			// Extract URLs untuk depth berikutnya
			currentURLs = s.extractFollowURLs(bulk, globalSeen, blacklist)
		}
	} else {
		perDepth = append(perDepth, depth0Stats)
	}

	depthReached := 0
	if len(perDepth) > 0 {
		depthReached = perDepth[len(perDepth)-1].Depth
	}

	return &DeepSweepResult{
		Keywords:      keywords,
		DepthReached:  depthReached,
		TotalFound:    len(globalSeen),
		TotalAnalyzed: totalAnalyzed,
		TotalGambling: totalGambling,
		TotalSkipped:  totalSkipped,
		PerDepth:      perDepth,
		TimeTaken:     time.Since(start).String(),
	}, nil
}

// DeepSweepJob seperti DeepSweep tapi mendukung pause/resume via JobState.
// Dipanggil dari goroutine background — update progress ke DB setiap depth selesai.
func (s *DiscoveryService) DeepSweepJob(ctx context.Context, jobID string, keywords []string, cfg DeepSweepConfig) (*DeepSweepResult, error) {
	job, hasJob := GlobalJobManager.Get(jobID)

	// Wrapper pause check — aman meski job tidak terdaftar
	checkPause := func() {
		if hasJob {
			job.CheckPause()
		}
	}
	logf := func(kind, format string, args ...any) {
		if hasJob {
			job.Logf(kind, format, args...)
		}
	}
	phase := func(p string, depth int) {
		if hasJob {
			job.SetPhase(p, depth)
		}
	}
	// Satu baris per situs yang selesai dianalisis
	logSite := func(res *ScanResult) {
		switch {
		case res == nil:
			return
		case res.Error != "":
			logf("skip", "✗ %s — %s", res.URL, shortErr(res.Error))
		case res.Score != nil && res.Score.IsGambling:
			logf("found", "✓ %s — judol %s %d%%", res.URL, res.Score.Priority, int(res.Score.Final*100))
		default:
			logf("info", "· %s — bukan judol", res.URL)
		}
	}

	// Progress counter (atomic-safe via mutex di JobState)
	processed := 0
	found := 0

	updateProgress := func(total int) {
		if s.PG != nil {
			s.PG.UpdateSweepJobProgress(ctx, jobID, processed, found, total)
		}
	}

	start := time.Now()
	if cfg.MaxDepth <= 0 { cfg.MaxDepth = 10 }
	if cfg.MaxURLs <= 0  { cfg.MaxURLs = 5000 }

	var blacklist []string
	if s.PG != nil {
		blacklist = s.PG.GetActiveBlacklistPatterns(ctx)
	}

	globalSeen    := make(map[string]bool)
	totalAnalyzed := 0
	totalGambling := 0
	totalSkipped  := 0
	var perDepth   []DepthStats

	// Depth 0
	checkPause()
	phase("Mencari di Google", 0)
	logf("phase", "Mencari di Google: %d keyword · maks %d URL · level %d", len(keywords), cfg.MaxURLs, cfg.MaxDepth)
	kwDone := 0
	var kwMu sync.Mutex
	allDomains := s.collectDomainsWithProgress(ctx, keywords, func(kw string, n int, cached bool, err error) {
		kwMu.Lock()
		kwDone++
		i := kwDone
		kwMu.Unlock()
		switch {
		case err != nil:
			logf("warn", "[%d/%d] \"%s\" gagal: %s", i, len(keywords), kw, shortErr(err.Error()))
		case cached:
			logf("info", "[%d/%d] \"%s\" → %d domain (cache)", i, len(keywords), kw, n)
		default:
			logf("info", "[%d/%d] \"%s\" → %d domain", i, len(keywords), kw, n)
		}
	})
	var seedURLs []string
	for domain := range allDomains {
		if !globalSeen[domain] {
			globalSeen[domain] = true
			seedURLs = append(seedURLs, domain)
		}
	}
	newSeedURLs := s.filterNewDomains(ctx, seedURLs)
	totalSkipped += len(seedURLs) - len(newSeedURLs)
	if len(newSeedURLs) > cfg.MaxURLs {
		newSeedURLs = newSeedURLs[:cfg.MaxURLs]
	}

	depth0Stats := DepthStats{Depth: 0, URLsFound: len(seedURLs), NewURLs: len(newSeedURLs), Domains: newSeedURLs}
	logf("phase", "Hasil Google: %d domain unik · %d baru · %d sudah pernah di-scan (dilewati)", len(seedURLs), len(newSeedURLs), len(seedURLs)-len(newSeedURLs))

	if len(newSeedURLs) > 0 {
		total := len(newSeedURLs)
		phase("Menganalisis hasil Google", 0)
		logf("phase", "Menganalisis %d situs dari hasil Google…", total)
		tickFn := func(res *ScanResult) {
			processed++
			logSite(res)
			updateProgress(total)
			checkPause()
		}
		bulk := s.ScanSvc.AnalyzeBulkJob(ctx, newSeedURLs, 0, tickFn)
		depth0Stats.Analyzed = bulk.Processed
		depth0Stats.GamblingFound = bulk.GamblingFound
		totalAnalyzed += bulk.Processed
		totalGambling += bulk.GamblingFound
		found = totalGambling
		logf("phase", "Hasil Google selesai: %d dianalisis, %d judol", bulk.Processed, bulk.GamblingFound)
		phase("Mencari jaringan (CT log, tracking, nameserver, tautan)", 0)
		logf("phase", "Mencari situs terkait lewat CT log, tracking ID, nameserver, domain alternatif & tautan tersimpan…")

		var enrichWG sync.WaitGroup
		var ctURLs, trackingURLs, nsURLs, alternateURLs, storedLinkURLs []string
		enrichWG.Add(5)
		go func() { defer enrichWG.Done(); ctURLs = s.discoverViaCT(bulk, globalSeen, blacklist) }()
		go func() { defer enrichWG.Done(); trackingURLs = s.discoverViaTracking(ctx, bulk, globalSeen) }()
		go func() { defer enrichWG.Done(); nsURLs = s.discoverViaNS(ctx, bulk, globalSeen) }()
		go func() { defer enrichWG.Done(); alternateURLs = s.discoverViaAlternate(bulk, globalSeen, blacklist) }()
		go func() { defer enrichWG.Done(); storedLinkURLs = s.discoverViaStoredLinks(ctx, globalSeen, blacklist, 2000) }()
		enrichWG.Wait()

		nextURLs := s.extractFollowURLs(bulk, globalSeen, blacklist)
		nextURLs  = append(nextURLs, ctURLs...)
		nextURLs  = append(nextURLs, trackingURLs...)
		nextURLs  = append(nextURLs, nsURLs...)
		nextURLs  = append(nextURLs, alternateURLs...)
		nextURLs  = append(nextURLs, storedLinkURLs...)
		perDepth  = append(perDepth, depth0Stats)
		logf("phase", "Kandidat level 1: %d domain (tautan %d · CT %d · tracking %d · NS %d · alternatif %d · tersimpan %d)",
			len(nextURLs), len(nextURLs)-len(ctURLs)-len(trackingURLs)-len(nsURLs)-len(alternateURLs)-len(storedLinkURLs),
			len(ctURLs), len(trackingURLs), len(nsURLs), len(alternateURLs), len(storedLinkURLs))

		currentURLs := nextURLs
		for depth := 1; depth <= cfg.MaxDepth; depth++ {
			checkPause()
			if len(currentURLs) == 0 {
				logf("phase", "Berhenti di level %d: tidak ada domain baru untuk diikuti", depth)
				break
			}
			if totalAnalyzed >= cfg.MaxURLs {
				logf("phase", "Berhenti di level %d: batas %d URL tercapai", depth, cfg.MaxURLs)
				break
			}
			remaining := cfg.MaxURLs - totalAnalyzed
			if len(currentURLs) > remaining { currentURLs = currentURLs[:remaining] }

			depthStats := DepthStats{Depth: depth, URLsFound: len(currentURLs), NewURLs: len(currentURLs), Domains: currentURLs}
			depthTotal := len(currentURLs)
			phase(fmt.Sprintf("Level %d", depth), depth)
			logf("phase", "Level %d: menganalisis %d domain…", depth, depthTotal)
			depthTickFn := func(res *ScanResult) {
				processed++
				logSite(res)
				updateProgress(processed + depthTotal)
				checkPause()
			}
			bulk := s.ScanSvc.AnalyzeBulkJob(ctx, currentURLs, depth, depthTickFn)
			depthStats.Analyzed    = bulk.Processed
			depthStats.GamblingFound = bulk.GamblingFound
			totalAnalyzed += bulk.Processed
			totalGambling += bulk.GamblingFound
			found          = totalGambling
			perDepth       = append(perDepth, depthStats)
			currentURLs    = s.extractFollowURLs(bulk, globalSeen, blacklist)
			logf("phase", "Level %d selesai: %d dianalisis, %d judol · %d kandidat level berikutnya", depth, bulk.Processed, bulk.GamblingFound, len(currentURLs))
		}
	} else {
		perDepth = append(perDepth, depth0Stats)
	}

	depthReached := 0
	if len(perDepth) > 0 { depthReached = perDepth[len(perDepth)-1].Depth }
	phase("Selesai", depthReached)
	logf("phase", "Selesai dalam %s — %d situs dianalisis, %d judol, %d dilewati", time.Since(start).Round(time.Second), totalAnalyzed, totalGambling, totalSkipped)

	return &DeepSweepResult{
		Keywords: keywords, DepthReached: depthReached,
		TotalFound: len(globalSeen), TotalAnalyzed: totalAnalyzed,
		TotalGambling: totalGambling, TotalSkipped: totalSkipped,
		PerDepth: perDepth, TimeTaken: time.Since(start).String(),
	}, nil
}

// extractFollowURLs mengekstrak external links + iframes dari situs judol
// dan mengembalikan hanya URL yang belum pernah dilihat dan belum pernah di-scan.
func (s *DiscoveryService) extractFollowURLs(bulk *BulkResult, globalSeen map[string]bool, blacklist []string) []string {
	candidateMap := make(map[string]bool)

	for _, res := range bulk.Results {
		// Hanya follow links dari situs yang terdeteksi judol
		if res.Score == nil || !res.Score.IsGambling {
			continue
		}
		if res.Result == nil || res.Result.Links == nil {
			continue
		}

		// External links
		for _, link := range res.Result.Links.External {
			domain := extractBaseDomain(link)
			if domain != "" && !globalSeen[domain] && !isBlacklisted(domain, blacklist) {
				candidateMap[domain] = true
			}
		}

		// Link pendaftaran (termasuk tujuan akhir shortener tombol DAFTAR) — situs kasino sebenarnya
		for _, link := range res.Result.Links.Register {
			domain := extractBaseDomain(link)
			if domain != "" && !globalSeen[domain] && !isBlacklisted(domain, blacklist) {
				candidateMap[domain] = true
			}
		}

		// Iframes — sering dipakai untuk embed game judol
		for _, link := range res.Result.Links.Iframes {
			domain := extractBaseDomain(link)
			if domain != "" && !globalSeen[domain] && !isBlacklisted(domain, blacklist) {
				candidateMap[domain] = true
			}
		}
	}

	// Filter yang sudah di-scan sebelumnya di DB
	var candidates []string
	for domain := range candidateMap {
		globalSeen[domain] = true // tandai sudah dilihat
		candidates = append(candidates, domain)
	}

	log.Printf("📂 [DeepSweep] Extracted %d candidates to follow", len(candidates))
	return s.filterNewDomains(context.Background(), candidates)
}

// extractBaseDomain mengekstrak base domain (scheme + host) dari URL lengkap.
func extractBaseDomain(rawURL string) string {
	// Cari scheme
	idx := strings.Index(rawURL, "://")
	if idx < 0 {
		return ""
	}

	// Ambil scheme + host saja (tanpa path)
	rest := rawURL[idx+3:]
	slashIdx := strings.Index(rest, "/")
	host := rest
	if slashIdx >= 0 {
		host = rest[:slashIdx]
	}

	if host == "" {
		return ""
	}

	scheme := rawURL[:idx]
	return scheme + "://" + host
}

// isBlacklisted mengecek apakah host URL sama dengan, atau subdomain dari, domain di blacklist.
// Dicocokkan per akhiran label agar "dw.com" tidak ikut memblokir "kingdw.com".
func isBlacklisted(domainURL string, blacklist []string) bool {
	host := strings.ToLower(strings.TrimSpace(domainURL))
	if strings.Contains(host, "://") {
		if u, err := url.Parse(host); err == nil {
			host = u.Hostname()
		}
	} else if i := strings.IndexAny(host, "/:"); i >= 0 {
		host = host[:i]
	}
	host = strings.TrimSuffix(host, ".")
	for _, pattern := range blacklist {
		p := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(pattern)), "*.")
		if p != "" && (host == p || strings.HasSuffix(host, "."+p)) {
			return true
		}
	}
	return false
}
// discoverViaCT mengambil brand keyword dari setiap situs judol yang dikonfirmasi,
// lalu query crt.sh untuk menemukan domain lain milik operator yang sama.
// Dibatasi max 10 brand keyword dan 3 concurrent requests agar tidak membebani crt.sh.
func (s *DiscoveryService) discoverViaCT(bulk *BulkResult, globalSeen map[string]bool, blacklist []string) []string {
	brands := extractBrandKeywords(bulk)
	if len(brands) == 0 {
		return nil
	}
	// Cap agar tidak spam crt.sh
	if len(brands) > 10 {
		brands = brands[:10]
	}

	var mu sync.Mutex
	var ctURLs []string
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for _, brand := range brands {
		wg.Add(1)
		go func(b string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			domains, err := s.CertScraper.Search(b)
			if err != nil {
				log.Printf("  ⚠️  [CT] crt.sh error for '%s': %v", b, err)
				return
			}

			mu.Lock()
			for _, d := range domains {
				base := extractBaseDomain(d)
				if base != "" && !globalSeen[base] && !isBlacklisted(base, blacklist) {
					globalSeen[base] = true
					ctURLs = append(ctURLs, base)
				}
			}
			mu.Unlock()
		}(brand)
	}
	wg.Wait()

	return s.filterNewDomains(context.Background(), ctURLs)
}

// extractBrandKeywords mengambil nama brand unik dari setiap situs judol yang confirmed.
// Contoh: "https://badak500.website/path" → "badak500"
func extractBrandKeywords(bulk *BulkResult) []string {
	seen := make(map[string]bool)
	var brands []string
	for _, res := range bulk.Results {
		if res.Score == nil || !res.Score.IsGambling {
			continue
		}
		brand := extractBrandFromURL(res.URL)
		if brand != "" && !seen[brand] {
			seen[brand] = true
			brands = append(brands, brand)
		}
	}
	return brands
}

// extractBrandFromURL mengambil nama domain utama tanpa TLD.
// "https://sub.badak500.website" → "badak500"
func extractBrandFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return ""
	}
	brand := parts[len(parts)-2]
	// Skip brand yang terlalu generik atau pendek
	if len(brand) < 4 {
		return ""
	}
	return brand
}

// discoverViaTracking mencari situs lain di DB yang berbagi tracking ID
// (AsiaSigma UUID, GigaGaming brand, GTM, GA, FB Pixel) dengan situs judol
// yang dikonfirmasi di Depth 0. Tidak memerlukan API eksternal.
func (s *DiscoveryService) discoverViaTracking(ctx context.Context, bulk *BulkResult, globalSeen map[string]bool) []string {
	if s.PG == nil {
		return nil
	}
	sourceURLs := gamblingURLs(bulk)
	if len(sourceURLs) == 0 {
		return nil
	}

	// Context independen — agar tidak ikut cancel saat sweep parent selesai
	dbCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	related, err := s.PG.GetSiteURLsBySharedTracking(dbCtx, sourceURLs)
	if err != nil {
		if ctx.Err() == nil { // hanya log jika bukan karena sweep cancel
			log.Printf("  ⚠️  [TrackingFollow] DB error: %v", err)
		}
		return nil
	}

	var newURLs []string
	for _, u := range related {
		if !globalSeen[u] {
			globalSeen[u] = true
			newURLs = append(newURLs, u)
		}
	}
	if len(newURLs) > 0 {
		log.Printf("🏷️  [TrackingFollow] %d situs baru via shared tracking ID", len(newURLs))
	}
	return s.filterNewDomains(ctx, newURLs)
}

// discoverViaNS mencari situs lain di DB yang berbagi nameserver dengan situs
// judol yang dikonfirmasi. NS yang sama = satu hosting panel/operator.
func (s *DiscoveryService) discoverViaNS(ctx context.Context, bulk *BulkResult, globalSeen map[string]bool) []string {
	if s.PG == nil {
		return nil
	}
	sourceURLs := gamblingURLs(bulk)
	if len(sourceURLs) == 0 {
		return nil
	}

	// Context independen — agar tidak ikut cancel saat sweep parent selesai
	dbCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	related, err := s.PG.GetSiteURLsBySharedNS(dbCtx, sourceURLs)
	if err != nil {
		if ctx.Err() == nil { // hanya log jika bukan karena sweep cancel
			log.Printf("  ⚠️  [NSFollow] DB error: %v", err)
		}
		return nil
	}

	var newURLs []string
	for _, u := range related {
		if !globalSeen[u] {
			globalSeen[u] = true
			newURLs = append(newURLs, u)
		}
	}
	if len(newURLs) > 0 {
		log.Printf("🌐 [NSFollow] %d situs baru via shared nameserver", len(newURLs))
	}
	return s.filterNewDomains(ctx, newURLs)
}

// discoverViaAlternate mengambil <link rel="alternate/canonical"> dari setiap
// situs judol yang dikonfirmasi — ini adalah domain terkait yang *secara eksplisit*
// dideklarasikan oleh operator sendiri di dalam HTML-nya.
func (s *DiscoveryService) discoverViaAlternate(bulk *BulkResult, globalSeen map[string]bool, blacklist []string) []string {
	var newURLs []string
	for _, res := range bulk.Results {
		if res.Score == nil || !res.Score.IsGambling {
			continue
		}
		if res.Result == nil || res.Result.Links == nil {
			continue
		}
		for _, link := range res.Result.Links.Alternate {
			base := extractBaseDomain(link)
			if base == "" || globalSeen[base] || isBlacklisted(base, blacklist) {
				continue
			}
			globalSeen[base] = true
			newURLs = append(newURLs, base)
		}
	}
	if len(newURLs) > 0 {
		log.Printf("🔗 [AlternateFollow] %d domain terkait dari <link rel=alternate>", len(newURLs))
	}
	return newURLs
}

// discoverViaStoredLinks mengambil external/iframe/register links dari tabel links DB
// yang berasal dari situs judol dan belum pernah di-scan. Ini melengkapi extractFollowURLs
// dengan menggunakan hasil sweep sebelumnya sebagai tambahan seed.
func (s *DiscoveryService) discoverViaStoredLinks(ctx context.Context, globalSeen map[string]bool, blacklist []string, limit int) []string {
	if s.PG == nil {
		return nil
	}
	rows, err := s.PG.Pool.Query(ctx, `
		SELECT DISTINCT l.url
		FROM links l
		JOIN site_links sl ON sl.link_id = l.id
		JOIN sites s ON s.id = sl.site_id
		WHERE s.is_gambling = TRUE
		  AND sl.link_type IN ('external', 'iframe', 'register')
		  AND l.domain IS NOT NULL AND l.domain != ''
		  AND NOT EXISTS (SELECT 1 FROM sites ss WHERE ss.url = l.url)
		  AND NOT EXISTS (
		    SELECT 1 FROM sites ss
		    WHERE regexp_replace(regexp_replace(lower(ss.url), '^https?://(www\.)?', ''), '/.*$', '') = l.domain
		  )
		ORDER BY l.url
		LIMIT $1
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var newURLs []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			continue
		}
		base := extractBaseDomain(u)
		if base == "" || globalSeen[base] || isBlacklisted(base, blacklist) {
			continue
		}
		globalSeen[base] = true
		newURLs = append(newURLs, base)
	}
	if len(newURLs) > 0 {
		log.Printf("🔗 [StoredLinks] %d domain baru dari tabel links DB", len(newURLs))
	}
	return newURLs
}

// gamblingURLs mengekstrak URL dari semua situs judol yang dikonfirmasi dalam BulkResult.
func gamblingURLs(bulk *BulkResult) []string {
	var urls []string
	for _, res := range bulk.Results {
		if res.Score != nil && res.Score.IsGambling && res.URL != "" {
			urls = append(urls, res.URL)
		}
	}
	return urls
}

// collectDomains mencari domain dari Serper untuk setiap keyword.
// Cache 24 jam: kalau keyword sudah pernah di-search hari ini, hasil diambil dari DB.
// Ini memungkinkan resume sweep tanpa hit Serper API lagi setelah restart.
func (s *DiscoveryService) collectDomains(ctx context.Context, keywords []string) map[string]bool {
	return s.collectDomainsWithProgress(ctx, keywords, nil)
}

// collectDomainsWithProgress sama dengan collectDomains, dengan callback per keyword (untuk log job).
func (s *DiscoveryService) collectDomainsWithProgress(ctx context.Context, keywords []string, onKeyword func(keyword string, n int, cached bool, err error)) map[string]bool {
	var mu sync.Mutex
	allDomains := make(map[string]bool)

	semaphore := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for _, kw := range keywords {
		wg.Add(1)
		go func(keyword string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 1. Cek cache DB dulu
			if s.PG != nil {
				if cached := s.PG.GetDiscoveryCache(ctx, "serper", keyword); len(cached) > 0 {
					log.Printf("💾 [Cache] Serper hit: '%s' → %d URL (dari cache)", keyword, len(cached))
					if onKeyword != nil {
						onKeyword(keyword, len(cached), true, nil)
					}
					mu.Lock()
					for _, u := range cached {
						allDomains[u] = true
					}
					mu.Unlock()
					return
				}
			}

			// 2. Cache miss — hit Serper API
			// Baca serper_pages dari DB settings (runtime), fallback ke nilai default scraper
			pages := s.Scraper.Pages
			if s.PG != nil {
				if p := s.PG.GetSettingInt(ctx, "serper_pages", 0); p > 0 {
					pages = p
				}
			}
			urls, err := s.Scraper.SearchPages(keyword, pages)
			if err != nil {
				log.Printf("❌ [Serper] Error untuk keyword '%s': %v", keyword, err)
				if onKeyword != nil {
					onKeyword(keyword, 0, false, err)
				}
				return
			}

			// 3. Simpan ke cache (TTL 24 jam)
			if s.PG != nil && len(urls) > 0 {
				if err := s.PG.SaveDiscoveryCache(ctx, "serper", keyword, urls, 24*time.Hour); err != nil {
					log.Printf("⚠️  [Cache] Gagal simpan cache untuk '%s': %v", keyword, err)
				}
			}

			if onKeyword != nil {
				onKeyword(keyword, len(urls), false, nil)
			}
			mu.Lock()
			for _, u := range urls {
				allDomains[u] = true
			}
			mu.Unlock()
		}(kw)
	}

	wg.Wait()
	return allDomains
}

// DiscoverCerts mencari domain baru dari Certificate Transparency logs via crt.sh.
// Cache 6 jam: CT logs berubah lebih cepat dari Serper.
func (s *DiscoveryService) DiscoverCerts(keyword string) ([]string, error) {
	ctx := context.Background()

	// Cek cache dulu
	if s.PG != nil {
		if cached := s.PG.GetDiscoveryCache(ctx, "crtsh", keyword); len(cached) > 0 {
			log.Printf("💾 [Cache] CT hit: '%s' → %d domain (dari cache)", keyword, len(cached))
			return cached, nil
		}
	}

	// Cache miss — hit crt.sh
	results, err := s.CertScraper.Search(keyword)
	if err != nil {
		return nil, err
	}

	// Simpan ke cache (TTL 6 jam — CT logs berubah lebih cepat)
	if s.PG != nil && len(results) > 0 {
		s.PG.SaveDiscoveryCache(ctx, "crtsh", keyword, results, 6*time.Hour)
	}

	return results, nil
}

func (s *DiscoveryService) filterNewDomains(ctx context.Context, urls []string) []string {
	if s.PG == nil {
		return urls
	}

	var newURLs []string
	for _, u := range urls {
		if !s.PG.IsAlreadyScanned(ctx, u) {
			newURLs = append(newURLs, u)
		}
	}
	return newURLs
}

// shortErr memotong pesan galat agar satu baris log tetap ringkas.
func shortErr(e string) string {
	if len(e) > 90 {
		return e[:90] + "…"
	}
	return e
}
