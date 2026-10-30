package store

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

type Operator struct {
	ID              int                `json:"id"`
	Name            string             `json:"name"`
	Platform        string             `json:"platform"`
	OperatorKey     string             `json:"operator_key"`
	SiteCount       int                `json:"site_count"`
	AvgConfidence   float64            `json:"avg_confidence"`
	DiscoveredAt    time.Time          `json:"discovered_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	Sites           []string           `json:"sites,omitempty"`
	Evidence        json.RawMessage    `json:"evidence,omitempty"`
	Members         []OperatorSite     `json:"members,omitempty"`
	SharedArtifacts []OperatorArtifact `json:"shared_artifacts,omitempty"`
}

// OperatorSite adalah satu situs anggota klaster beserta status hidup/blokirnya — bahan profil operator.
type OperatorSite struct {
	ID            int        `json:"id"`
	URL           string     `json:"url"`
	IsGambling    bool       `json:"is_gambling"`
	Confidence    float64    `json:"confidence"`
	ReachStatus   *string    `json:"reach_status"`
	BlockedSince  *time.Time `json:"blocked_since"`
	CreatedAt     time.Time  `json:"created_at"`
	LastScannedAt *time.Time `json:"last_scanned_at"`
}

// OperatorArtifact adalah rekening/kontak yang dipakai ≥2 situs dalam klaster yang sama.
type OperatorArtifact struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Name  string `json:"name"`
	Sites int    `json:"sites"`
}

type OperatorListResult struct {
	Total     int        `json:"total"`
	Operators []Operator `json:"operators"`
}

// DiscoverOperators menjalankan clustering otomatis berdasarkan shared tracking ID.
// Platform prioritas: AsiaSigma (UUID unik per operator) dan GigaGaming (brand code unik).
// Hasilnya di-upsert ke tabel operators dan site_operators.
func (db *PostgresDB) DiscoverOperators(ctx context.Context) (int, error) {
	// 1. Upsert operator dari AsiaSigma UUID dan GigaGaming brand
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO operators (platform, operator_key, site_count, avg_confidence, updated_at)
		SELECT
			t.platform,
			t.value AS operator_key,
			COUNT(DISTINCT st.site_id) AS site_count,
			AVG(s.confidence) AS avg_confidence,
			NOW()
		FROM tracking_ids t
		JOIN site_tracking st ON st.tracking_id = t.id
		JOIN sites s ON s.id = st.site_id
		WHERE t.platform IN ('AsiaSigma', 'GigaGaming')
		  AND s.is_gambling = TRUE
		GROUP BY t.platform, t.value
		HAVING COUNT(DISTINCT st.site_id) >= 1
		ON CONFLICT (platform, operator_key) DO UPDATE SET
			site_count     = EXCLUDED.site_count,
			avg_confidence = EXCLUDED.avg_confidence,
			updated_at     = NOW()
	`)
	if err != nil {
		return 0, err
	}

	// 2. Upsert operator dari Nameserver yang sama (min 2 situs judol)
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO operators (platform, operator_key, site_count, avg_confidence, updated_at)
		SELECT
			'NS-Cluster' AS platform,
			n.host AS operator_key,
			COUNT(DISTINCT sn.site_id) AS site_count,
			AVG(s.confidence) AS avg_confidence,
			NOW()
		FROM nameservers n
		JOIN site_ns sn ON sn.ns_id = n.id
		JOIN sites s ON s.id = sn.site_id
		WHERE s.is_gambling = TRUE
		GROUP BY n.host
		HAVING COUNT(DISTINCT sn.site_id) >= 2
		ON CONFLICT (platform, operator_key) DO UPDATE SET
			site_count     = EXCLUDED.site_count,
			avg_confidence = EXCLUDED.avg_confidence,
			updated_at     = NOW()
	`)
	if err != nil {
		return 0, err
	}

	// 3. Rebuild site_operators junction
	db.Pool.Exec(ctx, `DELETE FROM site_operators`)

	// Link AsiaSigma / GigaGaming
	db.Pool.Exec(ctx, `
		INSERT INTO site_operators (site_id, operator_id)
		SELECT DISTINCT st.site_id, o.id
		FROM tracking_ids t
		JOIN site_tracking st ON st.tracking_id = t.id
		JOIN operators o ON o.platform = t.platform AND o.operator_key = t.value
		ON CONFLICT DO NOTHING
	`)

	// Link NS-Cluster
	db.Pool.Exec(ctx, `
		INSERT INTO site_operators (site_id, operator_id)
		SELECT DISTINCT sn.site_id, o.id
		FROM nameservers n
		JOIN site_ns sn ON sn.ns_id = n.id
		JOIN operators o ON o.platform = 'NS-Cluster' AND o.operator_key = n.host
		ON CONFLICT DO NOTHING
	`)

	// Klaster operator dari atribut kuat bersama (rekening, referral, sertifikat APK, akun CS, …)
	if n, err := db.ClusterOperators(ctx); err != nil {
		return 0, err
	} else if n > 0 {
		log.Printf("🕸️  [Operator] %d klaster operator dibangun dari atribut bersama", n)
	}

	// Hitung total operator yang di-discover
	var total int
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM operators`).Scan(&total)
	return total, nil
}

