package store

import (
	"context"
	"net/url"
	"strings"
	"time"
)

// noiseDomains adalah domain yang tidak relevan untuk intelijen judol —
// CDN, analytics, image host, ad network. Tidak disimpan ke tabel links.
var noiseDomains = map[string]bool{
	// CDN / image host
	"pinimg.com": true, "cloudfront.net": true, "gstatic.com": true,
	"googleusercontent.com": true, "fbcdn.net": true, "akamaized.net": true,
	"fastly.net": true, "cdn.jsdelivr.net": true, "unpkg.com": true,
	"cdnjs.cloudflare.com": true, "bootstrapcdn.com": true,
	"imgur.com": true, "giphy.com": true, "wp.com": true,
	// Ad networks & trackers
	"doubleclick.net": true, "googleadservices.com": true, "googlesyndication.com": true,
	"google-analytics.com": true, "googletagmanager.com": true,
	"hotjar.com": true, "clarity.ms": true, "segment.io": true,
	"kl-youniverse.com": true, "adserver.online": true, "adnxs.com": true,
	"adsrvr.org": true, "criteo.com": true, "taboola.com": true,
	"outbrain.com": true, "sharethrough.com": true, "openx.net": true,
	"pubmatic.com": true, "rubiconproject.com": true, "smartadserver.com": true,
	"onetrust.com": true, "cookielaw.org": true, "cookiebot.com": true,
	// Platform besar (bukan target judol)
	"google.com": true, "youtube.com": true, "facebook.com": true,
	"instagram.com": true, "twitter.com": true, "x.com": true,
	"tiktok.com": true, "linkedin.com": true, "reddit.com": true,
	"github.com": true, "wikipedia.org": true, "w3.org": true,
	"schema.org": true, "gmpgo.org": true, "cloudflare.com": true,
	"whatsapp.com": true, "t.me": true, "telegram.org": true,
	"apple.com": true, "microsoft.com": true, "amazon.com": true,
	"shopee.co.id": true, "tokopedia.com": true, "bukalapak.com": true,
	"gopay.co.id": true, "ovo.id": true, "dana.id": true,
	// Media / news
	"detik.com": true, "kompas.com": true, "cnnindonesia.com": true,
	"tribunnews.com": true, "liputan6.com": true, "okezone.com": true,
	"kompasiana.com": true, "merdeka.com": true, "suara.com": true,
	"dw.com": true, "bbc.com": true, "reuters.com": true,
	"theguardian.com": true, "forbes.com": true,
	// Aggregator gambling (berguna di blacklist UI, tidak perlu disimpan sebagai link)
	"betimate.com": true, "flashscore.co.id": true, "flashscore.com": true,
	"slotcatalog.com": true, "livescore.in": true, "wizardofodds.com": true,
	"casino.guru": true, "chipy.com": true, "askgamblers.com": true,
	// Domain infra / legit yang sering muncul sebagai false positive
	"recaptcha.net": true, "hcaptcha.com": true, "jsdelivr.net": true,
	// Skrip/beacon pihak ketiga yang dipakai jutaan situs — bukan bukti keterkaitan operator
	"cloudflareinsights.com": true, "w.org": true, "withgoogle.com": true,
	"jquery.com": true, "fontawesome.com": true,
	// Image CDN tanpa ekstensi (hash path)
	"utdstc.com": true, "impactcdn.com": true, "s.ntv.io": true,
	"ntv.io": true, "hubvisor.io": true,
	// App store/catalog — link APK disimpan di tabel apps, bukan links
	"uptodown.com": true, "apk.cafe": true, "apkmirror.com": true,
	"apkmonk.com": true, "apkcombo.com": true,
	// Platform QnA / forum besar
	"quora.com": true, "dentsu.com": true,
	"pages.dev": false, // cloudflare pages — judol sering pakai ini, jangan filter
}

// IsNoiseDomain: domain infrastruktur/platform umum yang bukan bukti keterkaitan (dipakai juga oleh sub-crawl).
func IsNoiseDomain(domain string) bool { return isNoiseDomain(domain) }

