package store

import "context"

// AutoTarget: situs sasaran proses otomatis.
type AutoTarget struct {
	ID     int
	URL    string
	Reason string
}

// AutoEvidenceTargets: situs judol High yang masih aktif dan (a) belum punya bukti, atau
// (b) punya rekening/kontak baru yang tercatat setelah bukti terakhir diambil.
func (db *PostgresDB) AutoEvidenceTargets(ctx context.Context, limit int) ([]AutoTarget, error) {
	// Pengambilan yang tertahan (worker mati / task hilang) jangan memblokir situsnya selamanya
	db.Pool.Exec(ctx, `UPDATE evidence_captures SET status = 'error', error = 'kedaluwarsa: tidak diproses worker dalam 15 menit'
		WHERE status = 'pending' AND created_at < NOW() - INTERVAL '15 minutes'`)
	rows, err := db.Pool.Query(ctx, `
		WITH last_ev AS (
			SELECT site_id, MAX(created_at) AS at, BOOL_OR(status = 'pending') AS pending
			FROM evidence_captures GROUP BY site_id
		)
		SELECT s.id, s.url,
		       CASE WHEN le.at IS NULL THEN 'belum ada bukti' ELSE 'rekening/kontak baru sejak bukti terakhir' END
		FROM sites s LEFT JOIN last_ev le ON le.site_id = s.id
		WHERE s.is_gambling AND s.priority = 'High'
		  AND COALESCE(s.status, '') <> 'unreachable' AND s.offline_since IS NULL
		  AND s.last_scanned_at > NOW() - INTERVAL '14 days'
		  AND COALESCE(le.pending, false) = false
		  AND (le.at IS NULL OR EXISTS (
		        SELECT 1 FROM site_artifacts sa JOIN artifacts a ON a.id = sa.artifact_id
		        WHERE sa.site_id = s.id AND sa.first_seen > le.at
		          AND a.type IN ('Bank','E-Wallet','VirtualAccount','QRIS','Contact','Phone','Crypto')))
		ORDER BY (le.at IS NULL) DESC, s.last_scanned_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return scanTargets(rows)
}

// AutoOriginCandidates: situs judol High yang belum pernah dicari IP aslinya (atau sudah >30 hari).
// Penyaringan "di balik Cloudflare" dilakukan pemanggil dari daftar IP-nya.
func (db *PostgresDB) AutoOriginCandidates(ctx context.Context, limit int) ([]AutoTarget, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT s.id, s.url, '' FROM sites s
		LEFT JOIN site_origin o ON o.site_id = s.id
		WHERE s.is_gambling AND s.priority = 'High' AND COALESCE(s.status, '') <> 'unreachable'
		  AND (o.site_id IS NULL OR o.checked_at < NOW() - INTERVAL '30 days')
		  AND EXISTS (SELECT 1 FROM site_ips si WHERE si.site_id = s.id)
		ORDER BY s.last_scanned_at DESC NULLS LAST
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return scanTargets(rows)
}

// AutoWaybackTargets: situs judol High yang arsip Wayback-nya belum pernah diambil.
func (db *PostgresDB) AutoWaybackTargets(ctx context.Context, limit int) ([]AutoTarget, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, url, '' FROM sites
		WHERE is_gambling AND priority = 'High' AND wayback_checked_at IS NULL
		ORDER BY last_scanned_at DESC NULLS LAST LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return scanTargets(rows)
}

func (db *PostgresDB) MarkWaybackChecked(ctx context.Context, siteID int) error {
	_, err := db.Pool.Exec(ctx, `UPDATE sites SET wayback_checked_at = NOW() WHERE id = $1`, siteID)
	return err
}

func scanTargets(rows interface {
	Next() bool
	Scan(...any) error
	Close()
}) ([]AutoTarget, error) {
	defer rows.Close()
	var out []AutoTarget
	for rows.Next() {
		var t AutoTarget
		if err := rows.Scan(&t.ID, &t.URL, &t.Reason); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

type RetentionItem struct {
	ID  int
	Dir string
}

// EvidenceRetentionCandidates: bukti situs yang tidak aktif > days hari, bukan bagian kasus,
// dan situsnya tidak diverifikasi judol oleh analis.
func (db *PostgresDB) EvidenceRetentionCandidates(ctx context.Context, days, limit int) ([]RetentionItem, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT e.id, e.dir FROM evidence_captures e JOIN sites s ON s.id = e.site_id
		WHERE e.status = 'done' AND COALESCE(e.dir, '') <> ''
		  AND site_lifecycle(s.reach_status, s.status, s.offline_since) = 'inactive'
		  AND COALESCE(s.offline_since, s.last_scanned_at) < NOW() - make_interval(days => $1)
		  AND s.verified_status IS DISTINCT FROM 'judol'
		  AND NOT EXISTS (SELECT 1 FROM case_items c WHERE (c.kind = 'evidence' AND c.ref = e.id::text) OR (c.kind = 'site' AND c.ref = s.id::text))
		ORDER BY e.id LIMIT $2`, days, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RetentionItem
	for rows.Next() {
		var r RetentionItem
		if rows.Scan(&r.ID, &r.Dir) == nil {
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

// MarkEvidencePurged: file dihapus kebijakan simpan; catatan & manifest_sha256 tetap ada sebagai jejak.
func (db *PostgresDB) MarkEvidencePurged(ctx context.Context, id, days int) error {
	_, err := db.Pool.Exec(ctx, `UPDATE evidence_captures SET status = 'purged',
		error = 'File dihapus otomatis (kebijakan simpan ' || $2 || ' hari, situs tidak aktif). Hash SHA-256 manifest tetap tercatat.'
		WHERE id = $1`, id, days)
	return err
}
