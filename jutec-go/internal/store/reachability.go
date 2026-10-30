package store

import (
	"context"
	"time"

	"jutec-go/internal/netutil"
)

type ReachEvent struct {
	Status     string    `json:"status"`
	HTTPStatus *int      `json:"http_status"`
	ISPBlocked bool      `json:"isp_blocked"`
	Error      string    `json:"error,omitempty"`
	At         time.Time `json:"at"`
}

// ReachTargets: situs judol (kecuali yang diverifikasi bukan judol) yang paling lama tidak dicek.
// Situs yang sudah tidak aktif cukup dicek mingguan — tetap dipantau agar ketahuan bila hidup lagi.
func (db *PostgresDB) ReachTargets(ctx context.Context, limit int) ([]AutoTarget, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, url, '' FROM sites
		WHERE is_gambling AND verified_status IS DISTINCT FROM 'bukan_judol'
		  AND (reach_checked_at IS NULL OR reach_checked_at < NOW() - CASE
		        WHEN site_lifecycle(reach_status, status, offline_since) = 'inactive' THEN INTERVAL '7 days' ELSE INTERVAL '12 hours' END)
		ORDER BY reach_checked_at NULLS FIRST, priority = 'High' DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return scanTargets(rows)
}

// SaveReach memperbarui status terkini; riwayat hanya ditambah saat status berubah (atau pengecekan pertama).
func (db *PostgresDB) SaveReach(ctx context.Context, siteID int, r netutil.Reach) (changed bool, prev string, err error) {
	db.Pool.QueryRow(ctx, `SELECT COALESCE(reach_status,'') FROM sites WHERE id=$1`, siteID).Scan(&prev)
	blocked := r.Status == "blocked" || r.Status == "blocked_alive"
	down := r.Status == "offline" || r.Status == "blocked" || r.Status == "nxdomain"
	// Di SET, kolom di ruas kanan masih bernilai lama → revived_at dihitung dari siklus hidup sebelum pengecekan ini.
	if _, err = db.Pool.Exec(ctx, `
		UPDATE sites SET reach_status=$2, reach_checked_at=NOW(),
		       blocked_since = CASE WHEN $3 THEN COALESCE(blocked_since, NOW()) ELSE blocked_since END,
		       offline_since = CASE WHEN $4 THEN COALESCE(offline_since, NOW()) ELSE NULL END,
		       revived_at    = CASE WHEN NOT $4 AND site_lifecycle(reach_status, status, offline_since) = 'inactive'
		                            THEN NOW() ELSE revived_at END
		WHERE id=$1`, siteID, r.Status, blocked, down); err != nil {
		return false, prev, err
	}
	if prev == r.Status {
		return false, prev, nil
	}
	var hs *int
	if r.HTTPStatus > 0 {
		hs = &r.HTTPStatus
	}
	_, err = db.Pool.Exec(ctx, `INSERT INTO site_reachability (site_id, status, http_status, isp_blocked, nxdomain, error)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''))`, siteID, r.Status, hs, r.ISPBlocked, r.NXDomain, r.Error)
	return true, prev, err
}

func (db *PostgresDB) ReachHistory(ctx context.Context, siteID int) ([]ReachEvent, error) {
	rows, err := db.Pool.Query(ctx, `SELECT status, http_status, isp_blocked, COALESCE(error,''), checked_at
		FROM site_reachability WHERE site_id=$1 ORDER BY checked_at DESC LIMIT 50`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReachEvent{}
	for rows.Next() {
		var e ReachEvent
		if rows.Scan(&e.Status, &e.HTTPStatus, &e.ISPBlocked, &e.Error, &e.At) == nil {
			out = append(out, e)
		}
	}
	return out, nil
}
