package store

import (
	"context"
	"time"
)

type ImageRecord struct {
	ID        int       `json:"id"`
	URL       string    `json:"url"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"created_at"`
}

type ImageDomainStat struct {
	Domain    string   `json:"domain"`
	ImageCount int     `json:"image_count"`
	SiteCount int      `json:"site_count"`
	Sites     []string `json:"sites"`
}

// SaveImages menyimpan URL gambar eksternal dari satu situs.
// Fully idempotent — duplikat diabaikan.
func (db *PostgresDB) SaveImages(ctx context.Context, siteID int, imageURLs []string) error {
	for _, rawURL := range imageURLs {
		if rawURL == "" {
			continue
		}

		domain := extractDomain(rawURL)
		if domain == "" {
			continue
		}

		var imageID int
		err := db.Pool.QueryRow(ctx, `
			INSERT INTO images (url, domain)
			VALUES ($1, $2)
			ON CONFLICT (url) DO UPDATE SET domain = EXCLUDED.domain
			RETURNING id
		`, rawURL, domain).Scan(&imageID)
		if err != nil {
			continue
		}

		db.Pool.Exec(ctx, `
			INSERT INTO site_images (site_id, image_id)
			VALUES ($1, $2)
			ON CONFLICT (site_id, image_id) DO NOTHING
		`, siteID, imageID)
	}
	return nil
}

// GetTopImageDomains mengembalikan domain CDN/image yang paling banyak
// dipakai lintas situs judol — berguna untuk identifikasi platform bersama.
func (db *PostgresDB) GetTopImageDomains(ctx context.Context, limit int) ([]ImageDomainStat, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT i.domain,
		       COUNT(DISTINCT i.id)       AS image_count,
		       COUNT(DISTINCT si.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites
		FROM images i
		JOIN site_images si ON si.image_id = i.id
		JOIN sites s ON s.id = si.site_id
		WHERE s.is_gambling = TRUE
		GROUP BY i.domain
		HAVING COUNT(DISTINCT si.site_id) > 1
		ORDER BY site_count DESC, image_count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ImageDomainStat
	for rows.Next() {
		var r ImageDomainStat
		if err := rows.Scan(&r.Domain, &r.ImageCount, &r.SiteCount, &r.Sites); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}
