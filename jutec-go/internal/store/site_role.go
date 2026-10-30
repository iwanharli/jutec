package store

import "context"

// SaveSiteRole menyimpan peran situs (operator/landing) dan host tujuan tombol daftar/login-nya.
func (db *PostgresDB) SaveSiteRole(ctx context.Context, siteID int, role string, targets []string) error {
	if targets == nil {
		targets = []string{}
	}
	_, err := db.Pool.Exec(ctx, `UPDATE sites SET role = NULLIF($2, ''), entry_targets = $3 WHERE id = $1`, siteID, role, targets)
	return err
}

// RoleTarget: host tujuan landing page + situs Jutec yang cocok (nil = belum pernah dianalisis).
type RoleTarget struct {
	Host string   `json:"host"`
	Site *SiteRef `json:"site"`
}

// RoleLinks menautkan landing page dengan situs utamanya, ke dua arah.
type RoleLinks struct {
	Targets []RoleTarget `json:"targets"` // situs ini (landing) → situs utama tujuan
	Entries []SiteRef    `json:"entries"` // situs lain (landing) → situs ini
}

func (db *PostgresDB) siteRoleLinks(ctx context.Context, siteID int, siteURL string, targets []string) RoleLinks {
	out := RoleLinks{Targets: []RoleTarget{}, Entries: []SiteRef{}}
	for _, h := range targets {
		t := RoleTarget{Host: h}
		var r SiteRef
		if db.Pool.QueryRow(ctx, `
			SELECT id, url, COALESCE(priority,'Low'), COALESCE(confidence,0), COALESCE(is_gambling,false) FROM sites
			WHERE lower(url) ~ ('^https?://(www\.)?' || regexp_replace($1, '([.\-])', '\\\1', 'g') || '(/|$)')
			ORDER BY (role = 'operator') DESC NULLS LAST, last_scanned_at DESC NULLS LAST LIMIT 1`, h).
			Scan(&r.ID, &r.URL, &r.Priority, &r.Confidence, &r.IsGambling) == nil && r.ID != siteID {
			t.Site = &r
		}
		out.Targets = append(out.Targets, t)
	}
	if h := hostOf(siteURL); h != "" {
		rows, err := db.Pool.Query(ctx, `
			SELECT id, url, COALESCE(priority,'Low'), COALESCE(confidence,0), COALESCE(is_gambling,false) FROM sites
			WHERE entry_targets && ARRAY[$1, 'www.' || $1] AND id <> $2
			ORDER BY last_scanned_at DESC NULLS LAST LIMIT 100`, h, siteID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var r SiteRef
				if rows.Scan(&r.ID, &r.URL, &r.Priority, &r.Confidence, &r.IsGambling) == nil {
					out.Entries = append(out.Entries, r)
				}
			}
		}
	}
	return out
}
