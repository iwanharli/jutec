package store

import (
	"context"
	"encoding/json"
	"fmt"
	"jutec-go/internal/urlnorm"
	neturl "net/url"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// SiteRef adalah ringkasan situs yang dipakai di daftar relasi.
type SiteRef struct {
	ID         int     `json:"id"`
	URL        string  `json:"url"`
	Priority   string  `json:"priority"`
	Confidence float64 `json:"confidence"`
	IsGambling bool    `json:"is_gambling"`
}

type SiteCore struct {
	SiteRef
	Status         string         `json:"status"`
	SimHash        string         `json:"simhash"`
	RescanCount    int            `json:"rescan_count"`
	CreatedAt      time.Time      `json:"created_at"`
	LastScannedAt  *time.Time     `json:"last_scanned_at"`
	OfflineSince   *time.Time     `json:"offline_since"`
	InternalPages  []string       `json:"internal_pages"`
	AuthSurface    map[string]any `json:"auth_surface"`
	Injection      map[string]any `json:"injection"` // crawler.Injection: tanda situs sah yang disusupi
	Takeover       map[string]any `json:"takeover"` // wayback.Takeover: domain bekas situs sah
	TPListed       *bool          `json:"tp_listed"` // terdaftar di daftar blokir TrustPositif (nil = belum dicek)
	TPListedSince  *time.Time     `json:"tp_listed_since"`
	TPCheckedAt    *time.Time     `json:"tp_checked_at"`
	WidgetLinks    []string       `json:"widget_links"`
	AppPromo       bool           `json:"app_promo"`
	AppPromoEvid   []string       `json:"app_promo_evidence"`
	AppPromoPlat   []string       `json:"app_promo_platforms"`
	VerifiedStatus *string        `json:"verified_status"` // judol | bukan_judol | null (belum diverifikasi)
	VerifiedBy     *string        `json:"verified_by"`
	VerifiedAt     *time.Time     `json:"verified_at"`
	VerifiedNote   *string        `json:"verified_note"`
	ReachStatus    *string        `json:"reach_status"` // online | blocked_alive | blocked | offline | nxdomain
	ReachCheckedAt *time.Time     `json:"reach_checked_at"`
	BlockedSince   *time.Time     `json:"blocked_since"`
	Lifecycle      string         `json:"lifecycle"` // active | blocked_alive | uncertain | inactive (lihat site_lifecycle)
	Role           *string        `json:"role"`      // operator (situs utama) | landing (pintu masuk) | null
	Category       string         `json:"category"`  // kategori yang sama dengan kolom "Jenis" di tabel (lihat siteCategorySQL)
	EntryTargets   []string       `json:"entry_targets"`
	RevivedAt      *time.Time     `json:"revived_at"`
}

// SaveAppPromo menyimpan hasil deteksi promosi aplikasi (dipanggil tiap scan agar selalu terkini).
func (db *PostgresDB) SaveAppPromo(ctx context.Context, siteID int, detected bool, evidence, platforms []string) error {
	if evidence == nil {
		evidence = []string{}
	}
	if platforms == nil {
		platforms = []string{}
	}
	_, err := db.Pool.Exec(ctx,
		`UPDATE sites SET app_promo = $2, app_promo_evidence = $3, app_promo_platforms = $4 WHERE id = $1`,
		siteID, detected, evidence, platforms)
	return err
}

type SiteWHOIS struct {
	Domain        string     `json:"domain"`
	Registrar     string     `json:"registrar"`
	Registrant    string     `json:"registrant"`
	Email         string     `json:"email"`
	Org           string     `json:"org"`
	RegisteredAt  *time.Time `json:"registered_at"`
	ExpiresAt     *time.Time `json:"expires_at"`
	DomainAgeDays int        `json:"domain_age_days"`
	IsPrivate     bool       `json:"is_private"`
	TLD           string     `json:"tld"`
	RiskScore     float64    `json:"risk_score"`
}

// SiteAttr adalah satu atribut milik situs (IP, NS, artefak, dll) + jumlah situs lain yang ikut memakainya.
type SiteAttr struct {
	ID          int    `json:"id"`
	Kind        string `json:"kind"`  // ip, ns, ssl, txt, tracking, artifact, link, app, operator
	Group       string `json:"group"` // sub-tipe: platform tracking, tipe artefak, tipe link, dst.
	Value       string `json:"value"`
	Label       string `json:"label,omitempty"`
	SharedSites int    `json:"shared_sites"` // jumlah situs LAIN yang memakai atribut ini
	// Khusus artefak: situs lain yang memakai artefak ini dan sudah diverifikasi manual sebagai judol
	VerifiedSites []SiteRef `json:"verified_sites,omitempty"`
}

type SiteScan struct {
	ID        int       `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"` // ok | error (situs tidak bisa diakses)
	Score     *float64  `json:"score"`
	Priority  string    `json:"priority"`
}

type SiteEnrichment struct {
	URLScan    map[string]any `json:"urlscan,omitempty"`
	VirusTotal map[string]any `json:"virustotal,omitempty"`
}

type SiteDetail struct {
	Site          SiteCore       `json:"site"`
	WHOIS         *SiteWHOIS     `json:"whois"`
	Attributes    []SiteAttr     `json:"attributes"`
	Scans         []SiteScan     `json:"scans"`
	Enrichment    SiteEnrichment `json:"enrichment"`
	ImageCount    int            `json:"image_count"`
	GameProviders []string       `json:"game_providers"` // provider game (Pragmatic, PG Soft, …) dari scan terakhir
	ScoreExplain  *ScoreExplain  `json:"score_explain"`  // dasar skor dari scan lengkap terakhir (null bila belum pernah)
	RoleLinks     RoleLinks      `json:"role_links"`     // landing page ↔ situs utama
}

// ScoreExplain menjelaskan dari mana skor situs berasal: kontribusi tiap sinyal, kata kunci & pola yang cocok,
// dan nilai artefak yang terbaca di teks halaman (pembeda dengan artefak hasil OCR/QRIS atau scan lama).
type ScoreExplain struct {
	ScannedAt      time.Time      `json:"scanned_at"`
	Score          map[string]any `json:"score"` // final, priority, rule, breakdown
	Keywords       []string       `json:"keywords"`
	Patterns       []string       `json:"patterns"`
	ArtifactValues []string       `json:"artifact_values"`
	Redirects      []string       `json:"redirects"`
	MobileCrawled  bool           `json:"mobile_crawled"`
}

// attrQueries: setiap baris → (attr id, group, value, label, jumlah situs lain yang berbagi atribut).
// Tabel junction & atribut berbeda per jenis, jadi tiap jenis punya query sendiri.
var attrQueries = map[string]string{
	"ip": `SELECT x.id, COALESCE(x.subnet_24, ''), x.address, '',
	         (SELECT COUNT(*) FROM site_ips o WHERE o.ip_id = x.id AND o.site_id <> $1)
	       FROM site_ips j JOIN ips x ON x.id = j.ip_id WHERE j.site_id = $1`,
	"ns": `SELECT x.id, '', x.host, '',
	         (SELECT COUNT(*) FROM site_ns o WHERE o.ns_id = x.id AND o.site_id <> $1)
	       FROM site_ns j JOIN nameservers x ON x.id = j.ns_id WHERE j.site_id = $1`,
	"ssl": `SELECT x.id, '', x.thumbprint, '',
	          (SELECT COUNT(*) FROM site_ssl o WHERE o.cert_id = x.id AND o.site_id <> $1)
	        FROM site_ssl j JOIN ssl_certs x ON x.id = j.cert_id WHERE j.site_id = $1`,
	"txt": `SELECT x.id, x.service, x.token, '',
	          (SELECT COUNT(*) FROM site_txt o WHERE o.token_id = x.id AND o.site_id <> $1)
	        FROM site_txt j JOIN txt_tokens x ON x.id = j.token_id WHERE j.site_id = $1`,
	"tracking": `SELECT x.id, x.platform, x.value, '',
	               (SELECT COUNT(*) FROM site_tracking o WHERE o.tracking_id = x.id AND o.site_id <> $1)
	             FROM site_tracking j JOIN tracking_ids x ON x.id = j.tracking_id WHERE j.site_id = $1`,
	"artifact": `SELECT x.id, x.type, x.value, COALESCE(x.name, ''),
	               (SELECT COUNT(*) FROM site_artifacts o WHERE o.artifact_id = x.id AND o.site_id <> $1)
	             FROM site_artifacts j JOIN artifacts x ON x.id = j.artifact_id WHERE j.site_id = $1`,
	// Link dikelompokkan per DOMAIN (satu domain bisa punya puluhan URL). Label = jumlah URL.
	// Link internal & domain di blacklist (CDN, sosmed, dll) tidak relevan untuk keterkaitan.
	"link": `SELECT MIN(x.id), string_agg(DISTINCT j.link_type, ','), x.domain, COUNT(DISTINCT x.url)::text,
	           (SELECT COUNT(DISTINCT o.site_id) FROM site_links o JOIN links l ON l.id = o.link_id
	            WHERE l.domain = x.domain AND o.site_id <> $1)
	         FROM site_links j JOIN links x ON x.id = j.link_id
	         WHERE j.site_id = $1 AND j.link_type <> 'internal' AND COALESCE(x.domain, '') <> ''
	           AND NOT EXISTS (SELECT 1 FROM domain_blacklist b
	                           WHERE b.is_active AND (x.domain = b.domain OR x.domain LIKE '%.' || b.domain))
	         GROUP BY x.domain`,
	"app": `SELECT x.id, COALESCE(x.platform, ''), x.url, COALESCE(x.filename, ''),
	          (SELECT COUNT(*) FROM site_apps o WHERE o.app_id = x.id AND o.site_id <> $1)
	        FROM site_apps j JOIN apps x ON x.id = j.app_id WHERE j.site_id = $1`,
	"fingerprint": `SELECT x.id, x.kind, x.value, '',
	                  (SELECT COUNT(*) FROM site_fingerprints o WHERE o.fingerprint_id = x.id AND o.site_id <> $1)
	                FROM site_fingerprints j JOIN fingerprints x ON x.id = j.fingerprint_id WHERE j.site_id = $1`,
	"operator": `SELECT x.id, x.platform, x.operator_key, COALESCE(x.name, ''),
	               (SELECT COUNT(*) FROM site_operators o WHERE o.operator_id = x.id AND o.site_id <> $1)
	             FROM site_operators j JOIN operators x ON x.id = j.operator_id WHERE j.site_id = $1`,
}

// relatedQueries: situs lain yang berbagi atribut tertentu (attr id = $1, site pusat = $2).
var relatedQueries = map[string]string{
	"ip":       `FROM site_ips o JOIN sites s ON s.id = o.site_id WHERE o.ip_id = $1 AND o.site_id <> $2`,
	"ns":       `FROM site_ns o JOIN sites s ON s.id = o.site_id WHERE o.ns_id = $1 AND o.site_id <> $2`,
	"ssl":      `FROM site_ssl o JOIN sites s ON s.id = o.site_id WHERE o.cert_id = $1 AND o.site_id <> $2`,
	"txt":      `FROM site_txt o JOIN sites s ON s.id = o.site_id WHERE o.token_id = $1 AND o.site_id <> $2`,
	"tracking": `FROM site_tracking o JOIN sites s ON s.id = o.site_id WHERE o.tracking_id = $1 AND o.site_id <> $2`,
	"artifact": `FROM site_artifacts o JOIN sites s ON s.id = o.site_id WHERE o.artifact_id = $1 AND o.site_id <> $2`,
	"link": `FROM site_links o JOIN links l ON l.id = o.link_id JOIN sites s ON s.id = o.site_id
	         WHERE l.domain = (SELECT domain FROM links WHERE id = $1) AND o.site_id <> $2`,
	"app":         `FROM site_apps o JOIN sites s ON s.id = o.site_id WHERE o.app_id = $1 AND o.site_id <> $2`,
	"operator":    `FROM site_operators o JOIN sites s ON s.id = o.site_id WHERE o.operator_id = $1 AND o.site_id <> $2`,
	"fingerprint": `FROM site_fingerprints o JOIN sites s ON s.id = o.site_id WHERE o.fingerprint_id = $1 AND o.site_id <> $2`,
}

// attrKindOrder menentukan urutan tampilan atribut (paling kuat sebagai bukti operator duluan).
var attrKindOrder = []string{"operator", "artifact", "fingerprint", "tracking", "ssl", "app", "link", "ip", "ns", "txt"}

// GetSiteIDByURL mencari id situs dari URL persis, fallback ke pencocokan host.
func (db *PostgresDB) GetSiteIDByURL(ctx context.Context, url string) (int, error) {
	var id int
	// Cocokkan dengan/tanpa "/" di akhir; bila ada duplikat lama, ambil yang paling baru di-scan
	trimmed := strings.TrimRight(url, "/")
	err := db.Pool.QueryRow(ctx, `
		SELECT id FROM sites WHERE url IN ($1, $2, $2 || '/')
		ORDER BY last_scanned_at DESC NULLS LAST LIMIT 1`, url, trimmed).Scan(&id)
	if err == pgx.ErrNoRows {
		err = db.Pool.QueryRow(ctx, `
			SELECT id FROM sites WHERE url ILIKE '%' || $1 || '%'
			ORDER BY last_scanned_at DESC NULLS LAST LIMIT 1`, trimmed).Scan(&id)
	}
	return id, err
}

func (db *PostgresDB) getSiteAttributes(ctx context.Context, siteID int) ([]SiteAttr, error) {
	var attrs []SiteAttr
	for _, kind := range attrKindOrder {
		rows, err := db.Pool.Query(ctx, attrQueries[kind], siteID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			a := SiteAttr{Kind: kind}
			if err := rows.Scan(&a.ID, &a.Group, &a.Value, &a.Label, &a.SharedSites); err != nil {
				rows.Close()
				return nil, err
			}
			attrs = append(attrs, a)
		}
		rows.Close()
	}
	return attrs, nil
}

// attachVerifiedSites menandai rekening/kontak yang juga dipakai situs lain berstatus terverifikasi judol —
// sinyal terkuat bahwa artefak ini milik jaringan judol, karena sudah dikonfirmasi manusia, bukan hanya skor otomatis.
func (db *PostgresDB) attachVerifiedSites(ctx context.Context, siteID int, attrs []SiteAttr) error {
	idx := map[int]int{}
	var ids []int
	for i, a := range attrs {
		if a.Kind == "artifact" && a.SharedSites > 0 {
			idx[a.ID] = i
			ids = append(ids, a.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT o.artifact_id, s.id, s.url, COALESCE(s.priority, 'Low'), COALESCE(s.confidence, 0), true
		FROM site_artifacts o JOIN sites s ON s.id = o.site_id
		WHERE o.artifact_id = ANY($1) AND o.site_id <> $2 AND s.verified_status = 'judol'
		ORDER BY s.verified_at DESC NULLS LAST`, ids, siteID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var artID int
		var r SiteRef
		if err := rows.Scan(&artID, &r.ID, &r.URL, &r.Priority, &r.Confidence, &r.IsGambling); err != nil {
			return err
		}
		a := &attrs[idx[artID]]
		a.VerifiedSites = append(a.VerifiedSites, r)
	}
	return rows.Err()
}

// GetSiteDetail mengumpulkan seluruh data yang diketahui tentang satu situs.
func (db *PostgresDB) GetSiteDetail(ctx context.Context, siteID int) (*SiteDetail, error) {
	d := &SiteDetail{}
	s := &d.Site
	err := db.Pool.QueryRow(ctx, `
		SELECT id, url, COALESCE(priority, 'Low'), COALESCE(confidence, 0), COALESCE(is_gambling, false),
		       COALESCE(status, ''), COALESCE(simhash, ''), COALESCE(rescan_count, 0), created_at, last_scanned_at, offline_since,
		       app_promo, app_promo_evidence, app_promo_platforms, internal_pages, auth_surface, widget_links,
		       verified_status, verified_by, verified_at, verified_note,
		       reach_status, reach_checked_at, blocked_since,
		       site_lifecycle(reach_status, status, offline_since), revived_at, role, entry_targets, `+siteCategorySQL+`, injection, takeover, tp_listed, tp_listed_since, tp_checked_at
		FROM sites s WHERE id = $1`, siteID).Scan(
		&s.ID, &s.URL, &s.Priority, &s.Confidence, &s.IsGambling,
		&s.Status, &s.SimHash, &s.RescanCount, &s.CreatedAt, &s.LastScannedAt, &s.OfflineSince,
		&s.AppPromo, &s.AppPromoEvid, &s.AppPromoPlat, &s.InternalPages, &s.AuthSurface, &s.WidgetLinks,
		&s.VerifiedStatus, &s.VerifiedBy, &s.VerifiedAt, &s.VerifiedNote,
		&s.ReachStatus, &s.ReachCheckedAt, &s.BlockedSince,
		&s.Lifecycle, &s.RevivedAt, &s.Role, &s.EntryTargets, &s.Category, &s.Injection, &s.Takeover, &s.TPListed, &s.TPListedSince, &s.TPCheckedAt)
	if err != nil {
		return nil, err
	}

	var w SiteWHOIS
	err = db.Pool.QueryRow(ctx, `
		SELECT COALESCE(domain, ''), COALESCE(registrar, ''), COALESCE(registrant_name, ''), COALESCE(registrant_email, ''),
		       COALESCE(registrant_org, ''), registered_at, expires_at, COALESCE(domain_age_days, 0),
		       COALESCE(is_private, false), COALESCE(tld, ''), COALESCE(risk_score, 0)
		FROM whois_records WHERE site_id = $1 ORDER BY scanned_at DESC NULLS LAST LIMIT 1`, siteID).Scan(
		&w.Domain, &w.Registrar, &w.Registrant, &w.Email, &w.Org, &w.RegisteredAt, &w.ExpiresAt,
		&w.DomainAgeDays, &w.IsPrivate, &w.TLD, &w.RiskScore)
	if err == nil {
		d.WHOIS = &w
	} else if err != pgx.ErrNoRows {
		return nil, err
	}

	if d.Attributes, err = db.getSiteAttributes(ctx, siteID); err != nil {
		return nil, err
	}
	if err = db.attachVerifiedSites(ctx, siteID, d.Attributes); err != nil {
		return nil, err
	}

	rows, err := db.Pool.Query(ctx, `
		SELECT id, created_at,
		       CASE WHEN result ? 'error' THEN 'error' ELSE 'ok' END,
		       (result->'score'->>'final')::float8,
		       COALESCE(result->'score'->>'priority', '')
		FROM scan_logs WHERE site_id = $1 ORDER BY created_at DESC LIMIT 20`, siteID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sc SiteScan
		if err := rows.Scan(&sc.ID, &sc.CreatedAt, &sc.Status, &sc.Score, &sc.Priority); err != nil {
			rows.Close()
			return nil, err
		}
		d.Scans = append(d.Scans, sc)
	}
	rows.Close()

	var us struct {
		Verdict, Screenshot string
		Malicious           bool
		Score               int
		ScannedAt           *time.Time
		Source              string
	}
	if db.Pool.QueryRow(ctx, `
		SELECT COALESCE(verdict, ''), COALESCE(malicious, false), COALESCE(score, 0), COALESCE(screenshot_url, ''), scanned_at,
		       CASE WHEN scan_uuid LIKE 'cf:%' THEN 'Cloudflare' ELSE 'URLScan' END
		FROM urlscan_results WHERE site_id = $1 ORDER BY created_at DESC LIMIT 1`, siteID).
		Scan(&us.Verdict, &us.Malicious, &us.Score, &us.Screenshot, &us.ScannedAt, &us.Source) == nil {
		d.Enrichment.URLScan = map[string]any{
			"verdict": us.Verdict, "malicious": us.Malicious, "score": us.Score,
			"screenshot_url": us.Screenshot, "scanned_at": us.ScannedAt, "source": us.Source,
		}
	}
	var vt struct {
		Malicious, Suspicious, Harmless, Total int
		Permalink                              string
	}
	if db.Pool.QueryRow(ctx, `
		SELECT COALESCE(malicious_count, 0), COALESCE(suspicious_count, 0), COALESCE(harmless_count, 0),
		       COALESCE(total_engines, 0), COALESCE(permalink, '')
		FROM virustotal_results WHERE site_id = $1 ORDER BY created_at DESC LIMIT 1`, siteID).
		Scan(&vt.Malicious, &vt.Suspicious, &vt.Harmless, &vt.Total, &vt.Permalink) == nil {
		d.Enrichment.VirusTotal = map[string]any{
			"malicious": vt.Malicious, "suspicious": vt.Suspicious, "harmless": vt.Harmless,
			"total": vt.Total, "permalink": vt.Permalink,
		}
	}

	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM site_images WHERE site_id = $1`, siteID).Scan(&d.ImageCount)

	// Provider game dari scan lengkap terakhir (hanya tersimpan di payload scan_logs)
	d.GameProviders = []string{}
	var gpRaw []byte
	if db.Pool.QueryRow(ctx, `
		SELECT result->'result'->'network_signals'->'game_providers'
		FROM scan_logs WHERE site_id = $1 AND NOT (result ? 'error') AND result ? 'result'
		  AND (result->'result'->'network_signals'->'game_providers') IS NOT NULL
		ORDER BY created_at DESC LIMIT 1`, siteID).Scan(&gpRaw) == nil && len(gpRaw) > 0 {
		var gp []string
		if json.Unmarshal(gpRaw, &gp) == nil {
			seen := map[string]bool{}
			for _, g := range gp {
				if g = strings.TrimSpace(g); g != "" && !seen[g] {
					seen[g] = true
					d.GameProviders = append(d.GameProviders, g)
				}
			}
		}
	}

	var ex ScoreExplain
	var scoreRaw, kwRaw, patRaw, artRaw, redRaw []byte
	if db.Pool.QueryRow(ctx, `
		SELECT created_at, result->'score', result->'result'->'found_keywords', result->'result'->'found_patterns',
		       (SELECT jsonb_agg(a->>'value') FROM jsonb_array_elements(
		          CASE WHEN jsonb_typeof(result->'result'->'artifacts') = 'array' THEN result->'result'->'artifacts' ELSE '[]' END) a),
		       result->'result'->'redirect_chain', COALESCE((result->'result'->>'mobile_crawled')::bool, false)
		FROM scan_logs WHERE site_id = $1 AND NOT (result ? 'error') AND result ? 'score'
		ORDER BY created_at DESC LIMIT 1`, siteID).Scan(&ex.ScannedAt, &scoreRaw, &kwRaw, &patRaw, &artRaw, &redRaw, &ex.MobileCrawled) == nil {
		json.Unmarshal(scoreRaw, &ex.Score)
		json.Unmarshal(kwRaw, &ex.Keywords)
		json.Unmarshal(patRaw, &ex.Patterns)
		json.Unmarshal(artRaw, &ex.ArtifactValues)
		json.Unmarshal(redRaw, &ex.Redirects)
		for _, p := range []*[]string{&ex.Keywords, &ex.Patterns, &ex.ArtifactValues, &ex.Redirects} {
			if *p == nil {
				*p = []string{}
			}
		}
		d.ScoreExplain = &ex
	}

	d.RoleLinks = db.siteRoleLinks(ctx, siteID, s.URL, s.EntryTargets)

	// Situs unreachable tak punya atribut — kirim [] bukan null agar klien tak perlu cek null
	if d.Attributes == nil {
		d.Attributes = []SiteAttr{}
	}
	if d.Scans == nil {
		d.Scans = []SiteScan{}
	}
	return d, nil
}

// ── Relations (graph / tree) ─────────────────────────────────────────

type RelationAttr struct {
	SiteAttr
	Common bool      `json:"common"` // dipakai terlalu banyak situs → tidak diperluas & tidak dihitung
	Sites  []SiteRef `json:"sites"`  // dibatasi `perAttr`; total ada di SharedSites
}

// kindWeight: seberapa kuat kesamaan atribut menunjukkan operator yang sama.
var kindWeight = map[string]float64{
	"operator": 5, "artifact": 4, "fingerprint": 4, "tracking": 4, "ssl": 3, "app": 3, "txt": 2, "link": 1, "ip": 1, "ns": 0.5,
}

type RelatedSite struct {
	SiteRef
	SharedCount int      `json:"shared_count"` // jumlah atribut yang sama dengan situs pusat
	SharedKinds []string `json:"shared_kinds"` // jenis atribut yang sama (unik)
	Strength    float64  `json:"strength"`     // skor keterkaitan berbobot (lihat kindWeight)
	Category    string   `json:"category"`     // lihat siteCategorySQL
	Lifecycle   string   `json:"lifecycle"`
	Verified    string   `json:"verified"`     // judol | bukan_judol | "" (belum diverifikasi analis)
}

type SiteRelations struct {
	Site       SiteRef        `json:"site"`
	Attributes []RelationAttr `json:"attributes"`
	Related    []RelatedSite  `json:"related"` // diurutkan: paling banyak berbagi atribut duluan
}

// GetSiteRelations membangun pohon keterkaitan: situs → atribut → situs lain yang berbagi atribut itu.
// perAttr membatasi jumlah situs per atribut agar atribut "umum" (mis. NS Cloudflare) tidak membanjiri graf.
// maxShared: atribut yang dipakai lebih dari ini dianggap umum (CDN/NS publik) dan tidak diperluas.
func (db *PostgresDB) GetSiteRelations(ctx context.Context, siteID, perAttr, maxShared int) (*SiteRelations, error) {
	rel := &SiteRelations{}
	err := db.Pool.QueryRow(ctx, `
		SELECT id, url, COALESCE(priority, 'Low'), COALESCE(confidence, 0), COALESCE(is_gambling, false)
		FROM sites WHERE id = $1`, siteID).Scan(
		&rel.Site.ID, &rel.Site.URL, &rel.Site.Priority, &rel.Site.Confidence, &rel.Site.IsGambling)
	if err != nil {
		return nil, err
	}

	attrs, err := db.getSiteAttributes(ctx, siteID)
	if err != nil {
		return nil, err
	}

	selfHost := hostOf(rel.Site.URL)
	related := map[int]*RelatedSite{}
	kindSeen := map[int]map[string]bool{}
	for _, a := range attrs {
		ra := RelationAttr{SiteAttr: a, Sites: []SiteRef{}, Common: a.SharedSites > maxShared}
		if a.SharedSites > 0 && !ra.Common {
			// Judol & confidence tinggi didahulukan
			rows, err := db.Pool.Query(ctx, `
				SELECT DISTINCT s.id, s.url, COALESCE(s.priority, 'Low'), COALESCE(s.confidence, 0), COALESCE(s.is_gambling, false) `+
				relatedQueries[a.Kind]+`
				ORDER BY 5 DESC, 4 DESC LIMIT $3`, a.ID, siteID, perAttr)
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				var s SiteRef
				if err := rows.Scan(&s.ID, &s.URL, &s.Priority, &s.Confidence, &s.IsGambling); err != nil {
					rows.Close()
					return nil, err
				}
				// Baris lain untuk host yang sama (mis. beda "/" di akhir) bukan situs "terkait"
				if hostOf(s.URL) == selfHost {
					continue
				}
				ra.Sites = append(ra.Sites, s)
				r := related[s.ID]
				if r == nil {
					r = &RelatedSite{SiteRef: s}
					related[s.ID] = r
					kindSeen[s.ID] = map[string]bool{}
				}
				r.SharedCount++
				r.Strength += kindWeight[a.Kind]
				if !kindSeen[s.ID][a.Kind] {
					kindSeen[s.ID][a.Kind] = true
					r.SharedKinds = append(r.SharedKinds, a.Kind)
				}
			}
			rows.Close()
		}
		rel.Attributes = append(rel.Attributes, ra)
	}

	if len(related) > 0 {
		ids := make([]int, 0, len(related))
		for id := range related {
			ids = append(ids, id)
		}
		rows, err := db.Pool.Query(ctx, `
			SELECT s.id, `+siteCategorySQL+`, site_lifecycle(s.reach_status, s.status, s.offline_since),
			       COALESCE(s.verified_status, '')
			FROM sites s WHERE s.id = ANY($1)`, ids)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int
			var cat, life string
			var ver string
			if err := rows.Scan(&id, &cat, &life, &ver); err != nil {
				rows.Close()
				return nil, err
			}
			related[id].Category, related[id].Lifecycle, related[id].Verified = cat, life, ver
		}
		rows.Close()
	}
	for _, r := range related {
		rel.Related = append(rel.Related, *r)
	}
	sort.Slice(rel.Related, func(i, j int) bool {
		a, b := rel.Related[i], rel.Related[j]
		if a.Strength != b.Strength {
			return a.Strength > b.Strength
		}
		return a.Confidence > b.Confidence
	})
	if rel.Attributes == nil {
		rel.Attributes = []RelationAttr{}
	}
	if rel.Related == nil {
		rel.Related = []RelatedSite{}
	}
	return rel, nil
}

func hostOf(raw string) string {
	u, err := neturl.Parse(raw)
	if err != nil {
		return raw
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// ScannedWithin: true bila URL (dengan/tanpa "/" akhir) sudah di-scan dalam rentang waktu d.
func (db *PostgresDB) ScannedWithin(ctx context.Context, url string, d time.Duration) bool {
	var n int
	if c, err := urlnorm.Canonical(url); err == nil {
		url = c
	}
	db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM sites WHERE lower(rtrim(url, '/')) = $1 AND last_scanned_at > NOW() - $2::interval`,
		urlnorm.Key(url), fmt.Sprintf("%d seconds", int(d.Seconds()))).Scan(&n)
	return n > 0
}
