package store

import (
	"context"
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
