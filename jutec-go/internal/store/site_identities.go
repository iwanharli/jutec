package store

import (
	"context"
	"time"
)

// SiteIdentity: satu nomor kontak milik situs beserta identitas pemilik dari data bocoran.
type SiteIdentity struct {
	ArtifactID int            `json:"artifact_id"`
	Phone      string         `json:"phone"`      // ternormalisasi (62…)
	Label      string         `json:"label"`      // nama artefak (WhatsApp, dll)
	Checked    bool           `json:"checked"`    // sudah pernah dicek ke bot/API
	KomInfo    []LeakKomInfo  `json:"kominfo"`    // registrasi seluler cocok persis
	Dukcapil   []LeakDukcapil `json:"dukcapil"`   // data kependudukan cocok persis
	Related    []LeakKomInfo  `json:"related"`    // nomor lain dengan NIK yang sama
}

// GetSiteIdentities mengumpulkan nomor kontak situs lalu mencocokkannya dengan data bocoran yang
// sudah tersimpan (KomInfo/Dukcapil, sumber LeakOSINT maupun VflOSINT). Tidak memanggil bot.
func (db *PostgresDB) GetSiteIdentities(ctx context.Context, siteID int) ([]SiteIdentity, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT a.id, regexp_replace(a.value, '^\+', ''), COALESCE(NULLIF(a.name, ''), a.type)
		FROM site_artifacts sa JOIN artifacts a ON a.id = sa.artifact_id
		WHERE sa.site_id = $1 AND a.type IN ('Contact','Phone') AND a.value ~ '^\+?628[0-9]{7,12}$'
		GROUP BY a.id, a.value, a.name, a.type ORDER BY a.id`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type ph struct {
		id    int
		phone string
		label string
	}
	var phones []ph
	for rows.Next() {
		var p ph
		if rows.Scan(&p.id, &p.phone, &p.label) == nil {
			phones = append(phones, p)
		}
	}

	out := []SiteIdentity{}
	for _, p := range phones {
		it := SiteIdentity{ArtifactID: p.id, Phone: p.phone, Label: p.label, KomInfo: []LeakKomInfo{}, Dukcapil: []LeakDukcapil{}, Related: []LeakKomInfo{}}
		if km, _ := db.GetLeakKomInfoByPhone(ctx, p.phone); km != nil {
			it.KomInfo = km
		}
		if dk, _ := db.GetLeakDukcapilByPhone(ctx, p.phone); dk != nil {
			it.Dukcapil = dk
		}
		it.Checked = len(it.KomInfo) > 0 || len(it.Dukcapil) > 0 || db.VflQueriedWithin(ctx, p.phone, 365*24*time.Hour)
		seen := map[string]bool{}
		for _, k := range it.KomInfo {
			if k.NIK == "" || seen[k.NIK] {
				continue
			}
			seen[k.NIK] = true
			others, _ := db.GetLeakKomInfoByNIK(ctx, k.NIK)
			for _, o := range others {
				if o.Phone != p.phone {
					it.Related = append(it.Related, o)
				}
			}
		}
		out = append(out, it)
	}
	return out, nil
}
