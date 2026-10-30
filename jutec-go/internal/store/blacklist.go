package store

import (
	"context"
	"time"
)

type BlacklistDomain struct {
	ID        int       `json:"id"`
	Domain    string    `json:"domain"`
	Category  string    `json:"category"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// ListBlacklistDomains mengambil semua domain blacklist.
// Jika activeOnly = true, hanya yang is_active = true.
func (db *PostgresDB) ListBlacklistDomains(ctx context.Context, activeOnly bool) ([]BlacklistDomain, error) {
	query := `SELECT id, domain, category, is_active, created_at FROM domain_blacklist`
	if activeOnly {
		query += ` WHERE is_active = true`
	}
	query += ` ORDER BY category, domain`

	rows, err := db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []BlacklistDomain
	for rows.Next() {
		var d BlacklistDomain
		if err := rows.Scan(&d.ID, &d.Domain, &d.Category, &d.IsActive, &d.CreatedAt); err != nil {
			continue
		}
		results = append(results, d)
	}
	return results, nil
}

// GetActiveBlacklistPatterns mengambil hanya daftar domain string aktif.
// Dipakai oleh deep sweep untuk filtering.
func (db *PostgresDB) GetActiveBlacklistPatterns(ctx context.Context) []string {
	rows, err := db.Pool.Query(ctx, `SELECT domain FROM domain_blacklist WHERE is_active = true`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var patterns []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err == nil {
			patterns = append(patterns, d)
		}
	}
	return patterns
}

// CreateBlacklistDomain menambahkan domain baru ke blacklist.
func (db *PostgresDB) CreateBlacklistDomain(ctx context.Context, domain, category string) (*BlacklistDomain, error) {
	// Re-add domain yang sudah ada: aktifkan lagi, kategori lama dipertahankan kecuali diisi.
	var d BlacklistDomain
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO domain_blacklist (domain, category) VALUES ($1, COALESCE(NULLIF($2::text, ''), 'other'))
		 ON CONFLICT (domain) DO UPDATE SET
		   category  = COALESCE(NULLIF($2::text, ''), domain_blacklist.category),
		   is_active = TRUE
		 RETURNING id, domain, category, is_active, created_at`,
		domain, category,
	).Scan(&d.ID, &d.Domain, &d.Category, &d.IsActive, &d.CreatedAt)
	return &d, err
}

// UpdateBlacklistDomain mengubah status aktif domain blacklist.
func (db *PostgresDB) UpdateBlacklistDomain(ctx context.Context, id int, isActive bool) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE domain_blacklist SET is_active = $2 WHERE id = $1`,
		id, isActive,
	)
	return err
}

// DeleteBlacklistDomain menghapus domain dari blacklist.
func (db *PostgresDB) DeleteBlacklistDomain(ctx context.Context, id int) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM domain_blacklist WHERE id = $1`, id)
	return err
}
