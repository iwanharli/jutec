package store

import (
	"context"
	"encoding/json"
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
	// Bahan keputusan analis — kenapa situs ini dinilai judol
	Contacts   int        `json:"contacts"`    // WA/Telegram/nomor HP
	Apps       int        `json:"apps"`        // link aplikasi/APK
	Keywords   []string   `json:"keywords"`    // kata kunci yang cocok di scan lengkap terakhir
	Rule       string     `json:"rule"`        // aturan skor yang menentukan prioritas
	Lifecycle  string     `json:"lifecycle"`   // active | blocked_alive | uncertain | inactive
	EvidenceID *int       `json:"evidence_id"` // bukti forensik terbaru (untuk thumbnail)
	CreatedAt  *time.Time `json:"created_at"`
}

// VerifyQueue: situs bertanda judol yang belum diverifikasi manusia. Urutan: bukti terlemah dulu
// (tanpa rekening & tanpa klaster) karena paling mungkin salah deteksi — kecuali sort=confidence.
// StrongEvidenceSQL: bukti kuat bahwa situs s adalah judol — rekening/e-wallet/QRIS/kripto, provider game,
// atau tergabung dalam klaster operator. Situs seperti ini dianggap "terkonfirmasi mesin" dan tidak perlu
// diperiksa analis; antrean verifikasi hanya berisi yang buktinya lemah/sedang.
const StrongEvidenceSQL = `(EXISTS (SELECT 1 FROM site_artifacts sa JOIN artifacts a ON a.id = sa.artifact_id
		WHERE sa.site_id = s.id AND a.type IN ('Bank','E-Wallet','VirtualAccount','QRIS','Crypto'))
	OR EXISTS (SELECT 1 FROM site_operators so JOIN operators o ON o.id = so.operator_id AND o.platform = 'Klaster' WHERE so.site_id = s.id)
	OR COALESCE((SELECT jsonb_array_length(l.result->'result'->'network_signals'->'game_providers') FROM scan_logs l
		WHERE l.site_id = s.id AND jsonb_typeof(l.result->'result'->'network_signals'->'game_providers') = 'array'
		ORDER BY l.created_at DESC LIMIT 1), 0) > 0)`

func (db *PostgresDB) VerifyQueue(ctx context.Context, priority, sort string, limit, offset int, includeStrong bool) ([]VerifyQueueItem, int, error) {
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
		       (SELECT COUNT(*) FROM site_artifacts sa JOIN artifacts a ON a.id=sa.artifact_id
		         WHERE sa.site_id=s.id AND a.type IN ('Contact','Phone')) AS contacts,
		       (SELECT COUNT(*) FROM site_apps sp WHERE sp.site_id=s.id) AS apps,
		       COALESCE(lk.kw, '[]'::jsonb), COALESCE(lk.rule, ''),
		       site_lifecycle(s.reach_status, s.status, s.offline_since),
		       (SELECT e.id FROM evidence_captures e WHERE e.site_id=s.id AND e.status='done' ORDER BY e.id DESC LIMIT 1),
		       s.created_at,
		       COUNT(*) OVER()
		FROM sites s
		LEFT JOIN LATERAL (
			SELECT l.result->'result'->'found_keywords' AS kw, l.result->'score'->>'rule' AS rule
			FROM scan_logs l WHERE l.site_id = s.id AND NOT (l.result ? 'error') AND l.result ? 'score'
			ORDER BY l.created_at DESC LIMIT 1) lk ON true
		WHERE s.is_gambling AND s.verified_status IS NULL AND ($1 = '' OR s.priority = $1) AND ($4 OR NOT `+StrongEvidenceSQL+`)
		ORDER BY `+order+`, s.id LIMIT $2 OFFSET $3`, priority, limit, offset, includeStrong)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []VerifyQueueItem{}
	total := 0
	for rows.Next() {
		var v VerifyQueueItem
		var kw []byte
		if err := rows.Scan(&v.ID, &v.URL, &v.Priority, &v.Confidence, &v.Status, &v.LastScannedAt, &v.Artifacts, &v.Money, &v.Providers, &v.Clustered,
			&v.Contacts, &v.Apps, &kw, &v.Rule, &v.Lifecycle, &v.EvidenceID, &v.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		if json.Unmarshal(kw, &v.Keywords) != nil || v.Keywords == nil {
			v.Keywords = []string{}
		}
		out = append(out, v)
	}
	return out, total, nil
}

// VerifyStats: ringkasan kemajuan verifikasi.
func (db *PostgresDB) VerifyStats(ctx context.Context) map[string]int {
	out := map[string]int{}
	var a, b, c, d int
	// "pending" = perlu analis (bukti lemah/sedang); bukti kuat dihitung terpisah sebagai terkonfirmasi mesin
	var e, f, m int
	db.Pool.QueryRow(ctx, `SELECT
		COUNT(*) FILTER (WHERE s.is_gambling AND s.verified_status IS NULL AND NOT strong),
		COUNT(*) FILTER (WHERE s.is_gambling AND s.verified_status IS NULL AND NOT strong AND s.priority='High'),
		COUNT(*) FILTER (WHERE s.verified_status='judol'),
		COUNT(*) FILTER (WHERE s.verified_status='bukan_judol'),
		COUNT(*) FILTER (WHERE s.is_gambling AND s.verified_status IS NULL AND NOT strong AND s.priority='Medium'),
		COUNT(*) FILTER (WHERE s.is_gambling AND s.verified_status IS NULL AND NOT strong AND s.priority='Low'),
		COUNT(*) FILTER (WHERE s.is_gambling AND s.verified_status IS NULL AND strong)
		FROM sites s CROSS JOIN LATERAL (SELECT `+StrongEvidenceSQL+` AS strong) x`).Scan(&a, &b, &c, &d, &e, &f, &m)
	out["pending"], out["pending_high"], out["verified_judol"], out["verified_not"] = a, b, c, d
	out["pending_medium"], out["pending_low"], out["machine_confirmed"] = e, f, m
	// Keputusan hari ini (WIB) — tidak termasuk pembatalan
	var g int
	db.Pool.QueryRow(ctx, `SELECT COUNT(DISTINCT site_id) FROM site_verifications
		WHERE status <> 'reset' AND at >= date_trunc('day', NOW() AT TIME ZONE 'Asia/Jakarta') AT TIME ZONE 'Asia/Jakarta'`).Scan(&g)
	out["today"] = g
	return out
}
