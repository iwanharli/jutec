package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"jutec-go/internal/urlnorm"
	"log"
	neturl "net/url"
	"strings"
	"sync"
	"time"

	"jutec-go/internal/crawler"
	"jutec-go/internal/infra"
	"jutec-go/internal/intelligence"
	"jutec-go/internal/netutil"
	"jutec-go/internal/store"
	"jutec-go/internal/whois"

	"github.com/hibiken/asynq"
)

type ScanResult struct {
	URL         string                  `json:"url"`
	Result      *crawler.AnalysisResult `json:"result,omitempty"`
	DomainIntel *whois.DomainIntel      `json:"domain_intel,omitempty"`
	Score       *intelligence.Score     `json:"score,omitempty"`
	Saved       bool                    `json:"saved"`
	Error       string                  `json:"error,omitempty"`
}

type BulkResult struct {
	Total         int           `json:"total"`
	Processed     int           `json:"processed"`
	GamblingFound int           `json:"gambling_found"`
	Results       []*ScanResult `json:"results"`
	TimeTaken     string        `json:"time_taken"`
}

type ScanService struct {
	Crawler    *crawler.Crawler
	Neo4j      *store.Neo4jDB
	PG         *store.PostgresDB
	Scanner    *infra.Scanner
	TaskClient *asynq.Client
}

func NewScanService(neo4j *store.Neo4jDB, pg *store.PostgresDB, taskClient *asynq.Client) *ScanService {
	return &ScanService{
		Crawler:    crawler.NewCrawler(),
		Neo4j:      neo4j,
		PG:         pg,
		Scanner:    infra.NewScanner(),
		TaskClient: taskClient,
	}
}

// ProgressFunc dipanggil di setiap tahap scan untuk streaming progress ke client.
// step: nama tahap, message: deskripsi singkat, data: payload opsional.
type ProgressFunc func(step, message string, data interface{})

// nopProgress adalah ProgressFunc kosong agar AnalyzeURL tidak perlu mengecek nil.
func nopProgress(_, _ string, _ interface{}) {}

func (s *ScanService) AnalyzeURL(ctx context.Context, url string, depth int) (*ScanResult, error) {
	return s.analyzeURLInternal(ctx, url, depth, nopProgress)
}

// AnalyzeURLStreaming sama dengan AnalyzeURL tapi mengirim progress ke fn.
func (s *ScanService) AnalyzeURLStreaming(ctx context.Context, url string, depth int, fn ProgressFunc) (*ScanResult, error) {
	return s.analyzeURLInternal(ctx, url, depth, fn)
}

// ErrInvalidURL dikembalikan bila input bukan URL http(s) dengan hostname valid.
var ErrInvalidURL = errors.New("invalid url: harus http(s) dengan hostname valid")

// normalizeURL: bentuk kanonik bersama (lihat urlnorm) — juga ditegakkan di SaveScanResult & indeks unik DB.
func normalizeURL(raw string) (string, error) {
	u, err := urlnorm.Canonical(raw)
	if err != nil {
		return raw, ErrInvalidURL
	}
	return u, nil
}

