package store

import (
	"context"
	"fmt"
	"time"

	"jutec-go/internal/whois"
)

type WHOISRecord struct {
	ID               int        `json:"id"`
	SiteID           int        `json:"site_id"`
	Domain           string     `json:"domain"`
	Registrar        string     `json:"registrar"`
	RegistrantName   string     `json:"registrant_name"`
	RegistrantEmail  string     `json:"registrant_email"`
	RegistrantOrg    string     `json:"registrant_org"`
	RegisteredAt     *time.Time `json:"registered_at"`
	ExpiresAt        *time.Time `json:"expires_at"`
	DomainAgeDays    int        `json:"domain_age_days"`
	IsPrivate        bool       `json:"is_private"`
	TLD              string     `json:"tld"`
	RiskScore        float64    `json:"risk_score"`
	ScannedAt        time.Time  `json:"scanned_at"`
	// Join fields
	SiteURL          string     `json:"site_url,omitempty"`
	SitePriority     string     `json:"site_priority,omitempty"`
	SiteIsGambling   bool       `json:"site_is_gambling,omitempty"`
}

// SaveWHOIS menyimpan atau memperbarui data WHOIS untuk satu situs.
func (db *PostgresDB) SaveWHOIS(ctx context.Context, siteID int, rawURL string, intel *whois.DomainIntel) error {
	domain := extractDomain(rawURL)
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO whois_records
		    (site_id, domain, registrar, domain_age_days, is_private, tld, risk_score)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (site_id) DO UPDATE SET
		    registrar       = EXCLUDED.registrar,
		    domain_age_days = EXCLUDED.domain_age_days,
		    is_private      = EXCLUDED.is_private,
		    tld             = EXCLUDED.tld,
		    risk_score      = EXCLUDED.risk_score,
		    scanned_at      = NOW()
	`, siteID, domain, intel.Registrar, intel.DomainAgeDays, intel.IsPrivate, intel.TLD, intel.RiskScore)
	return err
}

// ReverseWHOIS mencari situs yang memiliki atribut WHOIS yang sama.
// Filter: registrar, tld, is_private, usia domain, atau kombinasinya.
type ReverseWHOISFilter struct {
	Registrar  string
	TLD        string
	IsPrivate  *bool
	MaxAgeDays int // 0 = tidak difilter; >0 = domain lebih muda dari N hari
	MinRisk    float64
}

type ReverseWHOISResult struct {
	Total   int           `json:"total"`
	Results []WHOISRecord `json:"results"`
}

func (db *PostgresDB) ReverseWHOIS(ctx context.Context, f ReverseWHOISFilter, limit, offset int) (*ReverseWHOISResult, error) {
	sql := `
		SELECT w.id, w.site_id, w.domain, w.registrar,
		       w.domain_age_days, w.is_private, w.tld, w.risk_score, w.scanned_at,
		       s.url, s.priority, s.is_gambling
		FROM whois_records w
		JOIN sites s ON s.id = w.site_id
		WHERE 1=1
	`
	args := []interface{}{}
	idx := 1

	if f.Registrar != "" {
		sql += ` AND w.registrar ILIKE $` + itoa(idx)
		args = append(args, "%"+f.Registrar+"%")
		idx++
	}
	if f.TLD != "" {
		sql += ` AND w.tld = $` + itoa(idx)
		args = append(args, f.TLD)
		idx++
	}
	if f.IsPrivate != nil {
		sql += ` AND w.is_private = $` + itoa(idx)
		args = append(args, *f.IsPrivate)
		idx++
	}
	if f.MaxAgeDays > 0 {
		sql += ` AND w.domain_age_days >= 0 AND w.domain_age_days <= $` + itoa(idx)
		args = append(args, f.MaxAgeDays)
		idx++
	}
	if f.MinRisk > 0 {
		sql += ` AND w.risk_score >= $` + itoa(idx)
		args = append(args, f.MinRisk)
		idx++
	}

	// Count total
	countSQL := `SELECT COUNT(*) FROM (` + sql + `) t`
	var total int
	db.Pool.QueryRow(ctx, countSQL, args...).Scan(&total)

	sql += ` ORDER BY w.risk_score DESC, s.priority DESC LIMIT $` + itoa(idx) + ` OFFSET $` + itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []WHOISRecord
	for rows.Next() {
		var r WHOISRecord
		if err := rows.Scan(&r.ID, &r.SiteID, &r.Domain, &r.Registrar,
			&r.DomainAgeDays, &r.IsPrivate, &r.TLD, &r.RiskScore, &r.ScannedAt,
			&r.SiteURL, &r.SitePriority, &r.SiteIsGambling); err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	return &ReverseWHOISResult{Total: total, Results: results}, nil
}

func itoa(i int) string { return fmt.Sprintf("%d", i) }
