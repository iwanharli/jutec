package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Tampilan lintas situs untuk Repository: atribut yang dipakai bersama banyak situs judol.

type SharedRow struct {
	Kind     string    `json:"kind"`
	Value    string    `json:"value"`
	Label    string    `json:"label,omitempty"` // keterangan tambahan (ASN/organisasi, jenis sidik jari…)
	Extra    any       `json:"extra,omitempty"`
	Sites    int       `json:"sites"`
	Judol    int       `json:"judol"`
	Live     int       `json:"live"` // situs judol yang masih beroperasi
	Samples  []SiteRef `json:"samples"`
	LastSeen *time.Time `json:"last_seen,omitempty"`
}

// sharedSources: jenis atribut → (subquery yang menghasilkan kolom key, label, site_id).
var sharedSources = map[string]string{
	"ip": `SELECT i.address AS key, COALESCE(NULLIF(x.data->>'asn','') || ' · ' || COALESCE(NULLIF(x.data->>'as_name',''), x.data->>'org'), '') AS label, j.site_id
	       FROM site_ips j JOIN ips i ON i.id = j.ip_id LEFT JOIN ip_intel x ON x.ip = i.address`,
	"asn": `SELECT x.data->>'asn' AS key, COALESCE(NULLIF(x.data->>'as_name',''), x.data->>'org', '') AS label, j.site_id
	        FROM site_ips j JOIN ips i ON i.id = j.ip_id JOIN ip_intel x ON x.ip = i.address WHERE COALESCE(x.data->>'asn','') <> ''`,
	"ns":  `SELECT n.host AS key, '' AS label, j.site_id FROM site_ns j JOIN nameservers n ON n.id = j.ns_id`,
	"ssl": `SELECT c.thumbprint AS key, '' AS label, j.site_id FROM site_ssl j JOIN ssl_certs c ON c.id = j.cert_id`,
	"fingerprint": `SELECT f.value AS key, f.kind AS label, j.site_id FROM site_fingerprints j JOIN fingerprints f ON f.id = j.fingerprint_id WHERE f.kind <> 'referral'`,
	"referral": `SELECT f.value AS key, '' AS label, j.site_id FROM site_fingerprints j JOIN fingerprints f ON f.id = j.fingerprint_id WHERE f.kind = 'referral'`,
	"image": `SELECT i.url AS key, COALESCE(i.domain, '') AS label, j.site_id FROM site_images j JOIN images i ON i.id = j.image_id`,
}