func (s *ScanService) analyzeURLInternal(ctx context.Context, url string, depth int, progress ProgressFunc) (*ScanResult, error) {
	url, err := normalizeURL(url)
	if err != nil {
		return &ScanResult{URL: url, Error: err.Error()}, err
	}
	dbKeywords := s.loadDetectionKeywords(ctx)

	// 1. Crawl + content & network analysis
	progress("crawling", "Membuka headless browser dan menganalisis halaman...", nil)
	analysis, err := s.Crawler.ProcessURL(url, dbKeywords, depth)
	if err != nil {
		if s.PG != nil {
			payload := map[string]interface{}{"error": err.Error()}
			s.PG.SaveScanFailure(ctx, url, payload)
		}
		return &ScanResult{URL: url, Error: err.Error()}, err
	}
	progress("crawled", fmt.Sprintf("Selesai crawl — %d keyword, %d artefak, %d game provider",
		len(analysis.FoundKeywords), len(analysis.Artifacts),
		len(analysis.NetworkSignals.GameProviders)), analysis)

	// 2. WHOIS domain intelligence (non-fatal)
	progress("whois", "Mengambil informasi domain (WHOIS)...", nil)
	domainIntel, _ := whois.Lookup(url)
	if domainIntel != nil {
		progress("whois_done", fmt.Sprintf("Domain umur %d hari, risk score %.2f", domainIntel.DomainAgeDays, domainIntel.RiskScore), domainIntel)
	}

	// 3. SimHash cluster boost
	progress("simhash", "Membandingkan template dengan situs judol yang diketahui...", nil)
	simHashBoost := 0.0
	if analysis.SimHash != "" && s.PG != nil {
		similar, err := s.PG.GetSimilarTemplates(ctx, analysis.SimHash, 8)
		if err == nil && len(similar) > 0 {
			simHashBoost = 0.35
			progress("simhash_hit", fmt.Sprintf("Template mirip %d situs judol (+0.35 boost)", len(similar)), similar)
			log.Printf("  🔗 [SimHash] Template mirip %d situs judol (boost +0.35)", len(similar))
		}
	}

	// 4. Composite scoring — query enrichment boost dari scan sebelumnya (jika ada)
	progress("scoring", "Menghitung composite score...", nil)
	domainRisk := 0.0
	if domainIntel != nil {
		domainRisk = domainIntel.RiskScore
	}

	// Cek enrichment data dari scan sebelumnya (VT + URLScan)
	enrichmentBoost := 0.0
	if s.PG != nil {
		existingSiteID := s.PG.GetSiteID(ctx, url)
		if existingSiteID > 0 {
			eb := s.PG.GetEnrichmentBoost(ctx, existingSiteID)
			if eb != nil && eb.Boost > 0 {
				enrichmentBoost = eb.Boost
				progress("enrichment_boost", fmt.Sprintf("Enrichment data: VT %d/%d malicious, URLScan %s (+%.2f)",
					eb.VTMalicious, eb.VTTotal, eb.URLScanVerdict, eb.Boost), eb)
			}
		}
	}

	score := intelligence.Compute(analysis, domainRisk, url, simHashBoost, enrichmentBoost)
	progress("scored", fmt.Sprintf("Score: %.2f | Priority: %s | IsGambling: %v",
		score.Final, score.Priority, score.IsGambling), score)

	// 5. Postgres — simpan semua situs (gambling maupun tidak) sebagai audit log
	progress("saving", "Menyimpan hasil ke database...", nil)
	pgSaved, siteID := s.persistToPostgres(ctx, url, analysis, score, "active")

	// Peran dalam jaringan: situs utama (operator) atau landing page yang mengarah ke domain lain
	if siteID > 0 && s.PG != nil {
		role, targets := crawler.ClassifyRole(analysis, url)
		s.PG.SaveSiteRole(ctx, siteID, role, targets)
	}

	// Diblokir DNS ISP tetapi berhasil dibuka lewat IP asli → catat sebagai "diblokir, masih hidup"
	if siteID > 0 && analysis.ISPBlocked && s.PG != nil {
		s.PG.SaveReach(ctx, siteID, netutil.Reach{Status: "blocked_alive", HTTPStatus: 200, ISPBlocked: true})
	}

	// Simpan WHOIS data
	if siteID > 0 && domainIntel != nil && s.PG != nil {
		s.PG.SaveWHOIS(ctx, siteID, url, domainIntel)
	}

	// 6. Neo4j — hanya situs judol, untuk graph intelligence
	neo4jSaved := false
	if score.IsGambling {
		neo4jSaved = s.persistToNeo4j(ctx, url, analysis, score)
	}

	// 7. Infra scan — DNS, SSL, TXT — dijalankan untuk SEMUA situs
	if siteID > 0 {
		progress("infra", "Scan infrastruktur (DNS, NS, SSL, TXT)...", nil)
		s.runInfraScan(ctx, url, siteID, score.IsGambling)
		progress("infra_done", "Infra scan selesai", nil)
	}

	// 8. Sub-crawl external links — hanya untuk situs judol yang dikonfirmasi
	if siteID > 0 && score.IsGambling && analysis.Links != nil {
		// Tujuan pendaftaran eksternal didahulukan: biasanya situs kasino asli di balik halaman promosi
		partners := externalRegisterLinks(url, analysis.Links.Register)
		// Situs rekanan (tujuan DAFTAR/LOGIN) dianalisis penuh sebagai situs tersendiri di background,
		// agar punya skor, WHOIS, sidik jari & halaman detail — bukan hanya ditumpahkan ke situs induk.
		follow := analysis.Links.External
		if n := s.enqueuePartnerScans(ctx, partners); n > 0 {
			progress("partners_queued", fmt.Sprintf("%d situs rekanan (tujuan pendaftaran) diantrekan untuk analisis penuh", n), partners)
		} else {
			// Antrean tak aktif → cadangan: sub-crawl rekanan & gabungkan ke situs induk seperti sebelumnya
			follow = mergeLinks(partners, analysis.Links.External)
		}
		progress("subcrawl", fmt.Sprintf("Sub-crawl %d external link...", min(10, len(follow))), nil)
		s.subCrawlExternalLinks(ctx, siteID, follow, dbKeywords, depth)
	}

	// 9b. Analisis lanjutan situs judol di background: dekode QRIS, OCR banner, analisis APK
	if siteID > 0 && s.TaskClient != nil && score.IsGambling {
		payload, _ := json.Marshal(map[string]int{"site_id": siteID})
		if _, err := s.TaskClient.Enqueue(asynq.NewTask("site:deep", payload, asynq.MaxRetry(1)),
			asynq.Queue("low"), asynq.Unique(6*time.Hour), asynq.Timeout(15*time.Minute)); err == nil {
			progress("deep_queued", "Analisis lanjutan (QRIS, OCR, APK) diantrekan di background", nil)
		}
	}

	// 9. Background enrichment: URLScan + VirusTotal (non-blocking)
	if siteID > 0 && s.TaskClient != nil {
		s.enqueueEnrichment(siteID, url)
		progress("enrichment_queued", "URLScan + VirusTotal enrichment di-queue (background)", nil)
	}

	return &ScanResult{
		URL:         url,
		Result:      analysis,
		DomainIntel: domainIntel,
		Score:       score,
		Saved:       neo4jSaved || pgSaved,
	}, nil
}

