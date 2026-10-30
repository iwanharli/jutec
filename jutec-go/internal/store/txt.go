package store

import (
	"context"
	"time"
)

type TXTTokenRecord struct {
	ID        int       `json:"id"`
	Service   string    `json:"service"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

type TXTTokenWithSites struct {
	TXTTokenRecord
	SiteCount int      `json:"site_count"`
	Sites     []string `json:"sites"`
}

// SaveTXTTokens menyimpan token verifikasi DNS dari satu situs.
// Fully idempotent.
func (db *PostgresDB) SaveTXTTokens(ctx context.Context, siteID int, tokens []map[string]string) error {
	for _, t := range tokens {
		var tokenID int
		err := db.Pool.QueryRow(ctx, `
			INSERT INTO txt_tokens (service, token)
			VALUES ($1, $2)
			ON CONFLICT (service, token) DO UPDATE SET service = EXCLUDED.service
			RETURNING id
		`, t["service"], t["token"]).Scan(&tokenID)
		if err != nil {
			continue
		}

		db.Pool.Exec(ctx, `
			INSERT INTO site_txt (site_id, token_id)
			VALUES ($1, $2)
			ON CONFLICT (site_id, token_id) DO NOTHING
		`, siteID, tokenID)
	}
	return nil
}

// GetSitesByTXTToken mencari semua situs yang memiliki token verifikasi yang sama.
// Ini adalah sinyal terkuat untuk mengidentifikasi operator yang sama.
func (db *PostgresDB) GetSitesByTXTToken(ctx context.Context, service, token string) ([]TXTTokenWithSites, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT t.id, t.service, t.token, t.created_at,
		       COUNT(DISTINCT st.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites
		FROM txt_tokens t
		JOIN site_txt st ON st.token_id = t.id
		JOIN sites s ON s.id = st.site_id
		WHERE t.service = $1 AND t.token = $2
		GROUP BY t.id
	`, service, token)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTXTRows(rows)
}

// GetTopSharedTXTTokens mengembalikan token yang dipakai paling banyak situs judol.
// service kosong = semua service.
func (db *PostgresDB) GetTopSharedTXTTokens(ctx context.Context, service string, limit int) ([]TXTTokenWithSites, error) {
	filter := service
	if filter == "" {
		filter = "%"
	}

	rows, err := db.Pool.Query(ctx, `
		SELECT t.id, t.service, t.token, t.created_at,
		       COUNT(DISTINCT st.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites
		FROM txt_tokens t
		JOIN site_txt st ON st.token_id = t.id
		JOIN sites s ON s.id = st.site_id
		WHERE s.is_gambling = TRUE
		  AND t.service LIKE $1
		GROUP BY t.id
		HAVING COUNT(DISTINCT st.site_id) > 1
		ORDER BY site_count DESC
		LIMIT $2
	`, filter, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTXTRows(rows)
}

func scanTXTRows(rows interface {
	Next() bool
	Scan(...interface{}) error
	Close()
}) ([]TXTTokenWithSites, error) {
	defer rows.Close()
	var results []TXTTokenWithSites
	for rows.Next() {
		var r TXTTokenWithSites
		if err := rows.Scan(&r.ID, &r.Service, &r.Token, &r.CreatedAt, &r.SiteCount, &r.Sites); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}
