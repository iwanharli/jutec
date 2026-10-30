package store

import (
	"context"
	"time"
)

type NameserverRecord struct {
	ID        int       `json:"id"`
	Host      string    `json:"host"`
	CreatedAt time.Time `json:"created_at"`
}

type NameserverWithSites struct {
	NameserverRecord
	SiteCount int      `json:"site_count"`
	Sites     []string `json:"sites"`
}

type SSLCertRecord struct {
	ID          int       `json:"id"`
	Thumbprint  string    `json:"thumbprint"`
	CreatedAt   time.Time `json:"created_at"`
}

type SSLCertWithSites struct {
	SSLCertRecord
	SiteCount int      `json:"site_count"`
	Sites     []string `json:"sites"`
}

// SaveNameservers menyimpan nameserver dari satu situs.
func (db *PostgresDB) SaveNameservers(ctx context.Context, siteID int, hosts []string) error {
	for _, host := range hosts {
		if host == "" {
			continue
		}
		var nsID int
		err := db.Pool.QueryRow(ctx, `
			INSERT INTO nameservers (host)
			VALUES ($1)
			ON CONFLICT (host) DO UPDATE SET host = EXCLUDED.host
			RETURNING id
		`, host).Scan(&nsID)
		if err != nil {
			continue
		}
		db.Pool.Exec(ctx, `
			INSERT INTO site_ns (site_id, ns_id)
			VALUES ($1, $2)
			ON CONFLICT (site_id, ns_id) DO NOTHING
		`, siteID, nsID)
	}
	return nil
}

// SaveSSLCert menyimpan SSL certificate thumbprint dari satu situs.
func (db *PostgresDB) SaveSSLCert(ctx context.Context, siteID int, thumbprint string) error {
	if thumbprint == "" {
		return nil
	}
	var certID int
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO ssl_certs (thumbprint)
		VALUES ($1)
		ON CONFLICT (thumbprint) DO UPDATE SET thumbprint = EXCLUDED.thumbprint
		RETURNING id
	`, thumbprint).Scan(&certID)
	if err != nil {
		return err
	}
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO site_ssl (site_id, cert_id)
		VALUES ($1, $2)
		ON CONFLICT (site_id, cert_id) DO NOTHING
	`, siteID, certID)
	return err
}

// GetSitesByNameserver mencari semua situs yang pakai nameserver yang sama.
func (db *PostgresDB) GetSitesByNameserver(ctx context.Context, host string) ([]NameserverWithSites, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT n.id, n.host, n.created_at,
		       COUNT(DISTINCT sn.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites
		FROM nameservers n
		JOIN site_ns sn ON sn.ns_id = n.id
		JOIN sites s ON s.id = sn.site_id
		WHERE n.host = $1
		GROUP BY n.id
	`, host)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []NameserverWithSites
	for rows.Next() {
		var r NameserverWithSites
		if err := rows.Scan(&r.ID, &r.Host, &r.CreatedAt, &r.SiteCount, &r.Sites); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetTopSharedNameservers mengembalikan nameserver yang dipakai paling banyak situs judol.
func (db *PostgresDB) GetTopSharedNameservers(ctx context.Context, limit int) ([]NameserverWithSites, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT n.id, n.host, n.created_at,
		       COUNT(DISTINCT sn.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites
		FROM nameservers n
		JOIN site_ns sn ON sn.ns_id = n.id
		JOIN sites s ON s.id = sn.site_id
		GROUP BY n.id
		HAVING COUNT(DISTINCT sn.site_id) >= 2
		ORDER BY site_count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []NameserverWithSites
	for rows.Next() {
		var r NameserverWithSites
		if err := rows.Scan(&r.ID, &r.Host, &r.CreatedAt, &r.SiteCount, &r.Sites); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetSitesBySSLCert mencari semua situs yang berbagi SSL cert yang sama.
func (db *PostgresDB) GetSitesBySSLCert(ctx context.Context, thumbprint string) ([]SSLCertWithSites, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT c.id, c.thumbprint, c.created_at,
		       COUNT(DISTINCT ss.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites
		FROM ssl_certs c
		JOIN site_ssl ss ON ss.cert_id = c.id
		JOIN sites s ON s.id = ss.site_id
		WHERE c.thumbprint = $1
		GROUP BY c.id
	`, thumbprint)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SSLCertWithSites
	for rows.Next() {
		var r SSLCertWithSites
		if err := rows.Scan(&r.ID, &r.Thumbprint, &r.CreatedAt, &r.SiteCount, &r.Sites); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetTopSharedSSLCerts mengembalikan SSL cert yang dipakai paling banyak situs judol.
func (db *PostgresDB) GetTopSharedSSLCerts(ctx context.Context, limit int) ([]SSLCertWithSites, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT c.id, c.thumbprint, c.created_at,
		       COUNT(DISTINCT ss.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites
		FROM ssl_certs c
		JOIN site_ssl ss ON ss.cert_id = c.id
		JOIN sites s ON s.id = ss.site_id
		WHERE s.is_gambling = TRUE
		GROUP BY c.id
		HAVING COUNT(DISTINCT ss.site_id) > 1
		ORDER BY site_count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SSLCertWithSites
	for rows.Next() {
		var r SSLCertWithSites
		if err := rows.Scan(&r.ID, &r.Thumbprint, &r.CreatedAt, &r.SiteCount, &r.Sites); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// GetSiteURLsBySharedNS mengembalikan URL situs lain yang berbagi nameserver
// dengan salah satu dari sourceURLs. Dipakai untuk NS cluster discovery.
func (db *PostgresDB) GetSiteURLsBySharedNS(ctx context.Context, sourceURLs []string) ([]string, error) {
	if len(sourceURLs) == 0 {
		return nil, nil
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT DISTINCT s.url
		FROM sites s
		JOIN site_nameservers sn  ON sn.site_id    = s.id
		JOIN nameservers      ns  ON ns.id          = sn.nameserver_id
		WHERE ns.host IN (
			SELECT DISTINCT ns2.host
			FROM sites          s2
			JOIN site_nameservers sn2 ON sn2.site_id    = s2.id
			JOIN nameservers      ns2 ON ns2.id          = sn2.nameserver_id
			WHERE s2.url = ANY($1)
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
