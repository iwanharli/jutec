package store

import (
	"context"
	"encoding/json"
	"time"

	"jutec-go/internal/ipintel"
)

func (db *PostgresDB) GetIPIntel(ctx context.Context, ip string) (*ipintel.Intel, time.Time, error) {
	var raw []byte
	var at time.Time
	if err := db.Pool.QueryRow(ctx, `SELECT data, fetched_at FROM ip_intel WHERE ip = $1`, ip).Scan(&raw, &at); err != nil {
		return nil, at, err
	}
	var in ipintel.Intel
	return &in, at, json.Unmarshal(raw, &in)
}

// SaveIPIntel memperbarui cache terkini DAN menambah riwayat bila isinya berbeda dari versi terakhir.
func (db *PostgresDB) SaveIPIntel(ctx context.Context, in *ipintel.Intel) error {
	raw, _ := json.Marshal(in)
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO ip_intel (ip, data, fetched_at) VALUES ($1, $2, NOW())
		ON CONFLICT (ip) DO UPDATE SET data = EXCLUDED.data, fetched_at = NOW()`, in.IP, raw); err != nil {
		return err
	}
	// Riwayat: simpan hanya bila berubah (sidik jari tanpa waktu ambil & galat sementara)
	_, err := db.Pool.Exec(ctx, `
		WITH v AS (SELECT $1::text AS ip, $2::jsonb AS data, md5(($2::jsonb - 'fetched_at' - 'errors')::text) AS fp)
		INSERT INTO ip_intel_history (ip, data, fingerprint)
		SELECT v.ip, v.data, v.fp FROM v
		WHERE v.fp IS DISTINCT FROM (SELECT fingerprint FROM ip_intel_history WHERE ip = v.ip ORDER BY fetched_at DESC LIMIT 1)`,
		in.IP, raw)
	return err
}

// IPIntelHistory: semua versi intel sebuah IP, terbaru dulu.
func (db *PostgresDB) IPIntelHistory(ctx context.Context, ip string) ([]ipintel.Intel, error) {
	rows, err := db.Pool.Query(ctx, `SELECT data FROM ip_intel_history WHERE ip = $1 ORDER BY fetched_at DESC LIMIT 50`, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ipintel.Intel{}
	for rows.Next() {
		var raw []byte
		var in ipintel.Intel
		if rows.Scan(&raw) == nil && json.Unmarshal(raw, &in) == nil {
			out = append(out, in)
		}
	}
	return out, nil
}

func (db *PostgresDB) GetSiteOrigin(ctx context.Context, siteID int) (*ipintel.OriginReport, error) {
	var raw []byte
	if err := db.Pool.QueryRow(ctx, `SELECT data FROM site_origin WHERE site_id = $1`, siteID).Scan(&raw); err != nil {
		return nil, err
	}
	var r ipintel.OriginReport
	return &r, json.Unmarshal(raw, &r)
}

func (db *PostgresDB) SaveSiteOrigin(ctx context.Context, siteID int, r *ipintel.OriginReport) error {
	raw, _ := json.Marshal(r)
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO site_origin (site_id, data, checked_at) VALUES ($1, $2, NOW())
		ON CONFLICT (site_id) DO UPDATE SET data = EXCLUDED.data, checked_at = NOW()`, siteID, raw)
	return err
}

// SiteIPs: semua IP yang tercatat untuk situs.
func (db *PostgresDB) SiteIPs(ctx context.Context, siteID int) ([]string, error) {
	rows, err := db.Pool.Query(ctx, `SELECT i.address FROM site_ips s JOIN ips i ON i.id = s.ip_id WHERE s.site_id = $1 ORDER BY i.address`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var ip string
		if rows.Scan(&ip) == nil {
			out = append(out, ip)
		}
	}
	return out, nil
}

// SitesOnIP: situs lain di database yang memakai IP ini (id, url, judol).
func (db *PostgresDB) SitesOnIP(ctx context.Context, ip string, exclude int, limit int) ([]SiteRef, int, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT s.id, s.url, COALESCE(s.priority,'Low'), COALESCE(s.confidence,0), COALESCE(s.is_gambling,false), COUNT(*) OVER()
		FROM site_ips si JOIN ips i ON i.id = si.ip_id JOIN sites s ON s.id = si.site_id
		WHERE i.address = $1 AND s.id <> $2
		ORDER BY s.is_gambling DESC, s.confidence DESC LIMIT $3`, ip, exclude, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []SiteRef{}
	total := 0
	for rows.Next() {
		var r SiteRef
		if rows.Scan(&r.ID, &r.URL, &r.Priority, &r.Confidence, &r.IsGambling, &total) == nil {
			out = append(out, r)
		}
	}
	return out, total, nil
}
