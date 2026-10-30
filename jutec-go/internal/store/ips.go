package store

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

type IPRecord struct {
	ID        int       `json:"id"`
	Address   string    `json:"address"`
	Subnet24  string    `json:"subnet_24"`
	CreatedAt time.Time `json:"created_at"`
}

type IPWithSites struct {
	IPRecord
	SiteCount int      `json:"site_count"`
	Sites     []string `json:"sites"`
}

// SaveIPs menyimpan IP addresses dari satu situs.
func (db *PostgresDB) SaveIPs(ctx context.Context, siteID int, ips []string) error {
	for _, addr := range ips {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}

		subnet := computeSubnet24(addr)

		var ipID int
		err := db.Pool.QueryRow(ctx, `
			INSERT INTO ips (address, subnet_24)
			VALUES ($1, $2)
			ON CONFLICT (address) DO UPDATE SET subnet_24 = EXCLUDED.subnet_24
			RETURNING id
		`, addr, subnet).Scan(&ipID)
		if err != nil {
			continue
		}

		db.Pool.Exec(ctx, `
			INSERT INTO site_ips (site_id, ip_id)
			VALUES ($1, $2)
			ON CONFLICT (site_id, ip_id) DO NOTHING
		`, siteID, ipID)
	}
	return nil
}

// GetSitesBySubnet mencari semua situs yang berada dalam subnet /24 yang sama.
// subnet format: "103.21.186"
func (db *PostgresDB) GetSitesBySubnet(ctx context.Context, subnet string) ([]IPWithSites, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT i.id, i.address, i.subnet_24, i.created_at,
		       COUNT(DISTINCT si.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites
		FROM ips i
		JOIN site_ips si ON si.ip_id = i.id
		JOIN sites s ON s.id = si.site_id
		WHERE i.subnet_24 = $1
		GROUP BY i.id
		ORDER BY site_count DESC
	`, subnet)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIPRows(rows)
}

// GetSitesByIP mencari semua situs yang pakai IP yang sama.
func (db *PostgresDB) GetSitesByIP(ctx context.Context, ipAddr string) ([]IPWithSites, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT i.id, i.address, i.subnet_24, i.created_at,
		       COUNT(DISTINCT si.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites
		FROM ips i
		JOIN site_ips si ON si.ip_id = i.id
		JOIN sites s ON s.id = si.site_id
		WHERE i.address = $1
		GROUP BY i.id
	`, ipAddr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIPRows(rows)
}

// GetTopSharedIPs mengembalikan IP/subnet yang paling banyak dipakai situs judol.
func (db *PostgresDB) GetTopSharedSubnets(ctx context.Context, limit int) ([]IPWithSites, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT i.id, i.address, i.subnet_24, i.created_at,
		       COUNT(DISTINCT si.site_id) AS site_count,
		       ARRAY_AGG(DISTINCT s.url ORDER BY s.url) AS sites
		FROM ips i
		JOIN site_ips si ON si.ip_id = i.id
		JOIN sites s ON s.id = si.site_id
		WHERE s.is_gambling = TRUE
		GROUP BY i.id
		HAVING COUNT(DISTINCT si.site_id) > 1
		ORDER BY site_count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanIPRows(rows)
}

// GetSimilarTemplates mencari situs lain dengan SimHash yang mirip.
// Karena Hamming distance tidak bisa dihitung langsung di SQL secara efisien,
// kita ambil semua lalu filter di aplikasi.
func (db *PostgresDB) GetSimilarTemplates(ctx context.Context, simhash string, threshold int) ([]map[string]interface{}, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT url, simhash, priority, confidence
		FROM sites
		WHERE simhash IS NOT NULL AND simhash != $1 AND is_gambling = TRUE
	`, simhash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var url, hash, priority string
		var confidence float64
		if err := rows.Scan(&url, &hash, &priority, &confidence); err != nil {
			continue
		}
		dist := hammingDistanceHex(simhash, hash)
		if dist <= threshold {
			results = append(results, map[string]interface{}{
				"url":              url,
				"simhash":          hash,
				"hamming_distance": dist,
				"priority":         priority,
				"confidence":       confidence,
			})
		}
	}
	return results, nil
}

func scanIPRows(rows interface {
	Next() bool
	Scan(...interface{}) error
	Close()
}) ([]IPWithSites, error) {
	defer rows.Close()
	var results []IPWithSites
	for rows.Next() {
		var r IPWithSites
		if err := rows.Scan(&r.ID, &r.Address, &r.Subnet24, &r.CreatedAt, &r.SiteCount, &r.Sites); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func computeSubnet24(ipAddr string) string {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return ""
	}
	ip = ip.To4()
	if ip == nil {
		return "" // IPv6 — skip subnet /24
	}
	return fmt.Sprintf("%d.%d.%d", ip[0], ip[1], ip[2])
}

func hammingDistanceHex(a, b string) int {
	if len(a) != 16 || len(b) != 16 {
		return 64
	}
	var ua, ub uint64
	fmt.Sscanf(a, "%x", &ua)
	fmt.Sscanf(b, "%x", &ub)
	xor := ua ^ ub
	count := 0
	for xor != 0 {
		count += int(xor & 1)
		xor >>= 1
	}
	return count
}
