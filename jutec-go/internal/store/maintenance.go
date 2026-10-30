package store

import (
	"context"
	"time"
)

// FakeArtifact: artefak tersimpan yang menurut aturan ekstraksi terbaru bukan nomor rekening.
type FakeArtifact struct {
	ID     int      `json:"id"`
	Type   string   `json:"type"`
	Name   string   `json:"name"`
	Value  string   `json:"value"`
	Reason string   `json:"reason"`
	Sites  []string `json:"sites"`
}

// BankArtifactsForReview mengembalikan semua artefak Bank beserta situs pemakainya (maks. 5 per artefak).
func (db *PostgresDB) BankArtifactsForReview(ctx context.Context) ([]FakeArtifact, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT a.id, a.type, COALESCE(a.name, ''), a.value,
		       COALESCE((SELECT array_agg(s.url ORDER BY s.id) FROM (SELECT s.url, s.id FROM site_artifacts sa JOIN sites s ON s.id = sa.site_id
		                 WHERE sa.artifact_id = a.id LIMIT 5) s), '{}')
		FROM artifacts a WHERE a.type = 'Bank' ORDER BY a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FakeArtifact
	for rows.Next() {
		var f FakeArtifact
		if err := rows.Scan(&f.ID, &f.Type, &f.Name, &f.Value, &f.Sites); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// DeleteArtifactsWithBackup menyalin artefak & keterkaitannya ke tabel cadangan, lalu menghapusnya.
func (db *PostgresDB) DeleteArtifactsWithBackup(ctx context.Context, ids []int, actor string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS artifact_cleanup_backup (
		id BIGSERIAL PRIMARY KEY, artifact JSONB NOT NULL, site_links JSONB NOT NULL,
		removed_by TEXT, removed_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO artifact_cleanup_backup (artifact, site_links, removed_by)
		SELECT to_jsonb(a), COALESCE((SELECT jsonb_agg(to_jsonb(sa)) FROM site_artifacts sa WHERE sa.artifact_id = a.id), '[]'), $2
		FROM artifacts a WHERE a.id = ANY($1)`, ids, actor); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM site_artifacts WHERE artifact_id = ANY($1)`, ids); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM artifacts WHERE id = ANY($1)`, ids)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), tx.Commit(ctx)
}

// RescanScopes: cakupan scan ulang. Situs yang sudah diverifikasi manual tidak pernah ikut (keputusannya terkunci).
var RescanScopes = map[string]string{
	"judol":   `s.is_gambling AND s.verified_status IS NULL`,
	"suspect": `s.verified_status IS NULL AND (s.is_gambling OR s.priority IN ('High','Medium'))`,
	"all":     `s.verified_status IS NULL`,
}

func (db *PostgresDB) RescanCounts(ctx context.Context) map[string]int {
	out := map[string]int{}
	for k, where := range RescanScopes {
		var n int
		db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM sites s WHERE `+where).Scan(&n)
		out[k] = n
	}
	return out
}

type RescanTarget struct {
	ID         int    `json:"id"`
	URL        string `json:"url"`
	IsGambling bool   `json:"is_gambling"`
}

func (db *PostgresDB) RescanTargets(ctx context.Context, scope string) ([]RescanTarget, error) {
	where, ok := RescanScopes[scope]
	if !ok {
		where = RescanScopes["judol"]
	}
	rows, err := db.Pool.Query(ctx, `SELECT s.id, s.url, COALESCE(s.is_gambling, false) FROM sites s WHERE `+where+` ORDER BY s.priority = 'High' DESC, s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RescanTarget
	for rows.Next() {
		var t RescanTarget
		if rows.Scan(&t.ID, &t.URL, &t.IsGambling) == nil {
			out = append(out, t)
		}
	}
	return out, rows.Err()
}

// RescanProgress: dari daftar situs, berapa yang sudah di-scan ulang sejak 'since', berapa yang gagal diakses,
// dan berapa yang tadinya judol (wasJudol) kini dinilai bukan judol (false positive terkoreksi).
func (db *PostgresDB) RescanProgress(ctx context.Context, ids, wasJudol []int, since time.Time) (done, failed, corrected int) {
	db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE last_scanned_at >= $3),
		       COUNT(*) FILTER (WHERE last_scanned_at >= $3 AND status = 'unreachable'),
		       COUNT(*) FILTER (WHERE last_scanned_at >= $3 AND id = ANY($2) AND NOT is_gambling)
		FROM sites WHERE id = ANY($1)`, ids, wasJudol, since).Scan(&done, &failed, &corrected)
	return
}

// SearchCacheStats & ClearSearchCache: cache hasil pencarian Google (Serper) — 24 jam.
func (db *PostgresDB) SearchCacheStats(ctx context.Context) (entries int, urls int) {
	db.Pool.QueryRow(ctx, `SELECT COUNT(*), COALESCE(SUM(result_count), 0) FROM discovery_cache WHERE source = 'serper' AND expires_at > NOW()`).Scan(&entries, &urls)
	return
}

func (db *PostgresDB) ClearSearchCache(ctx context.Context) (int, error) {
	tag, err := db.Pool.Exec(ctx, `DELETE FROM discovery_cache WHERE source = 'serper'`)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
