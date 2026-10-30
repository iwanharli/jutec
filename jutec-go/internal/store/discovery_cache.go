package store

import (
	"context"
	"time"
)

// GetDiscoveryCache mengambil hasil pencarian yang tersimpan jika masih valid (belum expire).
// Mengembalikan nil jika tidak ada cache atau sudah kadaluarsa.
func (db *PostgresDB) GetDiscoveryCache(ctx context.Context, source, query string) []string {
	var urls []string
	err := db.Pool.QueryRow(ctx, `
		SELECT urls FROM discovery_cache
		WHERE source = $1 AND query = $2 AND expires_at > NOW()
	`, source, query).Scan(&urls)
	if err != nil {
		return nil
	}
	return urls
}

// SaveDiscoveryCache menyimpan hasil pencarian ke cache.
// ttl: durasi cache (default 24 jam). Upsert — replace kalau keyword sudah ada.
func (db *PostgresDB) SaveDiscoveryCache(ctx context.Context, source, query string, urls []string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	expiresAt := time.Now().Add(ttl)
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO discovery_cache (source, query, urls, result_count, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (source, query) DO UPDATE SET
			urls         = EXCLUDED.urls,
			result_count = EXCLUDED.result_count,
			discovered_at = NOW(),
			expires_at   = EXCLUDED.expires_at
	`, source, query, urls, len(urls), expiresAt)
	return err
}

// PurgeExpiredCache menghapus semua cache yang sudah kadaluarsa.
func (db *PostgresDB) PurgeExpiredCache(ctx context.Context) (int64, error) {
	tag, err := db.Pool.Exec(ctx, `DELETE FROM discovery_cache WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// GetDiscoveryCacheStats mengembalikan statistik cache.
func (db *PostgresDB) GetDiscoveryCacheStats(ctx context.Context) (map[string]interface{}, error) {
	var total, active, expired int
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM discovery_cache`).Scan(&total)
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM discovery_cache WHERE expires_at > NOW()`).Scan(&active)
	expired = total - active

	rows, err := db.Pool.Query(ctx, `
		SELECT source, COUNT(*) AS cnt, SUM(result_count) AS total_urls
		FROM discovery_cache WHERE expires_at > NOW()
		GROUP BY source
	`)
	if err != nil {
		return map[string]interface{}{
			"total": total, "active": active, "expired": expired,
		}, nil
	}
	defer rows.Close()

	bySource := make(map[string]interface{})
	for rows.Next() {
		var source string
		var cnt, totalURLs int
		if rows.Scan(&source, &cnt, &totalURLs) == nil {
			bySource[source] = map[string]int{"entries": cnt, "total_urls": totalURLs}
		}
	}

	return map[string]interface{}{
		"total":     total,
		"active":    active,
		"expired":   expired,
		"by_source": bySource,
	}, nil
}
