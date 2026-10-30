package store

import (
	"context"
	"fmt"
	"jutec-go/internal/urlnorm"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDB struct {
	Pool *pgxpool.Pool
}

func NewPostgresConnection(connString string) (*PostgresDB, error) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to connect to postgres: %v", err)
	}

	log.Println("Successfully connected to Postgres")
	return &PostgresDB{Pool: pool}, nil
}

func (db *PostgresDB) Close() {
	db.Pool.Close()
}

// SaveScanResult menyimpan hasil scan dan mengembalikan site_id untuk keperluan linking artifacts.
func (db *PostgresDB) SaveScanResult(ctx context.Context, url string, isGambling bool, priority string, confidence float64, status string, result interface{}) (int, error) {
	// Bentuk kanonik di titik simpan: apa pun jalur pemanggilnya, satu situs = satu baris
	if c, err := urlnorm.Canonical(url); err == nil {
		url = c
	}
	var siteID int
	// Konflik dicek pada indeks unik kanonik lower(rtrim(url,'/')) — varian penulisan memperbarui baris yang ada
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO sites (url, is_gambling, priority, confidence, status, last_scanned_at)
		VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
		ON CONFLICT ((lower(rtrim(url, '/')))) DO UPDATE SET
			-- Keputusan verifikasi manual penyidik tidak boleh ditimpa hasil scan otomatis
			is_gambling     = CASE sites.verified_status WHEN 'judol' THEN true WHEN 'bukan_judol' THEN false ELSE EXCLUDED.is_gambling END,
			priority        = CASE WHEN sites.verified_status = 'bukan_judol' THEN 'Low' ELSE EXCLUDED.priority END,
			confidence      = EXCLUDED.confidence,
			status          = EXCLUDED.status,
			last_scanned_at = CURRENT_TIMESTAMP
		RETURNING id
	`, url, isGambling, priority, confidence, status).Scan(&siteID)
	if err != nil {
		return 0, err
	}

	_, err = db.Pool.Exec(ctx, `INSERT INTO scan_logs (site_id, url, result) VALUES ($1, $2, $3)`, siteID, url, result)
	return siteID, err
}

// GetSiteID mengembalikan ID situs berdasarkan URL. Mengembalikan 0 jika tidak ditemukan.
func (db *PostgresDB) GetSiteID(ctx context.Context, rawURL string) int {
	var id int
	if c, err := urlnorm.Canonical(rawURL); err == nil {
		rawURL = c
	}
	db.Pool.QueryRow(ctx, `SELECT id FROM sites WHERE lower(rtrim(url, '/')) = $1`, urlnorm.Key(rawURL)).Scan(&id)
	return id
}

func (db *PostgresDB) IsAlreadyScanned(ctx context.Context, rawURL string) bool {
	// Normalisasi sederhana: hapus trailing slash dan scheme
	cleanURL := strings.TrimRight(rawURL, "/")
	cleanURL = strings.Replace(cleanURL, "https://", "", 1)
	cleanURL = strings.Replace(cleanURL, "http://", "", 1)

	var exists bool
	// Cek apakah ada yang mirip di DB dan statusnya bukan error
	query := `
		SELECT EXISTS(
			SELECT 1 FROM sites 
			WHERE (url LIKE $1 OR url LIKE $2)
			AND status NOT IN ('error', 'unreachable')
		)
	`
	pattern1 := "%" + cleanURL
	pattern2 := "%" + cleanURL + "/"

	err := db.Pool.QueryRow(ctx, query, pattern1, pattern2).Scan(&exists)
	return err == nil && exists
}
