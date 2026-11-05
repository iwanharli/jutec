package store

import (
	"context"
	"time"
)

type URLScanRecord struct {
	ID            int       `json:"id"`
	SiteID        int       `json:"site_id"`
	ScanUUID      string    `json:"scan_uuid"`
	Verdict       string    `json:"verdict"`
	Malicious     bool      `json:"malicious"`
	Score         int       `json:"score"`
	ScreenshotURL string    `json:"screenshot_url"`
	Categories    []string  `json:"categories"`
	Technologies  []string  `json:"technologies"`
	ScannedAt     time.Time `json:"scanned_at"`
	CreatedAt     time.Time `json:"created_at"`
}

type VTRecord struct {
	ID              int       `json:"id"`
	SiteID          int       `json:"site_id"`
	Resource        string    `json:"resource"`
	ResourceType    string    `json:"resource_type"`
	MaliciousCount  int       `json:"malicious_count"`
	SuspiciousCount int       `json:"suspicious_count"`
	HarmlessCount   int       `json:"harmless_count"`
	TotalEngines    int       `json:"total_engines"`
	Categories      []string  `json:"categories"`
	Permalink       string    `json:"permalink"`
	AnalysisDate    time.Time `json:"analysis_date"`
	CreatedAt       time.Time `json:"created_at"`
}

// SaveURLScanResult menyimpan hasil URLScan. Idempotent berdasarkan (site_id, scan_uuid).
func (db *PostgresDB) SaveURLScanResult(ctx context.Context, siteID int, r *URLScanRecord) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO urlscan_results
		    (site_id, scan_uuid, verdict, malicious, score, screenshot_url, categories, technologies, scanned_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (site_id, scan_uuid) DO UPDATE SET
		    verdict       = EXCLUDED.verdict,
		    malicious     = EXCLUDED.malicious,
		    score         = EXCLUDED.score,
		    screenshot_url= EXCLUDED.screenshot_url,
		    categories    = EXCLUDED.categories,
		    technologies  = EXCLUDED.technologies,
		    scanned_at    = EXCLUDED.scanned_at
	`, siteID, r.ScanUUID, r.Verdict, r.Malicious, r.Score,
		r.ScreenshotURL, r.Categories, r.Technologies, r.ScannedAt)
	return err
}

// SaveVTResult menyimpan hasil VirusTotal. Idempotent berdasarkan (site_id, resource, resource_type).
func (db *PostgresDB) SaveVTResult(ctx context.Context, siteID int, r *VTRecord) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO virustotal_results
		    (site_id, resource, resource_type, malicious_count, suspicious_count,
		     harmless_count, total_engines, categories, permalink, analysis_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (site_id, resource, resource_type) DO UPDATE SET
		    malicious_count  = EXCLUDED.malicious_count,
		    suspicious_count = EXCLUDED.suspicious_count,
		    harmless_count   = EXCLUDED.harmless_count,
		    total_engines    = EXCLUDED.total_engines,
		    categories       = EXCLUDED.categories,
		    permalink        = EXCLUDED.permalink,
		    analysis_date    = EXCLUDED.analysis_date
	`, siteID, r.Resource, r.ResourceType, r.MaliciousCount, r.SuspiciousCount,
		r.HarmlessCount, r.TotalEngines, r.Categories, r.Permalink, r.AnalysisDate)
	return err
}

// GetURLScanBySite mengembalikan semua hasil URLScan untuk satu situs.
func (db *PostgresDB) GetURLScanBySite(ctx context.Context, siteID int) ([]URLScanRecord, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, site_id, scan_uuid, verdict, malicious, score,
		       screenshot_url, categories, technologies, scanned_at, created_at
		FROM urlscan_results WHERE site_id = $1 ORDER BY scanned_at DESC
	`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []URLScanRecord
	for rows.Next() {
		var r URLScanRecord
		if err := rows.Scan(&r.ID, &r.SiteID, &r.ScanUUID, &r.Verdict, &r.Malicious,
			&r.Score, &r.ScreenshotURL, &r.Categories, &r.Technologies,
			&r.ScannedAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// EnrichmentBoost berisi sinyal gabungan dari URLScan + VirusTotal untuk satu domain.
type EnrichmentBoost struct {
	Boost        float64 `json:"boost"`         // total boost untuk scoring (0.0–0.5)
	VTMalicious  int     `json:"vt_malicious"`  // jumlah engine VT yang flag malicious
	VTTotal      int     `json:"vt_total"`
	URLScanScore int     `json:"urlscan_score"` // 0-100
	URLScanVerdict string `json:"urlscan_verdict"`
}

// GetEnrichmentBoost mengambil sinyal enrichment yang sudah tersimpan untuk satu situs.
// Dipakai oleh scoring engine saat re-scan.
func (db *PostgresDB) GetEnrichmentBoost(ctx context.Context, siteID int) *EnrichmentBoost {
	result := &EnrichmentBoost{}

	// VirusTotal — ambil yang malicious count tertinggi untuk situs ini
	db.Pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(malicious_count), 0), COALESCE(MAX(total_engines), 0)
		FROM virustotal_results WHERE site_id = $1 AND resource_type = 'domain'
	`, siteID).Scan(&result.VTMalicious, &result.VTTotal)

	// URLScan — ambil scan terbaru
	db.Pool.QueryRow(ctx, `
		SELECT COALESCE(score, 0), COALESCE(verdict, 'unknown')
		FROM urlscan_results WHERE site_id = $1
		ORDER BY scanned_at DESC LIMIT 1
	`, siteID).Scan(&result.URLScanScore, &result.URLScanVerdict)

	// Hitung boost
	boost := 0.0
	if result.VTTotal > 0 {
		ratio := float64(result.VTMalicious) / float64(result.VTTotal)
		if ratio > 0.3 {
			boost += 0.35
		} else if ratio > 0.1 {
			boost += 0.20
		} else if result.VTMalicious > 0 {
			boost += 0.10
		}
	}
	if result.URLScanVerdict == "malicious" {
		boost += 0.15
	} else if result.URLScanVerdict == "suspicious" {
		boost += 0.08
	}
	if boost > 0.5 {
		boost = 0.5
	}
	result.Boost = boost
	return result
}

// GetVTBySite mengembalikan semua hasil VirusTotal untuk satu situs.
func (db *PostgresDB) GetVTBySite(ctx context.Context, siteID int) ([]VTRecord, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, site_id, resource, resource_type, malicious_count, suspicious_count,
		       harmless_count, total_engines, categories, permalink, analysis_date, created_at
		FROM virustotal_results WHERE site_id = $1 ORDER BY created_at DESC
	`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []VTRecord
	for rows.Next() {
		var r VTRecord
		if err := rows.Scan(&r.ID, &r.SiteID, &r.Resource, &r.ResourceType,
			&r.MaliciousCount, &r.SuspiciousCount, &r.HarmlessCount, &r.TotalEngines,
			&r.Categories, &r.Permalink, &r.AnalysisDate, &r.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// NeedsURLScan: hanya situs judi yang belum punya hasil pemindai URL dalam 7 hari — hemat kuota URLScan.
func (db *PostgresDB) NeedsURLScan(ctx context.Context, siteID int) bool {
	var ok bool
	db.Pool.QueryRow(ctx, `SELECT COALESCE(s.is_gambling, false) AND NOT EXISTS (
		SELECT 1 FROM urlscan_results u WHERE u.site_id = s.id AND u.created_at > NOW() - INTERVAL '7 days')
		FROM sites s WHERE s.id = $1`, siteID).Scan(&ok)
	return ok
}