func isNoiseDomain(domain string) bool {
	if domain == "" {
		return true
	}
	// cek exact match
	if noiseDomains[domain] {
		return true
	}
	// cek suffix (misal: s1-c.pinimg.com → pinimg.com)
	parts := strings.SplitN(domain, ".", 2)
	if len(parts) == 2 && noiseDomains[parts[1]] {
		return true
	}
	return false
}

type Link struct {
	ID        int       `json:"id"`
	URL       string    `json:"url"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"created_at"`
}

type LinkWithSites struct {
	Link
	LinkType  string   `json:"link_type"`
	SiteCount int      `json:"site_count"`
	Sites     []string `json:"sites"`
	ActiveCount int  `json:"active_site_count"` // situs pemakai yang masih beroperasi
	// Dipakai situs yang sudah mati DAN baru muncul (≤30 hari) di situs aktif — operator pindah domain
	Reappeared bool `json:"reappeared"`
}

// SaveLinks menyimpan semua link dari satu situs ke tabel links + site_links.
// Fully idempotent: link yang sudah ada tidak duplikat, relasi yang sudah ada di-skip.
func (db *PostgresDB) SaveLinks(ctx context.Context, siteID int, links map[string][]string) error {
	// links: { "internal": [...], "external": [...], "iframe": [...] }
	for linkType, urls := range links {
		for _, rawURL := range urls {
			rawURL = strings.TrimSpace(rawURL)
			if rawURL == "" || len(rawURL) > 300 {
				continue
			}

			domain := extractDomain(rawURL)
			if isNoiseDomain(domain) {
				continue
			}

			// Upsert link — deduplication by URL
			var linkID int
			err := db.Pool.QueryRow(ctx, `
				INSERT INTO links (url, domain)
				VALUES ($1, $2)
				ON CONFLICT (url) DO UPDATE SET domain = EXCLUDED.domain
				RETURNING id
			`, rawURL, domain).Scan(&linkID)
			if err != nil {
				continue // skip invalid entries, jangan hentikan proses
			}

			// Relasi situs ↔ link (ignore jika sudah ada)
			db.Pool.Exec(ctx, `
				INSERT INTO site_links (site_id, link_id, link_type)
				VALUES ($1, $2, $3)
				ON CONFLICT (site_id, link_id) DO NOTHING
			`, siteID, linkID, linkType)
		}
	}
	return nil
}

