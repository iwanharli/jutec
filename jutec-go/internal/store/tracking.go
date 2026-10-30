package store

import (
	"context"
	"time"
)

type TrackingIDRecord struct {
	ID        int       `json:"id"`
	Platform  string    `json:"platform"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

type TrackingWithSites struct {
	TrackingIDRecord
	SiteCount int      `json:"site_count"`
	Sites     []string `json:"sites"`
	ActiveCount int  `json:"active_site_count"` // situs pemakai yang masih beroperasi
	// Dipakai situs yang sudah mati DAN baru muncul (≤30 hari) di situs aktif — operator pindah domain
	Reappeared bool `json:"reappeared"`
}

// SaveTrackingIDs menyimpan tracking IDs dari satu situs.
// Fully idempotent — duplikat diabaikan.
func (db *PostgresDB) SaveTrackingIDs(ctx context.Context, siteID int, ids []map[string]string) error {
	for _, t := range ids {
		var trackingID int
		err := db.Pool.QueryRow(ctx, `
			INSERT INTO tracking_ids (platform, value)
			VALUES ($1, $2)
			ON CONFLICT (platform, value) DO UPDATE SET platform = EXCLUDED.platform
			RETURNING id
		`, t["platform"], t["value"]).Scan(&trackingID)
		if err != nil {
			continue
		}

		db.Pool.Exec(ctx, `
			INSERT INTO site_tracking (site_id, tracking_id)
			VALUES ($1, $2)
			ON CONFLICT (site_id, tracking_id) DO NOTHING
		`, siteID, trackingID)
	}
	return nil
}

// GetSitesByTracking mencari semua situs yang menggunakan tracking ID yang sama.
func (db *PostgresDB) GetSitesByTracking(ctx context.Context, platform, value string) ([]TrackingWithSites, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT t.id, t.platform, t.value, t.created_at,
		       COUNT(DISTINCT st.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites,
		       COUNT(DISTINCT st.site_id) FILTER (WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive') AS active_count,
		       (BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) = 'inactive') AND BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive' AND st.first_seen > NOW() - INTERVAL '30 days')) AS reappeared
		FROM tracking_ids t
		JOIN site_tracking st ON st.tracking_id = t.id
		JOIN sites s ON s.id = st.site_id
		WHERE t.platform = $1 AND t.value = $2
		GROUP BY t.id
	`, platform, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TrackingWithSites
	for rows.Next() {
		var r TrackingWithSites
		if err := rows.Scan(&r.ID, &r.Platform, &r.Value, &r.CreatedAt, &r.SiteCount, &r.Sites, &r.ActiveCount, &r.Reappeared); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetSiteURLsBySharedTracking mengembalikan URL situs lain yang berbagi tracking ID
// dengan salah satu dari sourceURLs. Berguna untuk menemukan domain satu operator.
// Platform prioritas: AsiaSigma dan GigaGaming (UUID unik per operator), lalu GTM/GA.
func (db *PostgresDB) GetSiteURLsBySharedTracking(ctx context.Context, sourceURLs []string) ([]string, error) {
	if len(sourceURLs) == 0 {
		return nil, nil
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT DISTINCT s.url
		FROM sites s
		JOIN site_tracking st  ON st.site_id  = s.id
		JOIN tracking_ids  tid ON tid.id       = st.tracking_id
		WHERE tid.value IN (
			SELECT DISTINCT tid2.value
			FROM sites       s2
			JOIN site_tracking  st2  ON st2.site_id  = s2.id
			JOIN tracking_ids   tid2 ON tid2.id       = st2.tracking_id
			WHERE s2.url = ANY($1)
			  AND tid2.platform IN (
			        'AsiaSigma','GigaGaming',
			        'GoogleTagManager','GoogleTag','FacebookPixel','LiveChat'
			      )
		)
		AND s.url != ALL($1)
		LIMIT 200
	`, sourceURLs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err == nil {
			urls = append(urls, u)
		}
	}
	return urls, nil
}

// GetTopSharedTracking mengembalikan tracking IDs yang dipakai paling banyak situs judol.
// platform kosong = semua platform.
// Item yang semua situs pemakainya sudah tidak aktif disembunyikan kecuali includeInactive.
func (db *PostgresDB) GetTopSharedTracking(ctx context.Context, platform string, limit int, includeInactive bool) ([]TrackingWithSites, error) {
	filter := platform
	if filter == "" {
		filter = "%"
	}

	rows, err := db.Pool.Query(ctx, `
		SELECT t.id, t.platform, t.value, t.created_at,
		       COUNT(DISTINCT st.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites,
		       COUNT(DISTINCT st.site_id) FILTER (WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive') AS active_count,
		       (BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) = 'inactive') AND BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive' AND st.first_seen > NOW() - INTERVAL '30 days')) AS reappeared
		FROM tracking_ids t
		JOIN site_tracking st ON st.tracking_id = t.id
		JOIN sites s ON s.id = st.site_id
		WHERE s.is_gambling = TRUE
		  AND t.platform LIKE $1
		GROUP BY t.id
		HAVING COUNT(DISTINCT st.site_id) > 1
		   AND ($3 OR COUNT(DISTINCT st.site_id) FILTER (WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive') > 0)
		ORDER BY reappeared DESC, active_count DESC, site_count DESC
		LIMIT $2
	`, filter, limit, includeInactive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TrackingWithSites
	for rows.Next() {
		var r TrackingWithSites
		if err := rows.Scan(&r.ID, &r.Platform, &r.Value, &r.CreatedAt, &r.SiteCount, &r.Sites, &r.ActiveCount, &r.Reappeared); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}
