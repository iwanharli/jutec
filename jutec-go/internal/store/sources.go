package store

import (
	"context"
	"encoding/json"
	"strings"
)

// SiteHosts: host (tanpa www) → id situs, untuk dicocokkan dengan daftar TrustPositif.
func (db *PostgresDB) SiteHosts(ctx context.Context) (map[string][]int, error) {
	rows, err := db.Pool.Query(ctx, `SELECT id, url FROM sites`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]int{}
	for rows.Next() {
		var id int
		var u string
		if rows.Scan(&id, &u) == nil {
			if h := hostOf(u); h != "" {
				out[h] = append(out[h], id)
			}
		}
	}
	return out, rows.Err()
}

// SaveTrustPositif menandai situs terdaftar/tidak. tp_listed_since diisi saat pertama kali terdaftar.
func (db *PostgresDB) SaveTrustPositif(ctx context.Context, listedIDs []int) (newlyListed int, err error) {
	tag, err := db.Pool.Exec(ctx, `
		UPDATE sites SET tp_listed_since = NOW() WHERE id = ANY($1) AND NOT COALESCE(tp_listed, false)`, listedIDs)
	if err != nil {
		return 0, err
	}
	_, err = db.Pool.Exec(ctx, `
		UPDATE sites SET tp_listed = (id = ANY($1)), tp_checked_at = NOW(),
		       tp_listed_since = CASE WHEN id = ANY($1) THEN tp_listed_since END`, listedIDs)
	return int(tag.RowsAffected()), err
}

// UnknownHosts: host yang belum punya baris situs sama sekali (URL apa pun di host itu).
func (db *PostgresDB) UnknownHosts(ctx context.Context, hosts []string) []string {
	if len(hosts) == 0 {
		return nil
	}
	known := map[string]bool{}
	rows, err := db.Pool.Query(ctx, `
		SELECT DISTINCT regexp_replace(lower(split_part(split_part(url, '://', 2), '/', 1)), '^www\.', '')
		FROM sites WHERE regexp_replace(lower(split_part(split_part(url, '://', 2), '/', 1)), '^www\.', '') = ANY($1)`, hosts)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var h string
		if rows.Scan(&h) == nil {
			known[h] = true
		}
	}
	var out []string
	for _, h := range hosts {
		if !known[strings.ToLower(h)] {
			out = append(out, h)
		}
	}
	return out
}

// TakeoverTargets: situs judol Indonesia yang riwayat domainnya belum dicek (atau >30 hari lalu).
func (db *PostgresDB) TakeoverTargets(ctx context.Context, limit int) (map[int]string, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, url FROM sites s
		WHERE site_is_judol(s.is_gambling, s.url, s.injection, s.market)
		  AND (s.takeover IS NULL OR (s.takeover->>'checked_at')::timestamptz < NOW() - INTERVAL '30 days')
		ORDER BY s.priority = 'High' DESC, s.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]string{}
	for rows.Next() {
		var id int
		var u string
		if rows.Scan(&id, &u) == nil {
			out[id] = u
		}
	}
	return out, rows.Err()
}

// SaveTakeover menyimpan hasil cek riwayat domain ke semua situs di domain induk yang sama.
func (db *PostgresDB) SaveTakeover(ctx context.Context, siteID int, t any) error {
	_, err := db.Pool.Exec(ctx, `UPDATE sites SET takeover = $2 WHERE id = $1`, siteID, t)
	return err
}

type TakeoverRow struct {
	SiteRef
	Takeover  json.RawMessage `json:"takeover"`
	Lifecycle string          `json:"lifecycle"`
	TPListed  *bool           `json:"tp_listed"`
}

// Takeovers: situs judol di domain bekas situs sah, terbaru diambil alih lebih dulu.
func (db *PostgresDB) Takeovers(ctx context.Context, limit int) ([]TakeoverRow, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT s.id, s.url, COALESCE(s.priority,'Low'), COALESCE(s.confidence,0), COALESCE(s.is_gambling,false), s.takeover,
		       site_lifecycle(s.reach_status, s.status, s.offline_since), s.tp_listed
		FROM sites s WHERE COALESCE((s.takeover->>'detected')::boolean, false)
		ORDER BY s.takeover->>'judol_since' DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TakeoverRow{}
	for rows.Next() {
		var r TakeoverRow
		var raw []byte
		if err := rows.Scan(&r.ID, &r.URL, &r.Priority, &r.Confidence, &r.IsGambling, &raw, &r.Lifecycle, &r.TPListed); err != nil {
			return nil, err
		}
		r.Takeover = raw
		out = append(out, r)
	}
	return out, rows.Err()
}
