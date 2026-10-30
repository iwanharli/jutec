package store

import (
	"context"
	"fmt"
	"time"
)

type SiteVerification struct {
	Status string    `json:"status"`
	Note   string    `json:"note"`
	Actor  string    `json:"actor"`
	At     time.Time `json:"at"`
}

// VerifySite mencatat keputusan penyidik (judol | bukan_judol | reset) dan mengunci status situs.
// "reset" mengembalikan penilaian ke engine otomatis (nilai is_gambling terakhir dipertahankan).
func (db *PostgresDB) VerifySite(ctx context.Context, siteID int, status, note, actor string) error {
	if status != "judol" && status != "bukan_judol" && status != "reset" {
		return fmt.Errorf("status verifikasi tidak dikenal: %q", status)
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var prevG bool
	var prevC float64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(is_gambling,false), COALESCE(confidence,0) FROM sites WHERE id = $1 FOR UPDATE`, siteID).
		Scan(&prevG, &prevC); err != nil {
		return err
	}
	switch status {
	case "judol":
		_, err = tx.Exec(ctx, `UPDATE sites SET verified_status='judol', verified_by=$2, verified_at=NOW(), verified_note=NULLIF($3,''),
			is_gambling=true, priority=CASE WHEN priority='Low' THEN 'High' ELSE priority END WHERE id=$1`, siteID, actor, note)
	case "bukan_judol":
		_, err = tx.Exec(ctx, `UPDATE sites SET verified_status='bukan_judol', verified_by=$2, verified_at=NOW(), verified_note=NULLIF($3,''),
			is_gambling=false, priority='Low' WHERE id=$1`, siteID, actor, note)
	case "reset":
		_, err = tx.Exec(ctx, `UPDATE sites SET verified_status=NULL, verified_by=NULL, verified_at=NULL, verified_note=NULL WHERE id=$1`, siteID)
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO site_verifications (site_id, status, note, actor, prev_is_gambling, prev_confidence)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,$6)`, siteID, status, note, actor, prevG, prevC); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *PostgresDB) SiteVerificationHistory(ctx context.Context, siteID int) ([]SiteVerification, error) {
	rows, err := db.Pool.Query(ctx, `SELECT status, COALESCE(note,''), actor, at FROM site_verifications WHERE site_id=$1 ORDER BY at DESC LIMIT 20`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SiteVerification{}
	for rows.Next() {
		var v SiteVerification
		if rows.Scan(&v.Status, &v.Note, &v.Actor, &v.At) == nil {
			out = append(out, v)
		}
	}
	return out, nil
}

type VerifyQueueItem struct {
	ID            int        `json:"id"`
	URL           string     `json:"url"`
	Priority      string     `json:"priority"`
	Confidence    float64    `json:"confidence"`
	Status        string     `json:"status"`
	LastScannedAt *time.Time `json:"last_scanned_at"`
	Artifacts     int        `json:"artifacts"`
	Money         int        `json:"money"` // rekening/e-wallet/QRIS/kripto
	Providers     int        `json:"providers"`
	Clustered     bool       `json:"clustered"`
}

// VerifyQueue: situs bertanda judol yang belum diverifikasi manusia. Urutan: bukti terlemah dulu
// (tanpa rekening & tanpa klaster) karena paling mungkin salah deteksi — kecuali sort=confidence.
func (db *PostgresDB) VerifyQueue(ctx context.Context, priority, sort string, limit, offset int) ([]VerifyQueueItem, int, error) {
	order := "money ASC, clustered ASC, s.confidence ASC"
	if sort == "confidence" {
		order = "s.confidence DESC"
	} else if sort == "recent" {
		order = "s.last_scanned_at DESC NULLS LAST"
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT s.id, s.url, COALESCE(s.priority,'Low'), COALESCE(s.confidence,0),
		       CASE WHEN s.status='unreachable' THEN 'unreachable' WHEN s.offline_since IS NOT NULL THEN 'offline' ELSE 'active' END,
		       s.last_scanned_at,
		       (SELECT COUNT(*) FROM site_artifacts sa WHERE sa.site_id=s.id) AS arts,
		       (SELECT COUNT(*) FROM site_artifacts sa JOIN artifacts a ON a.id=sa.artifact_id
		         WHERE sa.site_id=s.id AND a.type IN ('Bank','E-Wallet','VirtualAccount','QRIS','Crypto')) AS money,
		       COALESCE((SELECT jsonb_array_length(l.result->'result'->'network_signals'->'game_providers') FROM scan_logs l
		         WHERE l.site_id=s.id AND jsonb_typeof(l.result->'result'->'network_signals'->'game_providers')='array'
		         ORDER BY l.created_at DESC LIMIT 1), 0) AS providers,
		       EXISTS (SELECT 1 FROM site_operators so JOIN operators o ON o.id=so.operator_id AND o.platform='Klaster' WHERE so.site_id=s.id) AS clustered,
		       COUNT(*) OVER()
		FROM sites s
		WHERE s.is_gambling AND s.verified_status IS NULL AND ($1 = '' OR s.priority = $1)
		ORDER BY `+order+`, s.id LIMIT $2 OFFSET $3`, priority, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []VerifyQueueItem{}
	total := 0
	for rows.Next() {
		var v VerifyQueueItem
		if err := rows.Scan(&v.ID, &v.URL, &v.Priority, &v.Confidence, &v.Status, &v.LastScannedAt, &v.Artifacts, &v.Money, &v.Providers, &v.Clustered, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, nil
}

// VerifyStats: ringkasan kemajuan verifikasi.
func (db *PostgresDB) VerifyStats(ctx context.Context) map[string]int {
	out := map[string]int{}
	var a, b, c, d int
	db.Pool.QueryRow(ctx, `SELECT
		COUNT(*) FILTER (WHERE is_gambling AND verified_status IS NULL),
		COUNT(*) FILTER (WHERE is_gambling AND verified_status IS NULL AND priority='High'),
		COUNT(*) FILTER (WHERE verified_status='judol'),
		COUNT(*) FILTER (WHERE verified_status='bukan_judol') FROM sites`).Scan(&a, &b, &c, &d)
	out["pending"], out["pending_high"], out["verified_judol"], out["verified_not"] = a, b, c, d
	return out
}