// subCrawlExternalLinks melakukan crawl ringan ke setiap external link
// untuk mengekstrak artifacts (kontak, payment) dan links tambahan.
// Hasil disimpan ke DB dengan siteID parent sebagai referensi.
func (s *ScanService) subCrawlExternalLinks(ctx context.Context, parentSiteID int, externalLinks []string, dbKeywords []crawler.WeightedKeyword, depth int) {
	if s.PG == nil || len(externalLinks) == 0 {
		return
	}

	// Max 10 sub-crawl per site untuk menghindari overload
	limit := 10

	// Satu URL per host: halaman lain dari situs eksternal yang sama tak menambah informasi relasi
	seenHost := map[string]bool{}
	crawled := 0
	for i := 0; i < len(externalLinks) && crawled < limit; i++ {
		link := externalLinks[i]

		// Skip CDN, analytics, schema, dll
		if isNonContentURL(link) {
			continue
		}
		if u, err := neturl.Parse(link); err == nil {
			h := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
			// Daftar noise yang sama dengan penyimpanan tautan (api.w.org, beacon analytics, dll.)
			if seenHost[h] || store.IsNoiseDomain(h) {
				continue
			}
			seenHost[h] = true
		}
		crawled++

		log.Printf("  🔍 [Sub-Crawl D:%d] Link: %s", depth, link)

		_, cancel := context.WithTimeout(ctx, 20*time.Second)
		subResult, err := s.Crawler.ProcessURLDesktop(link, dbKeywords, depth)
		cancel()

		if err != nil {
			log.Printf("  ⚠️  [Sub-Crawl D:%d] Error: %v", depth, err)
			continue
		}

		// Simpan artifacts dari sub-link ke parent site
		if len(subResult.Artifacts) > 0 {
			var artMaps []map[string]string
			for _, a := range subResult.Artifacts {
				artMaps = append(artMaps, map[string]string{
					"type": a.Type, "value": a.Value, "name": a.Name,
				})
			}
			// Gunakan context baru untuk persistensi agar tidak mati jika parent ctx dibatalkan
			dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := s.PG.SaveArtifacts(dbCtx, parentSiteID, artMaps); err != nil {
				log.Printf("  ⚠️  [Postgres] Failed to save sub-crawl artifacts: %v", err)
			} else {
				log.Printf("  ✅ [Sub-Crawl D:%d] Found %d artifacts from %s", depth, len(subResult.Artifacts), link)
			}
			dbCancel()
		}

		// Simpan links dari sub-link juga
		if subResult.Links != nil {
			linkMap := map[string][]string{
				"external": subResult.Links.External,
				"iframe":   subResult.Links.Iframes,
				"register": subResult.Links.Register,
			}
			if err := s.PG.SaveLinks(ctx, parentSiteID, linkMap); err != nil {
				log.Printf("  ⚠️  [Postgres] Failed to save sub-crawl links: %v", err)
			}
		}

		// Simpan tracking IDs
		if len(subResult.TrackingIDs) > 0 {
			var tMaps []map[string]string
			for _, t := range subResult.TrackingIDs {
				tMaps = append(tMaps, map[string]string{
					"platform": t.Platform,
					"value":    t.Value,
				})
			}
			s.PG.SaveTrackingIDs(ctx, parentSiteID, tMaps)
		}
	}
}

