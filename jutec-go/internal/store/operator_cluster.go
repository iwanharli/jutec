package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Atribut KUAT untuk menyatukan situs ke satu operator. Atribut dipakai >maxClusterShare situs
// dianggap umum (CDN publik, kontak agregator) sehingga diabaikan.
const maxClusterShare = 40

var clusterAttrQueries = map[string]string{
	"artifact": `SELECT 'artifact:' || a.id, a.type || COALESCE(' ' || NULLIF(a.name,''), '') || ': ' || a.value, sa.site_id
		FROM artifacts a JOIN site_artifacts sa ON sa.artifact_id = a.id JOIN sites s ON s.id = sa.site_id
		WHERE s.is_gambling AND a.type IN ('Bank','E-Wallet','QRIS','Crypto','Contact','Phone','VirtualAccount')`,
	"fingerprint": `SELECT 'fp:' || f.id, f.kind || ': ' || f.value, sf.site_id
		FROM fingerprints f JOIN site_fingerprints sf ON sf.fingerprint_id = f.id JOIN sites s ON s.id = sf.site_id
		WHERE s.is_gambling AND f.kind IN ('referral','apk_cert','apk_pkg','cdn','favicon')`,
	"tracking": `SELECT 'trk:' || t.id, t.platform || ': ' || t.value, st.site_id
		FROM tracking_ids t JOIN site_tracking st ON st.tracking_id = t.id JOIN sites s ON s.id = st.site_id
		WHERE s.is_gambling AND t.platform IN ('LiveChat','TawkTo','Crisp','FacebookPixel','GoogleTag','GoogleAnalytics',
		                                       'GoogleTagManager','AsiaSigma','GigaGaming',
		                                       -- identifier pengelola: kunci layanan yang terdaftar atas akun pemilik
		                                       'ReCaptcha','GoogleAPIKey','FirebaseProject','AdSense','TelegramBot','TelegramChat',
		                                       'OneSignal','Sentry','CloudflareAnalytics','Clarity','YandexMetrica','TikTokPixel',
		                                       'Jivo','Tidio','Zendesk','Smartsupp')`,
}

type clusterEvidence struct {
	Attr  string `json:"attr"`
	Sites int    `json:"sites"`
}

