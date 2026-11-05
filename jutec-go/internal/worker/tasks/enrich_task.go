package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"jutec-go/internal/cfscan"
	"log"
	"net/url"
	"time"

	"jutec-go/internal/store"
	"jutec-go/internal/urlscan"
	"jutec-go/internal/virustotal"

	"github.com/hibiken/asynq"
)

const TypeEnrichSite = "enrich:site"

type EnrichPayload struct {
	SiteID int    `json:"site_id"`
	URL    string `json:"url"`
}

func NewEnrichTask(siteID int, rawURL string) (*asynq.Task, error) {
	payload, err := json.Marshal(EnrichPayload{SiteID: siteID, URL: rawURL})
	if err != nil {
		return nil, fmt.Errorf("enrich task marshal: %w", err)
	}
	// MaxRetry rendah karena rate limit tidak bisa di-retry cepat
	return asynq.NewTask(TypeEnrichSite, payload, asynq.MaxRetry(2)), nil
}

type EnrichHandler struct {
	URLScan *urlscan.Client
	VT      *virustotal.Client
	PG      *store.PostgresDB
}

func NewEnrichHandler(us *urlscan.Client, vt *virustotal.Client, pg *store.PostgresDB) *EnrichHandler {
	return &EnrichHandler{URLScan: us, VT: vt, PG: pg}
}

func (h *EnrichHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p EnrichPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("enrich: unmarshal payload: %w", err)
	}

	log.Printf("🔍 [Enrich] site_id=%d url=%s", p.SiteID, p.URL)

	u, err := url.Parse(p.URL)
	if err != nil {
		return nil // URL tidak valid, skip
	}
	domain := u.Hostname()

	// --- Pemindai URL: URLScan.io, cadangan Cloudflare URL Scanner saat kena limit ---
	// Hanya situs judi tanpa hasil 7 hari terakhir: ±70% situs terscan bukan judol & tak perlu kuota.
	if h.PG.NeedsURLScan(ctx, p.SiteID) {
		var result *urlscan.ScanResult
		source := "URLScan"
		if h.URLScan != nil && h.URLScan.APIKey != "" && !urlscan.InCooldown() {
			r, err := h.URLScan.SearchExisting(domain) // scan publik yang sudah ada: tanpa kuota
			if err == nil && r == nil {
				r, err = h.URLScan.ScanURL(p.URL)
			}
			switch {
			case errors.Is(err, urlscan.ErrRateLimit):
				urlscan.SetCooldown(15 * time.Minute)
				log.Printf("  ⏸️  [URLScan] Rate limit — jeda 15 menit, beralih ke Cloudflare")
			case err != nil:
				log.Printf("  ⚠️  [URLScan] Error for %s: %v", p.URL, err)
			default:
				result = r
			}
		}
		if result == nil && cfscan.Configured() {
			r, err := cfscan.Scan(p.URL)
			if err != nil {
				log.Printf("  ⚠️  [Cloudflare URL Scanner] %s: %v", p.URL, err)
			} else {
				result, source = r, "Cloudflare"
			}
		}
		if result != nil && result.UUID != "" {
			rec := &store.URLScanRecord{
				ScanUUID: result.UUID, Verdict: result.Verdict, Malicious: result.Malicious, Score: result.Score,
				ScreenshotURL: result.ScreenshotURL, Categories: result.Categories, Technologies: result.Technologies, ScannedAt: result.ScannedAt,
			}
			if err := h.PG.SaveURLScanResult(ctx, p.SiteID, rec); err != nil {
				log.Printf("  ⚠️  [%s] Failed to save result: %v", source, err)
			} else {
				log.Printf("  ✅ [%s] %s → verdict=%s score=%d", source, p.URL, result.Verdict, result.Score)
			}
		}
	}

	// --- VirusTotal: domain + URL ---
	if h.VT != nil && h.VT.APIKey != "" {
		// Domain check
		vtDomain, err := h.VT.CheckDomain(domain)
		switch {
		case errors.Is(err, virustotal.ErrRateLimit):
			log.Printf("  ⚠️  [VirusTotal] Rate limit — skipping site %d", p.SiteID)
		case err != nil:
			log.Printf("  ⚠️  [VirusTotal] Domain error for %s: %v", domain, err)
		case vtDomain != nil:
			rec := &store.VTRecord{
				Resource:        vtDomain.Resource,
				ResourceType:    "domain",
				MaliciousCount:  vtDomain.MaliciousCount,
				SuspiciousCount: vtDomain.SuspiciousCount,
				HarmlessCount:   vtDomain.HarmlessCount,
				TotalEngines:    vtDomain.TotalEngines,
				Categories:      vtDomain.Categories,
				Permalink:       vtDomain.Permalink,
				AnalysisDate:    vtDomain.AnalysisDate,
			}
			if err := h.PG.SaveVTResult(ctx, p.SiteID, rec); err != nil {
				log.Printf("  ⚠️  [VirusTotal] Failed to save domain result: %v", err)
			} else {
				log.Printf("  ✅ [VirusTotal] %s → malicious=%d/%d", domain,
					vtDomain.MaliciousCount, vtDomain.TotalEngines)
			}
		}

		// URL check (distinct from domain — catches path-specific flags)
		vtURL, err := h.VT.CheckURL(p.URL)
		switch {
		case errors.Is(err, virustotal.ErrRateLimit):
			// sudah di-log di atas, skip
		case err != nil:
			log.Printf("  ⚠️  [VirusTotal] URL error for %s: %v", p.URL, err)
		case vtURL != nil && vtURL.TotalEngines > 0:
			rec := &store.VTRecord{
				Resource:        vtURL.Resource,
				ResourceType:    "url",
				MaliciousCount:  vtURL.MaliciousCount,
				SuspiciousCount: vtURL.SuspiciousCount,
				HarmlessCount:   vtURL.HarmlessCount,
				TotalEngines:    vtURL.TotalEngines,
				Categories:      vtURL.Categories,
				Permalink:       vtURL.Permalink,
				AnalysisDate:    vtURL.AnalysisDate,
			}
			if err := h.PG.SaveVTResult(ctx, p.SiteID, rec); err != nil {
				log.Printf("  ⚠️  [VirusTotal] Failed to save URL result: %v", err)
			}
		}
	}

	// Setelah enrichment disimpan, update confidence situs berdasarkan data baru
	h.updateSiteScoreFromEnrichment(ctx, p.SiteID)

	return nil
}

