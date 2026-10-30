package service

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	"jutec-go/internal/leakosint"
	"jutec-go/internal/store"
	"jutec-go/internal/urlscan"
	"jutec-go/internal/virustotal"
)

// AutoEnricher menjalankan enrichment pasca-sweep secara background.
// Semua enrichment berjalan di goroutine terpisah — tidak memblokir response sweep.
type AutoEnricher struct {
	PG       *store.PostgresDB
	LeakCl   *leakosint.Client
	TgSvc    *TelegramService
	URLScan  *urlscan.Client
	VT       *virustotal.Client
}

func NewAutoEnricher(pg *store.PostgresDB, leak *leakosint.Client, tg *TelegramService,
	us *urlscan.Client, vt *virustotal.Client) *AutoEnricher {
	return &AutoEnricher{PG: pg, LeakCl: leak, TgSvc: tg, URLScan: us, VT: vt}
}

// Run dijalankan di goroutine setelah sweep selesai.
func (e *AutoEnricher) Run(ctx context.Context) {
	go func() {
		log.Println("🔄 [AutoEnrich] Mulai enrichment pasca-sweep...")
		e.discoverOperators(ctx)
		e.enrichLeak(ctx)
		e.resolveTelegram(ctx)
		e.enrichURLScan(ctx)
		e.enrichVirusTotal(ctx)
		log.Println("✅ [AutoEnrich] Selesai")
	}()
}

// --- 1. Operator Discovery (SQL murni, instan) ---

func (e *AutoEnricher) discoverOperators(ctx context.Context) {
	if e.PG == nil {
		return
	}
	total, err := e.PG.DiscoverOperators(ctx)
	if err != nil {
		log.Printf("⚠️  [AutoEnrich] Operator discovery error: %v", err)
		return
	}
	log.Printf("🕵️  [AutoEnrich] Operator discovery: %d operator terdeteksi", total)
}

// --- 2. LeakOSINT — kominfo + dukcapil (1.5 detik/request) ---

func (e *AutoEnricher) enrichLeak(ctx context.Context) {
	if e.PG == nil || e.LeakCl == nil {
		return
	}
	artifacts, err := e.PG.GetPhoneArtifactsNotEnriched(ctx)
	if err != nil || len(artifacts) == 0 {
		return
	}
	log.Printf("💊 [AutoEnrich] LeakOSINT: %d nomor HP akan di-enrich", len(artifacts))

	for i, art := range artifacts {
		select {
		case <-ctx.Done():
			return
		default:
		}
		result, err := e.LeakCl.Lookup(art.Phone)
		id := art.ID
		if err != nil {
			log.Printf("⚠️  [AutoEnrich] LeakOSINT error %s: %v", art.Phone, err)
			e.PG.SaveLeakEnrichmentAttempt(ctx, id, art.Phone, "error", err.Error())
		} else {
			for _, entry := range result.KomInfo {
				e.PG.SaveLeakKomInfo(ctx, entry.Phone, entry.NIK, entry.Provider, entry.RegDate, &id)
			}
			for _, entry := range result.Dukcapil {
				e.PG.SaveLeakDukcapil(ctx, entry.Phone, entry.NIK, entry.FullName, entry.Gender,
					entry.BirthDate, entry.Address, entry.District, entry.Location, entry.Region, &id)
			}
			status := "empty"
			if len(result.KomInfo) > 0 || len(result.Dukcapil) > 0 {
				status = "success"
			}
			e.PG.SaveLeakEnrichmentAttempt(ctx, id, art.Phone, status, "")
		}
		if i < len(artifacts)-1 {
			time.Sleep(1500 * time.Millisecond)
		}
	}
	log.Printf("💊 [AutoEnrich] LeakOSINT selesai: %d nomor diproses", len(artifacts))
}

// --- 3. Telegram resolve (rate limit bawaan gotd) ---

func (e *AutoEnricher) resolveTelegram(ctx context.Context) {
	if e.TgSvc == nil {
		return
	}
	resolved, err := e.TgSvc.ResolveAllPending(ctx)
	if err != nil {
		log.Printf("⚠️  [AutoEnrich] Telegram resolve error: %v", err)
		return
	}
	if resolved > 0 {
		log.Printf("📱 [AutoEnrich] Telegram: %d channel di-resolve", resolved)
	}
}

// --- 4. URLScan.io SearchExisting (tanpa submit baru, hanya lookup) ---

