package store

import (
	"context"
	"time"
)

type VflQuery struct {
	ID          int       `json:"id"`
	Query       string    `json:"query"`
	Kind        string    `json:"kind"`
	Depth       int       `json:"depth"`
	Parent      *string   `json:"parent"`
	Status      string    `json:"status"`
	ResultCount int       `json:"result_count"`
	Error       *string   `json:"error"`
	CreatedAt   time.Time `json:"created_at"`
}

func (db *PostgresDB) SaveVflQuery(ctx context.Context, q, kind string, depth int, rootArtifactID *int, parent, status string, count int, raw, errMsg string) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO vfl_queries (query, kind, depth, root_artifact_id, parent, status, result_count, raw, error)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,NULLIF($8,''),NULLIF($9,''))`,
		q, kind, depth, rootArtifactID, parent, status, count, raw, errMsg)
	return err
}

// VflQueriedWithin: kueri yang sama (selain error) sudah pernah dikirim dalam rentang waktu ini.
func (db *PostgresDB) VflQueriedWithin(ctx context.Context, q string, d time.Duration) bool {
	var ok bool
	db.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM vfl_queries WHERE query=$1 AND status<>'error' AND created_at > $2)`,
		q, time.Now().Add(-d)).Scan(&ok)
	return ok
}

func (db *PostgresDB) VflQueriesToday(ctx context.Context) int {
	var n int
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM vfl_queries WHERE created_at > NOW() - INTERVAL '24 hours'`).Scan(&n)
	return n
}

func (db *PostgresDB) SaveLeakKomInfoSource(ctx context.Context, phone, nik, provider, regDate, source string, artifactID *int) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO leak_kominfo (phone, nik, provider, reg_date, artifact_id, source)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (phone, nik, reg_date) DO UPDATE SET artifact_id = COALESCE(leak_kominfo.artifact_id, EXCLUDED.artifact_id)`,
		phone, nik, provider, regDate, artifactID, source)
	return err
}

// VflPendingPhones: nomor HP/WA dari situs judol yang belum pernah dicek ke bot, paling banyak dipakai situs dulu.
func (db *PostgresDB) VflPendingPhones(ctx context.Context, limit int) ([]struct {
	ID    int
	Phone string
}, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT a.id, regexp_replace(a.value, '^\+', '') FROM artifacts a
		JOIN site_artifacts sa ON sa.artifact_id = a.id
		JOIN sites s ON s.id = sa.site_id AND s.is_gambling
		WHERE a.type IN ('Contact','Phone') AND a.value ~ '^\+?628[0-9]{8,12}$'
		  AND NOT EXISTS (SELECT 1 FROM vfl_queries v WHERE v.query = regexp_replace(a.value, '^\+', '') AND v.status <> 'error')
		GROUP BY a.id ORDER BY COUNT(DISTINCT sa.site_id) DESC, a.id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		ID    int
		Phone string
	}
	for rows.Next() {
		var r struct {
			ID    int
			Phone string
		}
		if rows.Scan(&r.ID, &r.Phone) == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

// VflChain: riwayat rantai kueri untuk satu artefak (untuk audit & tampilan).
func (db *PostgresDB) VflChain(ctx context.Context, artifactID int) ([]VflQuery, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, query, kind, depth, parent, status, result_count, error, created_at FROM vfl_queries
		WHERE root_artifact_id = $1 ORDER BY created_at`, artifactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VflQuery{}
	for rows.Next() {
		var q VflQuery
		if rows.Scan(&q.ID, &q.Query, &q.Kind, &q.Depth, &q.Parent, &q.Status, &q.ResultCount, &q.Error, &q.CreatedAt) == nil {
			out = append(out, q)
		}
	}
	return out, nil
}
