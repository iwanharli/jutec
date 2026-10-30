package store

import (
	"jutec-go/internal/urlnorm"
	"context"
	"path"
	"strings"
	"time"
)

type AppRecord struct {
	ID           int       `json:"id"`
	URL          string    `json:"url"`
	Filename     string    `json:"filename"`
	Platform     string    `json:"platform"`
	DiscoveredAt time.Time `json:"discovered_at"`
	SiteCount    int       `json:"site_count,omitempty"`
	Sites        []string  `json:"sites,omitempty"`
}

// detectPlatform menentukan platform dari URL app.
func detectPlatform(rawURL string) string {
	lower := strings.ToLower(rawURL)
	switch {
	case strings.HasSuffix(lower, ".apk"), strings.HasSuffix(lower, ".aab"),
		strings.Contains(lower, "play.google.com/store/apps"):
		return "android"
	case strings.HasSuffix(lower, ".ipa"),
		strings.Contains(lower, "apps.apple.com"):
		return "ios"
	default:
		return "unknown"
	}
}

// extractFilename mengambil nama file dari URL (bagian terakhir path).
func extractFilename(rawURL string) string {
	u := strings.Split(rawURL, "?")[0]
	return path.Base(u)
}

// SaveApps menyimpan URL aplikasi mobile dari satu situs.
// Fully idempotent — duplikat diabaikan.
func (db *PostgresDB) SaveApps(ctx context.Context, siteID int, appURLs []string) error {
	for _, rawURL := range appURLs {
		rawURL = urlnorm.AppURL(rawURL)
		if rawURL == "" {
			continue
		}

		filename := extractFilename(rawURL)
		platform := detectPlatform(rawURL)

		var appID int
		err := db.Pool.QueryRow(ctx, `
			INSERT INTO apps (url, filename, platform)
			VALUES ($1, $2, $3)
			ON CONFLICT (url) DO UPDATE SET filename = EXCLUDED.filename
			RETURNING id
		`, rawURL, filename, platform).Scan(&appID)
		if err != nil {
			continue
		}

		db.Pool.Exec(ctx, `
			INSERT INTO site_apps (site_id, app_id)
			VALUES ($1, $2)
			ON CONFLICT (site_id, app_id) DO NOTHING
		`, siteID, appID)
	}
	return nil
}

// GetAppsBySite mengembalikan semua app yang ditemukan di satu situs.
func (db *PostgresDB) GetAppsBySite(ctx context.Context, siteID int) ([]AppRecord, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT a.id, a.url, a.filename, a.platform, a.discovered_at
		FROM apps a
		JOIN site_apps sa ON sa.app_id = a.id
		WHERE sa.site_id = $1
		ORDER BY a.discovered_at DESC
	`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AppRecord
	for rows.Next() {
		var r AppRecord
		if err := rows.Scan(&r.ID, &r.URL, &r.Filename, &r.Platform, &r.DiscoveredAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetTopApps mengembalikan app yang paling banyak ditemukan lintas situs judol.
// Berguna untuk mengidentifikasi APK yang disebarkan oleh banyak operator.
func (db *PostgresDB) GetTopApps(ctx context.Context, limit int) ([]AppRecord, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT a.id, a.url, a.filename, a.platform, a.discovered_at,
		       COUNT(DISTINCT sa.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites
		FROM apps a
		JOIN site_apps sa ON sa.app_id = a.id
		JOIN sites s ON s.id = sa.site_id
		GROUP BY a.id
		ORDER BY site_count DESC, a.discovered_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AppRecord
	for rows.Next() {
		var r AppRecord
		if err := rows.Scan(&r.ID, &r.URL, &r.Filename, &r.Platform, &r.DiscoveredAt, &r.SiteCount, &r.Sites); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}