// ListOperators mengembalikan semua operator dengan pagination.
func (db *PostgresDB) ListOperators(ctx context.Context, platform string, limit, offset int) (*OperatorListResult, error) {
	sql := `
		SELECT id, COALESCE(name, ''), platform, operator_key, site_count, avg_confidence, discovered_at, updated_at, evidence
		FROM operators
		WHERE 1=1
	`
	args := []interface{}{}
	idx := 1

	if platform != "" {
		sql += ` AND platform = $` + itoa(idx)
		args = append(args, platform)
		idx++
	}

	var total int
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM operators`+(func() string {
		if platform != "" {
			return ` WHERE platform = $1`
		}
		return ""
	}()), args[:idx-1]...).Scan(&total)

	sql += ` ORDER BY site_count DESC, avg_confidence DESC LIMIT $` + itoa(idx) + ` OFFSET $` + itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var operators []Operator
	for rows.Next() {
		var o Operator
		if err := rows.Scan(&o.ID, &o.Name, &o.Platform, &o.OperatorKey,
			&o.SiteCount, &o.AvgConfidence, &o.DiscoveredAt, &o.UpdatedAt, &o.Evidence); err != nil {
			return nil, err
		}
		operators = append(operators, o)
	}

	return &OperatorListResult{Total: total, Operators: operators}, nil
}

// GetOperatorByID mengembalikan detail satu operator beserta semua situsnya.
func (db *PostgresDB) GetOperatorByID(ctx context.Context, id int) (*Operator, error) {
	var o Operator
	err := db.Pool.QueryRow(ctx, `
		SELECT id, COALESCE(name, ''), platform, operator_key, site_count, avg_confidence, discovered_at, updated_at, evidence
		FROM operators WHERE id = $1
	`, id).Scan(&o.ID, &o.Name, &o.Platform, &o.OperatorKey,
		&o.SiteCount, &o.AvgConfidence, &o.DiscoveredAt, &o.UpdatedAt, &o.Evidence)
	if err != nil {
		return nil, err
	}

	rows, err := db.Pool.Query(ctx, `
		SELECT s.id, s.url, COALESCE(s.is_gambling, false), COALESCE(s.confidence, 0), s.reach_status, s.blocked_since,
		       s.created_at, s.last_scanned_at
		FROM sites s
		JOIN site_operators so ON so.site_id = s.id
		WHERE so.operator_id = $1
		ORDER BY s.confidence DESC
	`, id)
	if err != nil {
		return &o, nil
	}
	for rows.Next() {
		var m OperatorSite
		if err := rows.Scan(&m.ID, &m.URL, &m.IsGambling, &m.Confidence, &m.ReachStatus, &m.BlockedSince,
			&m.CreatedAt, &m.LastScannedAt); err == nil {
			o.Sites = append(o.Sites, m.URL)
			o.Members = append(o.Members, m)
		}
	}
	rows.Close()

	rows, err = db.Pool.Query(ctx, `
		SELECT a.type, a.value, COALESCE(a.name, ''), COUNT(DISTINCT sa.site_id)
		FROM site_artifacts sa
		JOIN artifacts a ON a.id = sa.artifact_id
		JOIN site_operators so ON so.site_id = sa.site_id AND so.operator_id = $1
		GROUP BY a.id HAVING COUNT(DISTINCT sa.site_id) >= 2
		ORDER BY COUNT(DISTINCT sa.site_id) DESC LIMIT 30
	`, id)
	if err != nil {
		return &o, nil
	}
	defer rows.Close()
	for rows.Next() {
		var a OperatorArtifact
		if err := rows.Scan(&a.Type, &a.Value, &a.Name, &a.Sites); err == nil {
			o.SharedArtifacts = append(o.SharedArtifacts, a)
		}
	}
	return &o, nil
}
