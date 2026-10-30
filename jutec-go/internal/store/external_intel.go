package store

import (
	"os"
	"context"
	"encoding/json"
	"time"
)

// AnalyticsPlatforms adalah platform tracking yang ID-nya bisa dicari balik lewat HackerTarget.
var AnalyticsPlatforms = []string{"GoogleAnalytics", "GoogleTag", "GoogleTagManager", "AdSense"}

type TrackingRef struct {
	Platform string `json:"platform"`
	Value    string `json:"value"`
}

// SiteAnalyticsIDs mengembalikan ID GA/GTM/AdSense milik satu situs.
func (db *PostgresDB) SiteAnalyticsIDs(ctx context.Context, siteID int) ([]TrackingRef, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT x.platform, x.value FROM site_tracking j JOIN tracking_ids x ON x.id = j.tracking_id
		WHERE j.site_id = $1 AND x.platform = ANY($2) ORDER BY x.platform, x.value`, siteID, AnalyticsPlatforms)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrackingRef
	for rows.Next() {
		var t TrackingRef
		if err := rows.Scan(&t.Platform, &t.Value); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ReverseAnalyticsFetchedWithin: true bila ID ini sudah ditanyakan (berhasil) dalam rentang waktu tsb.
func (db *PostgresDB) ReverseAnalyticsFetchedWithin(ctx context.Context, value string, d time.Duration) bool {
	var ok bool
	db.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM reverse_analytics_runs
		WHERE tracking_value = $1 AND error IS NULL AND fetched_at > NOW() - make_interval(secs => $2))`, value, d.Seconds()).Scan(&ok)
	return ok
}

func (db *PostgresDB) SaveReverseAnalytics(ctx context.Context, value string, domains []string, fetchErr error) error {
	var errText *string
	if fetchErr != nil {
		s := fetchErr.Error()
		errText = &s
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO reverse_analytics_runs (tracking_value, fetched_at, result_count, error) VALUES ($1, NOW(), $2, $3)
		ON CONFLICT (tracking_value) DO UPDATE SET fetched_at = NOW(), result_count = EXCLUDED.result_count, error = EXCLUDED.error`,
		value, len(domains), errText); err != nil {
		return err
	}
	for _, d := range domains {
		if _, err := db.Pool.Exec(ctx, `
			INSERT INTO reverse_analytics (tracking_value, domain) VALUES ($1, $2)
			ON CONFLICT (tracking_value, domain) DO UPDATE SET fetched_at = NOW()`, value, d); err != nil {
			return err
		}
	}
	return nil
}

// ReverseDomain adalah domain hasil reverse lookup + situs Jutec yang cocok (bila sudah pernah dianalisis).
type ReverseDomain struct {
	Domain string   `json:"domain"`
	Site   *SiteRef `json:"site"`
}

type ReverseAnalyticsResult struct {
	TrackingRef
	FetchedAt *time.Time      `json:"fetched_at"`
	Error     *string         `json:"error"`
	Domains   []ReverseDomain `json:"domains"`
}

// ReverseAnalyticsForSite menyusun hasil tersimpan untuk semua ID analytics situs.
func (db *PostgresDB) ReverseAnalyticsForSite(ctx context.Context, siteID int) ([]ReverseAnalyticsResult, error) {
	ids, err := db.SiteAnalyticsIDs(ctx, siteID)
	if err != nil {
		return nil, err
	}
	out := []ReverseAnalyticsResult{}
	for _, id := range ids {
		r := ReverseAnalyticsResult{TrackingRef: id, Domains: []ReverseDomain{}}
		db.Pool.QueryRow(ctx, `SELECT fetched_at, error FROM reverse_analytics_runs WHERE tracking_value = $1`, id.Value).Scan(&r.FetchedAt, &r.Error)
		rows, err := db.Pool.Query(ctx, `
			SELECT ra.domain, s.id, s.url, COALESCE(s.priority, 'Low'), COALESCE(s.confidence, 0), COALESCE(s.is_gambling, false)
			FROM reverse_analytics ra
			LEFT JOIN LATERAL (
				SELECT * FROM sites s WHERE lower(rtrim(s.url, '/')) IN ('https://' || ra.domain, 'http://' || ra.domain, 'https://www.' || ra.domain)
				ORDER BY s.last_scanned_at DESC NULLS LAST LIMIT 1
			) s ON true
			WHERE ra.tracking_value = $1
			ORDER BY s.is_gambling DESC NULLS LAST, ra.domain`, id.Value)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var d ReverseDomain
			var sid *int
			var url, prio *string
			var conf *float64
			var gamb *bool
			if err := rows.Scan(&d.Domain, &sid, &url, &prio, &conf, &gamb); err != nil {
				rows.Close()
				return nil, err
			}
			if sid != nil && *sid != siteID {
				d.Site = &SiteRef{ID: *sid, URL: *url, Priority: *prio, Confidence: *conf, IsGambling: *gamb}
			} else if sid != nil {
				continue // situs ini sendiri
			}
			r.Domains = append(r.Domains, d)
		}
		rows.Close()
		out = append(out, r)
	}
	return out, nil
}

// SiteTRONAddresses mengembalikan alamat USDT-TRC20 yang tercantum di situs.
func (db *PostgresDB) SiteTRONAddresses(ctx context.Context, siteID int) ([]string, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT x.value FROM site_artifacts j JOIN artifacts x ON x.id = j.artifact_id
		WHERE j.site_id = $1 AND x.type = 'Crypto' AND (x.value ~ '^T[1-9A-HJ-NP-Za-km-z]{33}$' OR x.value ~* '^0x[0-9a-f]{40}$')`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err == nil {
			out = append(out, v)
		}
	}
	return out, rows.Err()
}

