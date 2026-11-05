package store

import (
	"context"
	"time"
)

type Artifact struct {
	ID        int       `json:"id"`
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type ArtifactWithSites struct {
	Artifact
	SiteCount int      `json:"site_count"`
	Sites     []string `json:"sites"`
	ActiveCount int  `json:"active_site_count"` // situs pemakai yang masih beroperasi
	// Dipakai situs yang sudah mati DAN baru muncul (≤30 hari) di situs aktif — operator pindah domain
	Reappeared bool `json:"reappeared"`
}

// SaveArtifacts menyimpan artefak dari satu situs ke tabel artifacts + site_artifacts.
// Menggunakan upsert agar tidak duplikat.
func (db *PostgresDB) SaveArtifacts(ctx context.Context, siteID int, artifacts []map[string]string) error {
	for _, art := range artifacts {
		// Upsert artifact
		var artifactID int
		err := db.Pool.QueryRow(ctx, `
			INSERT INTO artifacts (type, value, name)
			VALUES ($1, $2, $3)
			ON CONFLICT (type, value) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		`, art["type"], art["value"], art["name"]).Scan(&artifactID)
		if err != nil {
			return err
		}

		// Link ke site (ignore jika sudah ada)
		_, err = db.Pool.Exec(ctx, `
			INSERT INTO site_artifacts (site_id, artifact_id)
			VALUES ($1, $2)
			ON CONFLICT (site_id, artifact_id) DO NOTHING
		`, siteID, artifactID)
		if err != nil {
			return err
		}
	}
	return nil
}

// SaveArtifactsGetIDs upsert artifacts tanpa site linkage, return list of artifact IDs.
// Digunakan oleh TelegramService untuk link artifact ke channel.
func (db *PostgresDB) SaveArtifactsGetIDs(ctx context.Context, artifacts []map[string]string) ([]int, error) {
	var ids []int
	for _, art := range artifacts {
		var id int
		err := db.Pool.QueryRow(ctx, `
			INSERT INTO artifacts (type, value, name)
			VALUES ($1, $2, $3)
			ON CONFLICT (type, value) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		`, art["type"], art["value"], art["name"]).Scan(&id)
		if err != nil {
			return ids, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// GetSitesByArtifact mencari semua situs yang berbagi artefak yang sama.
// type: Bank | E-Wallet | Crypto | QRIS | Contact
func (db *PostgresDB) GetSitesByArtifact(ctx context.Context, artifactType, value string) ([]ArtifactWithSites, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT a.id, a.type, a.value, a.name, a.created_at,
		       COUNT(DISTINCT sa.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites,
		       COUNT(DISTINCT sa.site_id) FILTER (WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive') AS active_count,
		       (BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) = 'inactive') AND BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive' AND sa.first_seen > NOW() - INTERVAL '30 days')) AS reappeared
		FROM artifacts a
		JOIN site_artifacts sa ON sa.artifact_id = a.id
		JOIN sites s ON s.id = sa.site_id
		WHERE a.type = $1 AND a.value = $2
		GROUP BY a.id
	`, artifactType, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []ArtifactWithSites{}
	for rows.Next() {
		var r ArtifactWithSites
		if err := rows.Scan(&r.ID, &r.Type, &r.Value, &r.Name, &r.CreatedAt, &r.SiteCount, &r.Sites, &r.ActiveCount, &r.Reappeared); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetSitesByArtifactDetailed returns sites (with url, priority, confidence) linked to an artifact.
func (db *PostgresDB) GetSitesByArtifactDetailed(ctx context.Context, artifactType, value string) ([]map[string]interface{}, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT s.id, s.url, s.priority, s.confidence, s.is_gambling
		FROM artifacts a
		JOIN site_artifacts sa ON sa.artifact_id = a.id
		JOIN sites s ON s.id = sa.site_id
		WHERE ($1 = '' OR a.type = $1)
		  AND (a.value ILIKE $2 OR a.name ILIKE $2)
		ORDER BY s.confidence DESC
	`, artifactType, "%"+value+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var url, priority string
		var confidence float64
		var isGambling bool
		if err := rows.Scan(&id, &url, &priority, &confidence, &isGambling); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id": id, "url": url, "priority": priority, "confidence": confidence, "is_gambling": isGambling,
		})
	}
	return results, nil
}

// infraLookupJoins memetakan tipe lookup infrastruktur ke join + kolom yang dicocokkan.
// Tipe di luar map ini dicari sebagai artefak (Bank, Crypto, Contact, dll).
var infraLookupJoins = map[string]string{
	"IP":  `JOIN site_ips j ON j.site_id = s.id JOIN ips x ON x.id = j.ip_id WHERE x.address ILIKE $1`,
	"NS":  `JOIN site_ns j ON j.site_id = s.id JOIN nameservers x ON x.id = j.ns_id WHERE x.host ILIKE $1`,
	"SSL": `JOIN site_ssl j ON j.site_id = s.id JOIN ssl_certs x ON x.id = j.cert_id WHERE x.thumbprint ILIKE $1`,
	// GA4 (G-xxxx) tersimpan sebagai platform GoogleTag
	"GA":  `JOIN site_tracking j ON j.site_id = s.id JOIN tracking_ids x ON x.id = j.tracking_id WHERE x.platform IN ('GoogleAnalytics','GoogleTag') AND x.value ILIKE $1`,
	"GTM": `JOIN site_tracking j ON j.site_id = s.id JOIN tracking_ids x ON x.id = j.tracking_id WHERE x.platform = 'GoogleTagManager' AND x.value ILIKE $1`,
	// Sidik jari infrastruktur: ID CDN (cloudfront:/bunnycdn:), hash favicon, host API
	"FP":  `JOIN site_fingerprints j ON j.site_id = s.id JOIN fingerprints x ON x.id = j.fingerprint_id WHERE x.value ILIKE $1`,
	"FB":  `JOIN site_tracking j ON j.site_id = s.id JOIN tracking_ids x ON x.id = j.tracking_id WHERE x.platform = 'FacebookPixel' AND x.value ILIKE $1`,
}

// GetSitesByLookup mencari situs berdasarkan IP/NS/SSL/tracking ID atau artefak.
func (db *PostgresDB) GetSitesByLookup(ctx context.Context, lookupType, value string) ([]map[string]interface{}, error) {
	join, ok := infraLookupJoins[lookupType]
	if !ok {
		return db.GetSitesByArtifactDetailed(ctx, lookupType, value)
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT DISTINCT s.id, s.url, s.priority, s.confidence, s.is_gambling
		FROM sites s `+join+`
		ORDER BY s.confidence DESC
		LIMIT 500
	`, "%"+value+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var url, priority string
		var confidence float64
		var isGambling bool
		if err := rows.Scan(&id, &url, &priority, &confidence, &isGambling); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id": id, "url": url, "priority": priority, "confidence": confidence, "is_gambling": isGambling,
		})
	}
	return results, nil
}

// GetArtifactFilters returns distinct types and names present in the artifacts table.
func (db *PostgresDB) GetArtifactFilters(ctx context.Context) (map[string][]string, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT DISTINCT type, name FROM artifacts
		WHERE name IS NOT NULL AND name <> ''
		ORDER BY type, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]string{}
	for rows.Next() {
		var t, n string
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		result[t] = append(result[t], n)
	}
	return result, nil
}

// GetTopArtifacts mengembalikan artefak yang dipakai paling banyak situs judol.
// Berguna untuk identifikasi operator yang kelola banyak domain.
// artifactType kosong = semua tipe.
func (db *PostgresDB) GetTopArtifacts(ctx context.Context, artifactType string, limit int) ([]ArtifactWithSites, error) {
	return db.GetTopArtifactsFiltered(ctx, artifactType, "", limit, true, true)
}

// Item yang semua situs pemakainya sudah tidak aktif disembunyikan kecuali includeInactive.
// sharedOnly: hanya artefak yang dipakai >1 situs (pola operator); false = semua, termasuk rekening yang baru muncul di satu situs.
func (db *PostgresDB) GetTopArtifactsFiltered(ctx context.Context, artifactType, artifactName string, limit int, includeInactive, sharedOnly bool) ([]ArtifactWithSites, error) {
	filterType := artifactType
	if filterType == "" {
		filterType = "%"
	}
	filterName := artifactName
	if filterName == "" {
		filterName = "%"
	}

	rows, err := db.Pool.Query(ctx, `
		SELECT a.id, a.type, a.value, a.name, a.created_at,
		       COUNT(DISTINCT sa.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites,
		       COUNT(DISTINCT sa.site_id) FILTER (WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive') AS active_count,
		       (BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) = 'inactive') AND BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive' AND sa.first_seen > NOW() - INTERVAL '30 days')) AS reappeared
		FROM artifacts a
		JOIN site_artifacts sa ON sa.artifact_id = a.id
		JOIN sites s ON s.id = sa.site_id
		WHERE s.is_gambling = TRUE
		  AND a.type LIKE $1
		  AND COALESCE(a.name,'') LIKE $2
		GROUP BY a.id
		HAVING COUNT(DISTINCT sa.site_id) > CASE WHEN $5 THEN 1 ELSE 0 END
		   AND ($4 OR COUNT(DISTINCT sa.site_id) FILTER (WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive') > 0)
		ORDER BY reappeared DESC, active_count DESC, site_count DESC
		LIMIT $3
	`, filterType, filterName, limit, includeInactive, sharedOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ArtifactWithSites
	for rows.Next() {
		var r ArtifactWithSites
		if err := rows.Scan(&r.ID, &r.Type, &r.Value, &r.Name, &r.CreatedAt, &r.SiteCount, &r.Sites, &r.ActiveCount, &r.Reappeared); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}