// ClusterOperators mengelompokkan situs judol yang berbagi atribut kuat (union-find) menjadi
// operator berplatform "Klaster". Klaster lama dihapus & dibangun ulang tiap dijalankan.
func (db *PostgresDB) ClusterOperators(ctx context.Context) (int, error) {
	attrSites := map[string][]int{}
	attrLabel := map[string]string{}
	for _, q := range clusterAttrQueries {
		rows, err := db.Pool.Query(ctx, q)
		if err != nil {
			return 0, err
		}
		for rows.Next() {
			var key, label string
			var site int
			if rows.Scan(&key, &label, &site) == nil {
				attrSites[key] = append(attrSites[key], site)
				attrLabel[key] = label
			}
		}
		rows.Close()
	}

	// Identifier yang juga terpasang di domain platform (blacklist: LiveChat, CDN, toko aplikasi, …)
	// milik penyedia layanan — ikut termuat lewat widget/iframe, bukan milik operator.
	platformOwned := map[string]bool{}
	// Berlaku untuk tracking, artefak (mis. akun Discord resmi "aptoide"), dan sidik jari.
	if rows, err := db.Pool.Query(ctx, `
		WITH plat AS (
			SELECT s.id FROM sites s
			CROSS JOIN LATERAL (SELECT rtrim(lower(split_part(regexp_replace(s.url, '^https?://', ''), '/', 1)), '/') AS host) h
			WHERE EXISTS (SELECT 1 FROM domain_blacklist b
			              WHERE b.is_active AND (h.host = b.domain OR h.host LIKE '%.' || b.domain)))
		SELECT DISTINCT 'trk:' || tracking_id FROM site_tracking WHERE site_id IN (SELECT id FROM plat)
		UNION SELECT DISTINCT 'artifact:' || artifact_id FROM site_artifacts WHERE site_id IN (SELECT id FROM plat)
		UNION SELECT DISTINCT 'fp:' || fingerprint_id FROM site_fingerprints WHERE site_id IN (SELECT id FROM plat)`); err == nil {
		for rows.Next() {
			var k string
			if rows.Scan(&k) == nil {
				platformOwned[k] = true
			}
		}
		rows.Close()
	}

	// Domain induk tiap situs: identifier yang hanya muncul di subdomain SATU domain induk
	// (mis. AdSense milik uptodown.com di 93 halaman *.uptodown.com) adalah milik platform itu —
	// tidak menambah bukti lintas domain, jadi tidak dipakai menyatukan klaster.
	parentDomain := map[int]string{}
	if rows, err := db.Pool.Query(ctx, `SELECT id, url FROM sites WHERE is_gambling`); err == nil {
		for rows.Next() {
			var id int
			var u string
			if rows.Scan(&id, &u) == nil {
				parentDomain[id] = registrableOf(u)
			}
		}
		rows.Close()
	}
	sameParent := func(sites []int) bool {
		d := parentDomain[sites[0]]
		for _, s := range sites[1:] {
			if parentDomain[s] != d {
				return false
			}
		}
		return d != ""
	}

	// Union-find
	parent := map[int]int{}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		if _, ok := parent[a]; !ok {
			parent[a] = a
		}
		if _, ok := parent[b]; !ok {
			parent[b] = b
		}
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}
	for key, sites := range attrSites {
		uniq := uniqueInts(sites)
		attrSites[key] = uniq
		if len(uniq) < 2 || len(uniq) > maxClusterShare || (len(uniq) > 2 && sameParent(uniq)) || platformOwned[key] {
			continue
		}
		for _, s := range uniq[1:] {
			union(uniq[0], s)
		}
	}

	clusters := map[int][]int{}
	for s := range parent {
		r := find(s)
		clusters[r] = append(clusters[r], s)
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM operators WHERE platform = 'Klaster'`); err != nil {
		return 0, err
	}

	created := 0
	for _, members := range clusters {
		if len(members) < 2 {
			continue
		}
		sort.Ints(members)
		inCluster := map[int]bool{}
		for _, m := range members {
			inCluster[m] = true
		}
		// Bukti: atribut yang menghubungkan ≥2 anggota klaster ini
		var ev []clusterEvidence
		for key, sites := range attrSites {
			n := 0
			for _, s := range sites {
				if inCluster[s] {
					n++
				}
			}
			if n >= 2 && len(sites) <= maxClusterShare {
				ev = append(ev, clusterEvidence{Attr: attrLabel[key], Sites: n})
			}
		}
		sort.Slice(ev, func(i, j int) bool { return ev[i].Sites > ev[j].Sites })
		if len(ev) > 15 {
			ev = ev[:15]
		}
		evJSON, _ := json.Marshal(ev)

		var name string
		var avg float64
		tx.QueryRow(ctx, `SELECT url, (SELECT AVG(confidence) FROM sites WHERE id = ANY($1)) FROM sites
			WHERE id = ANY($1) ORDER BY confidence DESC, id LIMIT 1`, members).Scan(&name, &avg)
		if u, err := url.Parse(name); err == nil && u.Hostname() != "" {
			name = strings.TrimPrefix(u.Hostname(), "www.")
		}

		var opID int
		err := tx.QueryRow(ctx, `
			INSERT INTO operators (name, platform, operator_key, site_count, avg_confidence, evidence, updated_at)
			VALUES ($1, 'Klaster', $2, $3, $4, $5, NOW()) RETURNING id`,
			"Klaster "+name, fmt.Sprintf("cluster:%d", members[0]), len(members), avg, evJSON).Scan(&opID)
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO site_operators (site_id, operator_id) SELECT unnest($1::int[]), $2
			ON CONFLICT DO NOTHING`, members, opID); err != nil {
			return 0, err
		}
		created++
	}
	return created, tx.Commit(ctx)
}

func uniqueInts(in []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Ints(out)
	return out
}

// registrableOf: domain induk dari URL (dua label terakhir; tiga untuk co.id, com.br, dst.).
func registrableOf(raw string) string {
	h := hostOf(raw)
	parts := strings.Split(strings.TrimSuffix(strings.ToLower(h), "."), ".")
	if len(parts) <= 2 {
		return strings.Join(parts, ".")
	}
	if l := len(parts); len(parts[l-2]) <= 3 && len(parts[l-1]) == 2 {
		return strings.Join(parts[l-3:], ".")
	}
	return strings.Join(parts[len(parts)-2:], ".")
}