// updateSiteScoreFromEnrichment menarik data enrichment terbaru dan mengupdate
// confidence + status situs di DB tanpa harus crawl ulang.
func (h *EnrichHandler) updateSiteScoreFromEnrichment(ctx context.Context, siteID int) {
	if h.PG == nil {
		return
	}
	eb := h.PG.GetEnrichmentBoost(ctx, siteID)
	if eb == nil || eb.Boost == 0 {
		return
	}

	// Update confidence dengan tambahan boost enrichment (max 1.0)
	_, err := h.PG.Pool.Exec(ctx, `
		UPDATE sites SET
			confidence = LEAST(confidence + $1, 1.0),
			priority   = CASE
				WHEN LEAST(confidence + $1, 1.0) >= 0.75 THEN 'High'
				WHEN LEAST(confidence + $1, 1.0) >= 0.45 THEN 'Medium'
				ELSE priority
			END
		WHERE id = $2 AND confidence + $1 > confidence
	`, eb.Boost, siteID)
	if err != nil {
		log.Printf("  ⚠️  [Enrich] Failed to update score for site %d: %v", siteID, err)
	} else if eb.Boost > 0 {
		log.Printf("  📈 [Enrich] Updated site %d score +%.2f (VT: %d/%d, URLScan: %s)",
			siteID, eb.Boost, eb.VTMalicious, eb.VTTotal, eb.URLScanVerdict)
	}
}
