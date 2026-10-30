package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type DetectionKeyword struct {
	ID        int       `json:"id"`
	Keyword   string    `json:"keyword"`
	Weight    float64   `json:"weight"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type SearchKeyword struct {
	ID        int       `json:"id"`
	Keyword   string    `json:"keyword"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// --- Detection Keywords ---

func (db *PostgresDB) ListDetectionKeywords(ctx context.Context, activeOnly bool) ([]DetectionKeyword, error) {
	query := `SELECT id, keyword, weight, is_active, created_at FROM detection_keywords`
	if activeOnly {
		query += ` WHERE is_active = TRUE`
	}
	query += ` ORDER BY id`

	rows, err := db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keywords []DetectionKeyword
	for rows.Next() {
		var k DetectionKeyword
		if err := rows.Scan(&k.ID, &k.Keyword, &k.Weight, &k.IsActive, &k.CreatedAt); err != nil {
			return nil, err
		}
		keywords = append(keywords, k)
	}
	return keywords, nil
}

func (db *PostgresDB) CreateDetectionKeyword(ctx context.Context, keyword string, weight float64) (*DetectionKeyword, error) {
	var k DetectionKeyword
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO detection_keywords (keyword, weight)
		VALUES ($1, $2)
		RETURNING id, keyword, weight, is_active, created_at
	`, keyword, weight).Scan(&k.ID, &k.Keyword, &k.Weight, &k.IsActive, &k.CreatedAt)
	return &k, err
}

// UpdateDetectionKeyword: weight nil = pertahankan bobot lama. Mengembalikan pgx.ErrNoRows bila id tidak ada.
func (db *PostgresDB) UpdateDetectionKeyword(ctx context.Context, id int, weight *float64, isActive bool) error {
	tag, err := db.Pool.Exec(ctx, `
		UPDATE detection_keywords SET weight = COALESCE($1, weight), is_active = $2 WHERE id = $3
	`, weight, isActive, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (db *PostgresDB) DeleteDetectionKeyword(ctx context.Context, id int) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM detection_keywords WHERE id = $1`, id)
	return err
}

// --- Search Keywords ---

func (db *PostgresDB) ListSearchKeywords(ctx context.Context, activeOnly bool) ([]SearchKeyword, error) {
	query := `SELECT id, keyword, is_active, created_at FROM search_keywords`
	if activeOnly {
		query += ` WHERE is_active = TRUE`
	}
	query += ` ORDER BY id`

	rows, err := db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keywords []SearchKeyword
	for rows.Next() {
		var k SearchKeyword
		if err := rows.Scan(&k.ID, &k.Keyword, &k.IsActive, &k.CreatedAt); err != nil {
			return nil, err
		}
		keywords = append(keywords, k)
	}
	return keywords, nil
}

func (db *PostgresDB) CreateSearchKeyword(ctx context.Context, keyword string) (*SearchKeyword, error) {
	var k SearchKeyword
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO search_keywords (keyword)
		VALUES ($1)
		RETURNING id, keyword, is_active, created_at
	`, keyword).Scan(&k.ID, &k.Keyword, &k.IsActive, &k.CreatedAt)
	return &k, err
}

func (db *PostgresDB) UpdateSearchKeyword(ctx context.Context, id int, isActive bool) error {
	_, err := db.Pool.Exec(ctx, `
		UPDATE search_keywords SET is_active = $1 WHERE id = $2
	`, isActive, id)
	return err
}

func (db *PostgresDB) DeleteSearchKeyword(ctx context.Context, id int) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM search_keywords WHERE id = $1`, id)
	return err
}
