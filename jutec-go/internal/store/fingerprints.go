package store

import "context"

// SaveFingerprints menyimpan sidik jari infrastruktur situs (upsert, idempoten).
func (db *PostgresDB) SaveFingerprints(ctx context.Context, siteID int, fps []map[string]string) error {
	for _, f := range fps {
		var id int
		err := db.Pool.QueryRow(ctx, `
			INSERT INTO fingerprints (kind, value) VALUES ($1, $2)
			ON CONFLICT (kind, value) DO UPDATE SET kind = EXCLUDED.kind
			RETURNING id`, f["kind"], f["value"]).Scan(&id)
		if err != nil {
			return err
		}
		if _, err := db.Pool.Exec(ctx, `
			INSERT INTO site_fingerprints (site_id, fingerprint_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, siteID, id); err != nil {
			return err
		}
	}
	return nil
}

// SaveInternalPages mencatat halaman internal yang ikut dianalisis pada scan terakhir.
func (db *PostgresDB) SaveInternalPages(ctx context.Context, siteID int, pages []string) error {
	if pages == nil {
		pages = []string{}
	}
	_, err := db.Pool.Exec(ctx, `UPDATE sites SET internal_pages = $2 WHERE id = $1`, siteID, pages)
	return err
}

// SaveAuthSurface menyimpan peta login & pendaftaran hasil scan terakhir (JSON).
func (db *PostgresDB) SaveAuthSurface(ctx context.Context, siteID int, surface any) error {
	_, err := db.Pool.Exec(ctx, `UPDATE sites SET auth_surface = $2 WHERE id = $1`, siteID, surface)
	return err
}

// SaveWidgetLinks menyimpan link dari pesan sambutan widget chat (LiveChat dll) pada scan terakhir.
func (db *PostgresDB) SaveWidgetLinks(ctx context.Context, siteID int, links []string) error {
	if links == nil {
		links = []string{}
	}
	_, err := db.Pool.Exec(ctx, `UPDATE sites SET widget_links = $2 WHERE id = $1`, siteID, links)
	return err
}

// SaveInjection menyimpan tanda penyusupan hasil scan terakhir (nil = tidak ada → dikosongkan).
// detected=false juga dikosongkan agar kolom hanya terisi bila ada sinyal.
func (db *PostgresDB) SaveInjection(ctx context.Context, siteID int, inj any, detected bool) error {
	if !detected {
		inj = nil
	}
	_, err := db.Pool.Exec(ctx, `UPDATE sites SET injection = $2 WHERE id = $1`, siteID, inj)
	return err
}