// SharedAttributes: atribut jenis `kind` yang dipakai ≥minSites situs, diurutkan dari situs judol terbanyak.
func (db *PostgresDB) SharedAttributes(ctx context.Context, kind, q string, minSites, limit int) ([]SharedRow, error) {
	src, ok := sharedSources[kind]
	if !ok {
		return nil, fmt.Errorf("jenis tidak dikenal: %s", kind)
	}
	rows, err := db.Pool.Query(ctx, `
		WITH a AS (`+src+`)
		SELECT a.key, MAX(a.label), COUNT(DISTINCT s.id),
		       COUNT(DISTINCT s.id) FILTER (WHERE s.is_gambling),
		       COUNT(DISTINCT s.id) FILTER (WHERE s.is_gambling AND `+liveSite+`),
		       (SELECT json_agg(t) FROM (
		          SELECT s2.id, s2.url, COALESCE(s2.priority,'Low') AS priority, COALESCE(s2.confidence,0) AS confidence, COALESCE(s2.is_gambling,false) AS is_gambling
		          FROM (`+src+`) b JOIN sites s2 ON s2.id = b.site_id WHERE b.key = a.key
		          GROUP BY s2.id ORDER BY s2.is_gambling DESC, s2.confidence DESC NULLS LAST LIMIT 5) t),
		       MAX(s.last_scanned_at)
		FROM a JOIN sites s ON s.id = a.site_id
		WHERE ($1 = '' OR a.key ILIKE '%' || $1 || '%' OR a.label ILIKE '%' || $1 || '%')
		GROUP BY a.key
		HAVING COUNT(DISTINCT s.id) FILTER (WHERE s.is_gambling) >= 1 AND COUNT(DISTINCT s.id) >= $2
		ORDER BY 4 DESC, 3 DESC LIMIT $3`, q, minSites, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SharedRow{}
	for rows.Next() {
		r := SharedRow{Kind: kind}
		var samples []byte
		if err := rows.Scan(&r.Value, &r.Label, &r.Sites, &r.Judol, &r.Live, &samples, &r.LastSeen); err != nil {
			return nil, err
		}
		json.Unmarshal(samples, &r.Samples)
		if r.Samples == nil {
			r.Samples = []SiteRef{}
		}
		out = append(out, r)
	}
	if kind == "ip" || kind == "asn" {
		db.attachAbuse(ctx, kind, out)
	}
	return out, rows.Err()
}

// attachAbuse: email abuse penyedia hosting (dari RDAP) — tujuan laporan penyalahgunaan.
func (db *PostgresDB) attachAbuse(ctx context.Context, kind string, rows []SharedRow) {
	for i := range rows {
		col := "ip"
		if kind == "asn" {
			col = "data->>'asn'"
		}
		var emails []string
		db.Pool.QueryRow(ctx, `SELECT COALESCE(array_agg(DISTINCT e), '{}') FROM ip_intel, jsonb_array_elements_text(COALESCE(data->'abuse_emails','[]')) e WHERE `+col+` = $1`, rows[i].Value).Scan(&emails)
		var hosting struct {
			Country string `json:"country"`
			CDN     string `json:"cdn"`
		}
		var raw []byte
		db.Pool.QueryRow(ctx, `SELECT data FROM ip_intel WHERE `+col+` = $1 LIMIT 1`, rows[i].Value).Scan(&raw)
		json.Unmarshal(raw, &hosting)
		rows[i].Extra = map[string]any{"abuse": emails, "country": hosting.Country, "cdn": hosting.CDN}
	}
}

// ── Dompet kripto lintas situs ──

type WalletRow struct {
	Address   string          `json:"address"`
	Network   string          `json:"network"`
	Sites     int             `json:"sites"`
	Judol     int             `json:"judol"`
	Samples   []SiteRef       `json:"samples"`
	Profile   json.RawMessage `json:"profile,omitempty"`
	FetchedAt *time.Time      `json:"fetched_at,omitempty"`
}

func (db *PostgresDB) Wallets(ctx context.Context, q string, limit int) ([]WalletRow, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT x.value, COUNT(DISTINCT s.id), COUNT(DISTINCT s.id) FILTER (WHERE s.is_gambling),
		       (SELECT json_agg(t) FROM (SELECT s2.id, s2.url, COALESCE(s2.priority,'Low') AS priority, COALESCE(s2.confidence,0) AS confidence, COALESCE(s2.is_gambling,false) AS is_gambling
		          FROM site_artifacts j2 JOIN sites s2 ON s2.id = j2.site_id WHERE j2.artifact_id = x.id ORDER BY s2.is_gambling DESC, s2.confidence DESC NULLS LAST LIMIT 5) t),
		       p.profile, p.fetched_at
		FROM artifacts x JOIN site_artifacts j ON j.artifact_id = x.id JOIN sites s ON s.id = j.site_id
		LEFT JOIN crypto_profiles p ON p.address = x.value
		WHERE x.type = 'Crypto' AND ($1 = '' OR x.value ILIKE '%' || $1 || '%')
		GROUP BY x.id, x.value, p.profile, p.fetched_at
		HAVING COUNT(DISTINCT s.id) FILTER (WHERE s.is_gambling) >= 1
		ORDER BY COALESCE((p.profile->>'total_in')::float, 0) DESC, 3 DESC LIMIT $2`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WalletRow{}
	for rows.Next() {
		var w WalletRow
		var samples, profile []byte
		if err := rows.Scan(&w.Address, &w.Sites, &w.Judol, &samples, &profile, &w.FetchedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(samples, &w.Samples)
		if len(profile) > 0 {
			w.Profile = profile
		}
		switch {
		case strings.HasPrefix(w.Address, "T"):
			w.Network = "TRC20"
		case strings.HasPrefix(strings.ToLower(w.Address), "0x"):
			w.Network = "ERC20"
		case strings.HasPrefix(w.Address, "bc1"), strings.HasPrefix(w.Address, "1"), strings.HasPrefix(w.Address, "3"):
			w.Network = "BTC"
		default:
			w.Network = "Lainnya"
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ── Situs susupan ──

type InjectedRow struct {
	SiteRef
	Host      string          `json:"host"`
	Lifecycle string          `json:"lifecycle"`
	TPListed  *bool           `json:"tp_listed"`
	Injection json.RawMessage `json:"injection"`
	Verified  string          `json:"verified"`
	FoundAt   time.Time       `json:"found_at"`
}

func (db *PostgresDB) InjectedSites(ctx context.Context, q string, limit int) ([]InjectedRow, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT s.id, s.url, COALESCE(s.priority,'Low'), COALESCE(s.confidence,0), COALESCE(s.is_gambling,false),
		       site_lifecycle(s.reach_status, s.status, s.offline_since), s.tp_listed, s.injection, COALESCE(s.verified_status,''), s.created_at
		FROM sites s
		WHERE (`+siteCategorySQL+`) = 'injected' AND ($1 = '' OR s.url ILIKE '%' || $1 || '%')
		ORDER BY s.created_at DESC LIMIT $2`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InjectedRow{}
	for rows.Next() {
		var r InjectedRow
		var inj []byte
		if err := rows.Scan(&r.ID, &r.URL, &r.Priority, &r.Confidence, &r.IsGambling, &r.Lifecycle, &r.TPListed, &inj, &r.Verified, &r.FoundAt); err != nil {
			return nil, err
		}
		if len(inj) > 0 {
			r.Injection = inj
		}
		r.Host = hostOf(r.URL)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Domain respawn: situs baru di klaster operator yang sama, muncul ≤30 hari setelah situs lain diblokir/mati ──

type RespawnRow struct {
	Operator   string    `json:"operator"`
	OperatorID int       `json:"operator_id"`
	Old        SiteRef   `json:"old"`
	OldDownAt  time.Time `json:"old_down_at"`
	OldReason  string    `json:"old_reason"` // blocked | offline
	New        SiteRef   `json:"new"`
	NewAt      time.Time `json:"new_at"`
	GapDays    float64   `json:"gap_days"`
	NewLive    bool      `json:"new_live"`
}

func (db *PostgresDB) Respawns(ctx context.Context, limit int) ([]RespawnRow, error) {
	rows, err := db.Pool.Query(ctx, `
		WITH m AS (
		  SELECT o.id AS op_id, o.name, s.id, s.url, COALESCE(s.priority,'Low') AS priority, COALESCE(s.confidence,0) AS confidence,
		         COALESCE(s.is_gambling,false) AS is_gambling, s.created_at,
		         LEAST(s.blocked_since, s.offline_since) AS down_at,
		         CASE WHEN s.blocked_since IS NOT NULL AND (s.offline_since IS NULL OR s.blocked_since <= s.offline_since) THEN 'blocked' ELSE 'offline' END AS reason,
		         `+liveSite+` AS live
		  FROM operators o JOIN site_operators so ON so.operator_id = o.id JOIN sites s ON s.id = so.site_id
		  WHERE s.is_gambling)
		SELECT * FROM (
		SELECT DISTINCT ON (n.id) n.name, n.op_id, d.id, d.url, d.priority, d.confidence, d.is_gambling, d.down_at, d.reason,
		       n.id, n.url, n.priority, n.confidence, n.is_gambling, n.created_at,
		       EXTRACT(EPOCH FROM n.created_at - d.down_at) / 86400, n.live
		FROM m d JOIN m n ON n.op_id = d.op_id AND n.id <> d.id
		WHERE d.down_at IS NOT NULL AND n.created_at > d.down_at AND n.created_at < d.down_at + INTERVAL '30 days'
		ORDER BY n.id, n.created_at - d.down_at) r
		ORDER BY 15 DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RespawnRow{}
	for rows.Next() {
		var r RespawnRow
		if err := rows.Scan(&r.Operator, &r.OperatorID, &r.Old.ID, &r.Old.URL, &r.Old.Priority, &r.Old.Confidence, &r.Old.IsGambling, &r.OldDownAt, &r.OldReason,
			&r.New.ID, &r.New.URL, &r.New.Priority, &r.New.Confidence, &r.New.IsGambling, &r.NewAt, &r.GapDays, &r.NewLive); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