// isNonContentURL mengecek apakah URL jelas bukan halaman konten (CDN, JS, analytics).
func isNonContentURL(rawURL string) bool {
	skipPatterns := []string{
		"cloudflare", "googleapis.com", "gstatic.com", "cdn.", "jquery",
		"bootstrap", "fontawesome", "schema.org", "w3.org",
		"google-analytics", "googletagmanager", "ampproject.org",
		"facebook.com", "twitter.com", "instagram.com", "youtube.com",
		"play.google.com", "plus.google.com", "namecheap.com",
		".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".woff", ".woff2", ".eot", ".ttf",
		".mp4", ".webm", ".avi", ".mp3", ".wav", ".pdf", ".zip", ".rar",
		// Endpoint teknis WordPress/CMS — bukan halaman konten
		"xmlrpc.php", "/wp-json", "oembed", "/wp-content/", "/wp-includes/", "/feed", "?rsd", "/comments/",
		"whatsapp.net", "whatsapp.com", "t.me/", "wa.me/",
	}
	lower := strings.ToLower(rawURL)
	for _, p := range skipPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func (s *ScanService) AnalyzeBulk(ctx context.Context, urls []string, depth int) *BulkResult {
	// depth 0 = initial seed, gunakan lebih banyak worker
	start := time.Now()

	resultsChan := make(chan *ScanResult, len(urls))
	semaphore := make(chan struct{}, bulkConcurrency)
	var wg sync.WaitGroup

	for _, u := range urls {
		wg.Add(1)
		go func(targetURL string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			scanCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			res, _ := s.AnalyzeURL(scanCtx, targetURL, depth)
			resultsChan <- res
		}(u)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var results []*ScanResult
	gamblingCount := 0
	for res := range resultsChan {
		results = append(results, res)
		if res.Score != nil && res.Score.IsGambling {
			gamblingCount++
		}
	}

	return &BulkResult{
		Total:         len(urls),
		Processed:     len(results),
		GamblingFound: gamblingCount,
		Results:       results,
		TimeTaken:     time.Since(start).String(),
	}
}

// AnalyzeBulkJob seperti AnalyzeBulk tapi memanggil tickFn setelah setiap URL selesai.
// tickFn bisa dipakai untuk CheckPause() atau update progress counter.
// bulkConcurrency: jumlah situs yang dianalisis bersamaan pada scan massal / Deep Sweep.
// Tiap analisis = satu Chrome headless (±130 MB profil + ratusan MB RAM); 10 terlalu berat untuk satu laptop.
const bulkConcurrency = 4

func (s *ScanService) AnalyzeBulkJob(ctx context.Context, urls []string, depth int, tickFn func(res *ScanResult)) *BulkResult {
	start := time.Now()

	resultsChan := make(chan *ScanResult, len(urls))
	semaphore := make(chan struct{}, bulkConcurrency)
	var wg sync.WaitGroup

	for _, u := range urls {
		wg.Add(1)
		go func(targetURL string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			scanCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			res, _ := s.AnalyzeURL(scanCtx, targetURL, depth)
			resultsChan <- res
			if tickFn != nil {
				tickFn(res)
			}
		}(u)
	}

	go func() { wg.Wait(); close(resultsChan) }()

	var results []*ScanResult
	gamblingCount := 0
	for res := range resultsChan {
		results = append(results, res)
		if res.Score != nil && res.Score.IsGambling {
			gamblingCount++
		}
	}

	return &BulkResult{
		Total: len(urls), Processed: len(results),
		GamblingFound: gamblingCount, Results: results,
		TimeTaken: time.Since(start).String(),
	}
}

// persistToPostgres menyimpan semua situs ke Postgres sebagai sumber kebenaran utama.
// Mengembalikan (sukses, siteID) agar siteID bisa dipakai langkah berikutnya.
func (s *ScanService) persistToPostgres(ctx context.Context, url string, analysis *crawler.AnalysisResult, score *intelligence.Score, status string) (bool, int) {
	if s.PG == nil {
		return false, 0
	}

	// Gunakan context background untuk persistensi agar tidak terpengaruh request cancel
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer dbCancel()

	payload := map[string]interface{}{
		"result": analysis,
		"score":  score,
	}
	siteID, err := s.PG.SaveScanResult(dbCtx, url, score.IsGambling, score.Priority, score.Final, status, payload)
	if err != nil {
		log.Printf("⚠️  [Postgres] Failed to save result: %v", err)
		return false, 0
	}

	if len(analysis.Artifacts) > 0 {
		var artMaps []map[string]string
		for _, a := range analysis.Artifacts {
			artMaps = append(artMaps, map[string]string{
				"type": a.Type, "value": a.Value, "name": a.Name,
			})
		}
		if err := s.PG.SaveArtifacts(dbCtx, siteID, artMaps); err != nil {
			log.Printf("⚠️  [Postgres] Failed to save artifacts: %v", err)
		}
	}

	if analysis.Links != nil {
		linkMap := map[string][]string{
			"internal":  analysis.Links.Internal,
			"external":  analysis.Links.External,
			"iframe":    analysis.Links.Iframes,
			"alternate": analysis.Links.Alternate,
			"register":  analysis.Links.Register,
		}
		if err := s.PG.SaveLinks(dbCtx, siteID, linkMap); err != nil {
			log.Printf("⚠️  [Postgres] Failed to save links: %v", err)
		}

		if len(analysis.Links.Images) > 0 {
			if err := s.PG.SaveImages(dbCtx, siteID, analysis.Links.Images); err != nil {
				log.Printf("⚠️  [Postgres] Failed to save images: %v", err)
			}
		}
	}

	if len(analysis.TrackingIDs) > 0 {
		var tMaps []map[string]string
		for _, t := range analysis.TrackingIDs {
			tMaps = append(tMaps, map[string]string{
				"platform": t.Platform,
				"value":    t.Value,
			})
		}
		if err := s.PG.SaveTrackingIDs(dbCtx, siteID, tMaps); err != nil {
			log.Printf("⚠️  [Postgres] Failed to save tracking IDs: %v", err)
		}
	}

	if analysis.Links != nil && len(analysis.Links.Apps) > 0 {
		if err := s.PG.SaveApps(dbCtx, siteID, analysis.Links.Apps); err != nil {
			log.Printf("⚠️  [Postgres] Failed to save apps: %v", err)
		}
	}

	if len(analysis.Fingerprints) > 0 {
		var fps []map[string]string
		for _, f := range analysis.Fingerprints {
			fps = append(fps, map[string]string{"kind": f.Kind, "value": f.Value})
		}
		if err := s.PG.SaveFingerprints(dbCtx, siteID, fps); err != nil {
			log.Printf("⚠️  [Postgres] Failed to save fingerprints: %v", err)
		}
	}
	if err := s.PG.SaveWidgetLinks(dbCtx, siteID, analysis.WidgetLinks); err != nil {
		log.Printf("⚠️  [Postgres] Failed to save widget links: %v", err)
	}
	if analysis.AuthSurface != nil {
		if err := s.PG.SaveAuthSurface(dbCtx, siteID, analysis.AuthSurface); err != nil {
			log.Printf("⚠️  [Postgres] Failed to save auth surface: %v", err)
		}
	}
	market := ""
	if score.IsGambling {
		market = crawler.Market(analysis, url)
	}
	s.PG.SaveMarket(dbCtx, siteID, market)
	if err := s.PG.SaveInjection(dbCtx, siteID, analysis.Injection, analysis.Injection != nil && analysis.Injection.Detected); err != nil {
		log.Printf("⚠️  [Postgres] Failed to save injection: %v", err)
	}
	// Situs korban penyusupan: tujuan judol dari versi tersembunyi (situs operator) dianalisis otomatis
	if inj := analysis.Injection; inj != nil && inj.Detected && s.TaskClient != nil {
		for _, target := range inj.CloakTargets {
			payload, _ := json.Marshal(map[string]string{"url": target})
			if _, err := s.TaskClient.Enqueue(asynq.NewTask("scan:url", payload), asynq.Queue("default"), asynq.MaxRetry(1), asynq.Unique(24*time.Hour)); err == nil {
				log.Printf("  🎯 [Injection] Tujuan %s (%s) diantrekan untuk dianalisis", target, inj.CloakBrand)
			}
		}
	}
	if err := s.PG.SaveInternalPages(dbCtx, siteID, analysis.InternalPages); err != nil {
		log.Printf("⚠️  [Postgres] Failed to save internal pages: %v", err)
	}

	if ap := analysis.AppPromo; ap != nil {
		if err := s.PG.SaveAppPromo(dbCtx, siteID, ap.Detected, ap.Evidence, ap.Platforms); err != nil {
			log.Printf("⚠️  [Postgres] Failed to save app promo: %v", err)
		}
	}

	if len(analysis.MetaTokens) > 0 {
		var mtMaps []map[string]string
		for _, t := range analysis.MetaTokens {
			mtMaps = append(mtMaps, map[string]string{
				"service": t.Service,
				"token":   t.Token,
			})
		}
		if err := s.PG.SaveTXTTokens(dbCtx, siteID, mtMaps); err != nil {
			log.Printf("⚠️  [Postgres] Failed to save meta tokens: %v", err)
		}
	}

	if analysis.SimHash != "" {
		s.PG.Pool.Exec(dbCtx, `UPDATE sites SET simhash = $1 WHERE id = $2`, analysis.SimHash, siteID)
	}

	return true, siteID
}

// persistToNeo4j menyimpan data graph ke Neo4j — hanya dipanggil untuk situs judol.
// Tanggung jawabnya murni Neo4j: keywords, fingerprint, artifact relationships.
func (s *ScanService) persistToNeo4j(ctx context.Context, url string, analysis *crawler.AnalysisResult, score *intelligence.Score) bool {
	if s.Neo4j == nil {
		return false
	}

	var artMaps []map[string]string
	for _, a := range analysis.Artifacts {
		artMaps = append(artMaps, map[string]string{
			"type": a.Type, "value": a.Value, "name": a.Name,
		})
	}

	err := s.Neo4j.SaveAnalysisResult(
		ctx, url,
		score.IsGambling, score.Final, score.Priority,
		analysis.FoundKeywords,
		analysis.Fingerprint.HTMLHash, analysis.Fingerprint.ScriptsHash,
		artMaps,
	)
	if err != nil {
		log.Printf("Warning: Failed to save to Neo4j: %v", err)
		return false
	}

	return true
}

// runInfraScan melakukan DNS, SSL, dan TXT lookup.
// Dipanggil untuk SEMUA situs — bukan hanya judol.
// Postgres: selalu disimpan.
// Neo4j: hanya jika isGambling = true.
func (s *ScanService) runInfraScan(ctx context.Context, url string, siteID int, isGambling bool) {
	infraInfo, err := s.Scanner.Scan(url)
	if err != nil {
		log.Printf("Infra scan failed for %s: %v", url, err)
		return
	}

	sslThumb := s.Scanner.GetSSLThumbprint(url)
	txtTokens := s.Scanner.ScanTXT(url)

	// --- Gunakan context background untuk persistensi infrastruktur ---
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer dbCancel()

	// --- Postgres: simpan semua infra data ---
	if s.PG != nil {
		if len(infraInfo.IPs) > 0 {
			if err := s.PG.SaveIPs(dbCtx, siteID, infraInfo.IPs); err != nil {
				log.Printf("Warning: Failed to save IPs: %v", err)
			}
		}
		if len(infraInfo.NameServers) > 0 {
			if err := s.PG.SaveNameservers(dbCtx, siteID, infraInfo.NameServers); err != nil {
				log.Printf("Warning: Failed to save nameservers: %v", err)
			}
		}
		if sslThumb != "" {
			if err := s.PG.SaveSSLCert(dbCtx, siteID, sslThumb); err != nil {
				log.Printf("Warning: Failed to save SSL cert: %v", err)
			}
		}
		if len(txtTokens) > 0 {
			var tMaps []map[string]string
			for _, t := range txtTokens {
				tMaps = append(tMaps, map[string]string{
					"service": t.Service,
					"token":   t.Token,
				})
			}
			if err := s.PG.SaveTXTTokens(dbCtx, siteID, tMaps); err != nil {
				log.Printf("Warning: Failed to save TXT tokens: %v", err)
			}
		}
	}

	// --- Neo4j: hanya situs judol ---
	if isGambling && s.Neo4j != nil {
		s.Neo4j.SaveInfraInfo(dbCtx, url, infraInfo.IPs, infraInfo.NameServers, sslThumb)
	}
}

// enqueuePartnerScans mengantrekan analisis penuh untuk situs rekanan. Situs yang di-scan
// dalam 24 jam terakhir dilewati agar tidak berputar (A → B → A) atau membuang kuota.
func (s *ScanService) enqueuePartnerScans(ctx context.Context, partners []string) int {
	if s.TaskClient == nil || s.PG == nil {
		return 0
	}
	n := 0
	seen := map[string]bool{}
	for _, p := range partners {
		u, err := neturl.Parse(p)
		if err != nil || u.Hostname() == "" {
			continue
		}
		root := u.Scheme + "://" + strings.ToLower(u.Host) // beranda situs rekanan, bukan URL ?ref=…
		if seen[root] || s.PG.ScannedWithin(ctx, root, 24*time.Hour) {
			continue
		}
		seen[root] = true
		payload, _ := json.Marshal(map[string]string{"url": root})
		if _, err := s.TaskClient.Enqueue(asynq.NewTask("scan:url", payload, asynq.MaxRetry(1)), asynq.Queue("default")); err != nil {
			log.Printf("⚠️  [Partner] Gagal antre %s: %v", root, err)
			continue
		}
		log.Printf("  🤝 [Partner] %s diantrekan untuk analisis penuh", root)
		n++
		if n == 5 {
			break
		}
	}
	return n
}

// EnqueueEnrichment mengantrekan enrichment untuk situs yang sudah ada (tombol "Perkaya Sekarang").
func (s *ScanService) EnqueueEnrichment(siteID int, url string) error {
	if s.TaskClient == nil {
		return fmt.Errorf("antrean background (Redis) tidak aktif")
	}
	s.enqueueEnrichment(siteID, url)
	return nil
}

// enqueueEnrichment memasukkan task URLScan + VirusTotal ke background queue.
// Non-blocking — kalau queue penuh atau TaskClient nil, diabaikan.
func (s *ScanService) enqueueEnrichment(siteID int, url string) {
	payload, err := json.Marshal(map[string]interface{}{
		"site_id": siteID,
		"url":     url,
	})
	if err != nil {
		return
	}
	task := asynq.NewTask("enrich:site", payload, asynq.MaxRetry(2))
	if _, err := s.TaskClient.Enqueue(task, asynq.Queue("enrichment")); err != nil {
		log.Printf("⚠️  [Enrich] Failed to enqueue task for site %d: %v", siteID, err)
	}
}

func (s *ScanService) loadDetectionKeywords(ctx context.Context) []crawler.WeightedKeyword {
	if s.PG == nil {
		return []crawler.WeightedKeyword{}
	}
	rows, err := s.PG.ListDetectionKeywords(ctx, true)
	if err != nil {
		return []crawler.WeightedKeyword{}
	}
	kws := make([]crawler.WeightedKeyword, len(rows))
	for i, r := range rows {
		kws[i] = crawler.WeightedKeyword{Keyword: r.Keyword, Weight: r.Weight}
	}
	return kws
}

// externalRegisterLinks: link pendaftaran yang mengarah ke domain lain (bukan shortener itu sendiri).
func externalRegisterLinks(siteURL string, register []string) []string {
	self := ""
	if u, err := neturl.Parse(siteURL); err == nil {
		self = strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	}
	var out []string
	for _, r := range register {
		u, err := neturl.Parse(r)
		if err != nil || u.Hostname() == "" || crawler.IsShortenerURL(r) {
			continue
		}
		if strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.") != self {
			out = append(out, r)
		}
	}
	return out
}

func mergeLinks(first, rest []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range append(append([]string{}, first...), rest...) {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	return out
}