// GetSitesByLink mencari semua situs yang mengandung URL tertentu.
func (db *PostgresDB) GetSitesByLink(ctx context.Context, linkURL string) ([]LinkWithSites, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT l.id, l.url, l.domain, l.created_at,
		       sl.link_type,
		       COUNT(DISTINCT sl.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites,
		       COUNT(DISTINCT sl.site_id) FILTER (WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive') AS active_count,
		       (BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) = 'inactive') AND BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive' AND sl.first_seen > NOW() - INTERVAL '30 days')) AS reappeared
		FROM links l
		JOIN site_links sl ON sl.link_id = l.id
		JOIN sites s ON s.id = sl.site_id
		WHERE l.url = $1
		GROUP BY l.id, sl.link_type
	`, linkURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LinkWithSites
	for rows.Next() {
		var r LinkWithSites
		if err := rows.Scan(&r.ID, &r.URL, &r.Domain, &r.CreatedAt, &r.LinkType, &r.SiteCount, &r.Sites, &r.ActiveCount, &r.Reappeared); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetSitesByDomain mencari semua situs yang punya link ke domain tertentu.
func (db *PostgresDB) GetSitesByDomain(ctx context.Context, domain string) ([]LinkWithSites, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT l.id, l.url, l.domain, l.created_at,
		       sl.link_type,
		       COUNT(DISTINCT sl.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites,
		       COUNT(DISTINCT sl.site_id) FILTER (WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive') AS active_count,
		       (BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) = 'inactive') AND BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive' AND sl.first_seen > NOW() - INTERVAL '30 days')) AS reappeared
		FROM links l
		JOIN site_links sl ON sl.link_id = l.id
		JOIN sites s ON s.id = sl.site_id
		WHERE l.domain = $1
		GROUP BY l.id, sl.link_type
		ORDER BY site_count DESC
	`, domain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LinkWithSites
	for rows.Next() {
		var r LinkWithSites
		if err := rows.Scan(&r.ID, &r.URL, &r.Domain, &r.CreatedAt, &r.LinkType, &r.SiteCount, &r.Sites, &r.ActiveCount, &r.Reappeared); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetTopExternalDomains mengembalikan domain eksternal yang paling banyak
// dirujuk oleh situs judol — berguna untuk menemukan jaringan iklan atau afiliasi.
func (db *PostgresDB) GetTopExternalDomains(ctx context.Context, limit int) ([]LinkWithSites, error) {
	results, _, err := db.GetExternalDomainsPaged(ctx, "", "", limit, 0, true)
	return results, err
}

// Item yang semua situs pemakainya sudah tidak aktif disembunyikan kecuali includeInactive.
func (db *PostgresDB) GetExternalDomainsPaged(ctx context.Context, q, linkType string, limit, offset int, includeInactive bool) ([]LinkWithSites, int, error) {
	var total int
	countSQL := `
		SELECT COUNT(*) FROM (
			SELECT l.id
			FROM links l
			JOIN site_links sl ON sl.link_id = l.id
			JOIN sites s ON s.id = sl.site_id
			WHERE sl.link_type IN ('external', 'iframe', 'register')
			  AND s.is_gambling = TRUE
			  AND ($1 = '' OR sl.link_type = $1)
			  AND ($2 = '' OR l.domain ILIKE $2 OR l.url ILIKE $2)
			GROUP BY l.id, sl.link_type
			HAVING COUNT(DISTINCT sl.site_id) > 1
			   AND ($3 OR COUNT(DISTINCT sl.site_id) FILTER (WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive') > 0)
		) sub`
	db.Pool.QueryRow(ctx, countSQL, linkType, func() string {
		if q != "" { return "%" + q + "%" }
		return ""
	}(), includeInactive).Scan(&total)

	rows, err := db.Pool.Query(ctx, `
		SELECT l.id, l.url, l.domain, l.created_at,
		       sl.link_type,
		       COUNT(DISTINCT sl.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites,
		       COUNT(DISTINCT sl.site_id) FILTER (WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive') AS active_count,
		       (BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) = 'inactive') AND BOOL_OR(site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive' AND sl.first_seen > NOW() - INTERVAL '30 days')) AS reappeared
		FROM links l
		JOIN site_links sl ON sl.link_id = l.id
		JOIN sites s ON s.id = sl.site_id
		WHERE sl.link_type IN ('external', 'iframe', 'register')
		  AND s.is_gambling = TRUE
		  AND ($1 = '' OR sl.link_type = $1)
		  AND ($2 = '' OR l.domain ILIKE $2 OR l.url ILIKE $2)
		GROUP BY l.id, sl.link_type
		HAVING COUNT(DISTINCT sl.site_id) > 1
		   AND ($5 OR COUNT(DISTINCT sl.site_id) FILTER (WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive') > 0)
		ORDER BY reappeared DESC, active_count DESC, site_count DESC
		LIMIT $3 OFFSET $4
	`, linkType, func() string {
		if q != "" { return "%" + q + "%" }
		return ""
	}(), limit, offset, includeInactive)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []LinkWithSites
	for rows.Next() {
		var r LinkWithSites
		if err := rows.Scan(&r.ID, &r.URL, &r.Domain, &r.CreatedAt, &r.LinkType, &r.SiteCount, &r.Sites, &r.ActiveCount, &r.Reappeared); err != nil {
			return nil, 0, err
		}
		results = append(results, r)
	}
	return results, total, nil
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}