// CachedCryptoProfile mengembalikan profil tersimpan bila belum lebih tua dari maxAge.
func (db *PostgresDB) CachedCryptoProfile(ctx context.Context, address string, maxAge time.Duration) json.RawMessage {
	var raw []byte
	if db.Pool.QueryRow(ctx, `SELECT profile FROM crypto_profiles WHERE address = $1 AND fetched_at > NOW() - make_interval(secs => $2)`,
		address, maxAge.Seconds()).Scan(&raw) != nil {
		return nil
	}
	return raw
}

func (db *PostgresDB) SaveCryptoProfile(ctx context.Context, address string, profile any) error {
	raw, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO crypto_profiles (address, profile, fetched_at) VALUES ($1, $2, NOW())
		ON CONFLICT (address) DO UPDATE SET profile = EXCLUDED.profile, fetched_at = NOW()`, address, raw)
	return err
}

// SitesByCryptoAddress memetakan alamat kripto → situs Jutec yang mencantumkannya (kecuali situs ini sendiri).
// Dipakai untuk menandai lawan transaksi yang ternyata dompet situs judol lain.
func (db *PostgresDB) SitesByCryptoAddress(ctx context.Context, addresses []string, exceptSite int) (map[string][]SiteRef, error) {
	out := map[string][]SiteRef{}
	if len(addresses) == 0 {
		return out, nil
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT CASE WHEN x.value ~* '^0x' THEN lower(x.value) ELSE x.value END, s.id, s.url, COALESCE(s.priority, 'Low'), COALESCE(s.confidence, 0), COALESCE(s.is_gambling, false)
		FROM artifacts x JOIN site_artifacts j ON j.artifact_id = x.id JOIN sites s ON s.id = j.site_id
		WHERE x.type = 'Crypto' AND (x.value = ANY($1) OR lower(x.value) = ANY($1)) AND s.id <> $2`, addresses, exceptSite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var addr string
		var r SiteRef
		if err := rows.Scan(&addr, &r.ID, &r.URL, &r.Priority, &r.Confidence, &r.IsGambling); err != nil {
			return nil, err
		}
		out[addr] = append(out[addr], r)
	}
	return out, rows.Err()
}

// AutoReverseTargets: ID GA/GTM/AdSense di situs judol yang masih beroperasi dan belum ditanyakan ke
// HackerTarget dalam 7 hari — untuk putaran otomatis (dibatasi agar kuota gratis harian cukup).
func (db *PostgresDB) AutoReverseTargets(ctx context.Context, limit int) ([]string, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT DISTINCT x.value FROM tracking_ids x
		JOIN site_tracking j ON j.tracking_id = x.id JOIN sites s ON s.id = j.site_id
		WHERE x.platform = ANY($1) AND s.is_gambling
		  AND site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive'
		  AND NOT EXISTS (SELECT 1 FROM reverse_analytics_runs r WHERE r.tracking_value = x.value
		                  AND r.error IS NULL AND r.fetched_at > NOW() - INTERVAL '7 days')
		LIMIT $2`, AnalyticsPlatforms, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if rows.Scan(&v) == nil {
			out = append(out, v)
		}
	}
	return out, rows.Err()
}

// UnknownDomains menyaring domain yang belum pernah dianalisis Jutec.
func (db *PostgresDB) UnknownDomains(ctx context.Context, domains []string) []string {
	var out []string
	for _, d := range domains {
		var n int
		db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM sites WHERE lower(rtrim(url, '/')) IN ('https://' || $1, 'http://' || $1, 'https://www.' || $1)`, d).Scan(&n)
		if n == 0 {
			out = append(out, d)
		}
	}
	return out
}

// AutoCryptoTargets: alamat USDT-TRC20 di situs judol yang profilnya belum ada atau lebih tua dari 24 jam.
func (db *PostgresDB) AutoCryptoTargets(ctx context.Context, limit int) ([]string, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT DISTINCT x.value FROM site_artifacts j JOIN artifacts x ON x.id = j.artifact_id JOIN sites s ON s.id = j.site_id
		WHERE s.is_gambling AND x.type = 'Crypto' AND (x.value ~ '^T[1-9A-HJ-NP-Za-km-z]{33}$' OR (x.value ~* '^0x[0-9a-f]{40}$' AND $2))
		  AND NOT EXISTS (SELECT 1 FROM crypto_profiles p WHERE p.address = x.value AND p.fetched_at > NOW() - INTERVAL '24 hours')
		LIMIT $1`, limit, os.Getenv("ETHERSCAN_API_KEY") != "")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if rows.Scan(&v) == nil {
			out = append(out, v)
		}
	}
	return out, rows.Err()
}