func (e *AutoEnricher) enrichURLScan(ctx context.Context) {
	if e.PG == nil || e.URLScan == nil {
		return
	}

	// Ambil situs judol yang belum punya data URLScan
	rows, err := e.PG.Pool.Query(ctx, `
		SELECT s.id, s.url FROM sites s
		WHERE s.is_gambling = TRUE
		  AND NOT EXISTS (SELECT 1 FROM urlscan_results u WHERE u.site_id = s.id)
		ORDER BY s.confidence DESC
		LIMIT 100
	`)
	if err != nil || rows == nil {
		return
	}
	defer rows.Close()

	type site struct{ id int; u string }
	var sites []site
	for rows.Next() {
		var s site
		if err := rows.Scan(&s.id, &s.u); err == nil {
			sites = append(sites, s)
		}
	}

	if len(sites) == 0 {
		return
	}
	log.Printf("🔍 [AutoEnrich] URLScan: %d situs judol akan dicek", len(sites))

	found := 0
	for i, s := range sites {
		select {
		case <-ctx.Done():
			return
		default:
		}

		domain := extractDomainSimple(s.u)
		result, err := e.URLScan.SearchExisting(domain)
		if err != nil {
			if err == urlscan.ErrRateLimit {
				log.Printf("⚠️  [AutoEnrich] URLScan rate limit, berhenti sementara")
				time.Sleep(60 * time.Second)
				continue
			}
			continue
		}
		if result == nil {
			// Tidak ada scan existing — submit baru
			result, err = e.URLScan.ScanURL(s.u)
			if err != nil || result == nil {
				continue
			}
		}

		rec := &store.URLScanRecord{
			SiteID:        s.id,
			ScanUUID:      result.UUID,
			Verdict:       result.Verdict,
			Malicious:     result.Malicious,
			Score:         result.Score,
			ScreenshotURL: result.ScreenshotURL,
			Categories:    result.Categories,
			Technologies:  result.Technologies,
			ScannedAt:     result.ScannedAt,
		}
		if err := e.PG.SaveURLScanResult(ctx, s.id, rec); err == nil {
			found++
			if result.Malicious {
				log.Printf("🚨 [URLScan] %s → MALICIOUS (score:%d)", domain, result.Score)
			}
		}

		// URLScan: sopan 1 detik antar request
		if i < len(sites)-1 {
			time.Sleep(1 * time.Second)
		}
	}
	log.Printf("🔍 [AutoEnrich] URLScan selesai: %d/%d situs berhasil", found, len(sites))
}

// --- 5. VirusTotal CheckDomain (rate limit 4 req/menit = 15 detik/req) ---

func (e *AutoEnricher) enrichVirusTotal(ctx context.Context) {
	if e.PG == nil || e.VT == nil {
		return
	}

	rows, err := e.PG.Pool.Query(ctx, `
		SELECT s.id, s.url FROM sites s
		WHERE s.is_gambling = TRUE
		  AND NOT EXISTS (
		    SELECT 1 FROM virustotal_results v
		    WHERE v.site_id = s.id AND v.resource_type = 'domain'
		  )
		ORDER BY s.confidence DESC
		LIMIT 50
	`)
	if err != nil || rows == nil {
		return
	}
	defer rows.Close()

	type site struct{ id int; u string }
	var sites []site
	for rows.Next() {
		var s site
		if err := rows.Scan(&s.id, &s.u); err == nil {
			sites = append(sites, s)
		}
	}

	if len(sites) == 0 {
		return
	}
	log.Printf("🦠 [AutoEnrich] VirusTotal: %d domain akan dicek", len(sites))

	found := 0
	for i, s := range sites {
		select {
		case <-ctx.Done():
			return
		default:
		}

		domain := extractDomainSimple(s.u)
		result, err := e.VT.CheckDomain(domain)
		if err != nil {
			if err == virustotal.ErrRateLimit {
				log.Printf("⚠️  [AutoEnrich] VirusTotal rate limit, tunggu 60 detik")
				time.Sleep(60 * time.Second)
				// Retry sekali
				result, err = e.VT.CheckDomain(domain)
				if err != nil {
					continue
				}
			} else {
				continue
			}
		}
		if result == nil {
			continue
		}

		rec := &store.VTRecord{
			SiteID:          s.id,
			Resource:        domain,
			ResourceType:    "domain",
			MaliciousCount:  result.MaliciousCount,
			SuspiciousCount: result.SuspiciousCount,
			HarmlessCount:   result.HarmlessCount,
			TotalEngines:    result.TotalEngines,
			Categories:      result.Categories,
			Permalink:       result.Permalink,
			AnalysisDate:    result.AnalysisDate,
		}
		if err := e.PG.SaveVTResult(ctx, s.id, rec); err == nil {
			found++
			if result.MaliciousCount > 2 {
				log.Printf("🚨 [VirusTotal] %s → %d/%d engines flag malicious",
					domain, result.MaliciousCount, result.TotalEngines)
			}
		}

		// VT free: max 4 req/menit = 15 detik/req
		if i < len(sites)-1 {
			time.Sleep(15 * time.Second)
		}
	}
	log.Printf("🦠 [AutoEnrich] VirusTotal selesai: %d/%d domain berhasil", found, len(sites))
}

func extractDomainSimple(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}
