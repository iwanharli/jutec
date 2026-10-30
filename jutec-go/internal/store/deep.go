package store

import (
	"context"
	"time"
)

type DeepApp struct {
	ID  int
	URL string
}

// DeepTargets: URL situs, gambar (kandidat QRIS/OCR diurutkan paling relevan dulu), dan APK yang belum dianalisis.
func (db *PostgresDB) DeepTargets(ctx context.Context, siteID int) (siteURL string, images []string, apps []DeepApp, err error) {
	if err = db.Pool.QueryRow(ctx, `SELECT url FROM sites WHERE id = $1`, siteID).Scan(&siteURL); err != nil {
		return
	}
	rows, err := db.Pool.Query(ctx, `
		SELECT i.url FROM site_images si JOIN images i ON i.id = si.image_id
		WHERE si.site_id = $1 AND i.url !~* '\.(svg|ico)(\?|$)'
		ORDER BY (i.url ~* '(qr|qris|deposit|depo|bayar|payment|bank|rek|wa|whatsapp|contact|kontak|cs|promo)') DESC, si.id
		LIMIT 60`, siteID)
	if err != nil {
		return
	}
	for rows.Next() {
		var u string
		if rows.Scan(&u) == nil {
			images = append(images, u)
		}
	}
	rows.Close()

	rows, err = db.Pool.Query(ctx, `
		SELECT a.id, a.url FROM site_apps sa JOIN apps a ON a.id = sa.app_id
		LEFT JOIN apk_analyses x ON x.app_id = a.id
		WHERE sa.site_id = $1
		  AND (x.app_id IS NULL OR x.analyzed_at < NOW() - INTERVAL '30 days'
		       OR (x.package_name IS NULL OR x.package_name = ''))  -- gagal sebelumnya (server APK mati) → coba lagi
		  AND a.url ~* '\.apk(\?|$)'`, siteID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var a DeepApp
		if rows.Scan(&a.ID, &a.URL) == nil {
			apps = append(apps, a)
		}
	}
	return
}

type APKRecord struct {
	SHA256, PackageName, VersionName, AppLabel, CertSHA256, CertSubject, CertIssuer string
	SizeBytes                                                                      int64
	CertNotBefore                                                                  *time.Time
	Permissions, URLs, Domains                                                     []string
	Error                                                                          string
}

func nz(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (db *PostgresDB) SaveAPKAnalysis(ctx context.Context, appID int, r APKRecord) error {
	// Gagal unduh/bongkar: jangan timpa hasil sukses sebelumnya, cukup catat error-nya
	if r.Error != "" && r.PackageName == "" {
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO apk_analyses (app_id, error, analyzed_at) VALUES ($1, $2, NOW())
			ON CONFLICT (app_id) DO UPDATE SET error = $2,
			  analyzed_at = CASE WHEN apk_analyses.package_name IS NULL OR apk_analyses.package_name = '' THEN NOW() ELSE apk_analyses.analyzed_at END`,
			appID, r.Error)
		return err
	}
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO apk_analyses (app_id, sha256, size_bytes, package_name, version_name, app_label, cert_sha256,
		                          cert_subject, cert_issuer, cert_not_before, permissions, urls, domains, error, analyzed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,''),NOW())
		ON CONFLICT (app_id) DO UPDATE SET sha256=$2, size_bytes=$3, package_name=$4, version_name=$5, app_label=$6,
		  cert_sha256=$7, cert_subject=$8, cert_issuer=$9, cert_not_before=$10, permissions=$11, urls=$12, domains=$13,
		  error=NULLIF($14,''), analyzed_at=NOW()`,
		appID, r.SHA256, r.SizeBytes, r.PackageName, r.VersionName, r.AppLabel, r.CertSHA256, r.CertSubject, r.CertIssuer,
		r.CertNotBefore, nz(r.Permissions), nz(r.URLs), nz(r.Domains), r.Error)
	return err
}

// SitesUsingApp: semua situs yang menautkan APK ini (sidik jari APK disebar ke semuanya).
func (db *PostgresDB) SitesUsingApp(ctx context.Context, appID int) []int {
	rows, err := db.Pool.Query(ctx, `SELECT site_id FROM site_apps WHERE app_id = $1`, appID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// APKAnalysesForSite: hasil analisis APK untuk halaman detail.
func (db *PostgresDB) APKAnalysesForSite(ctx context.Context, siteID int) ([]map[string]any, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT a.url, COALESCE(x.package_name,''), COALESCE(x.version_name,''), COALESCE(x.app_label,''),
		       COALESCE(x.cert_sha256,''), COALESCE(x.cert_subject,''), x.cert_not_before, COALESCE(x.size_bytes,0),
		       x.permissions, x.domains, COALESCE(x.error,''), x.analyzed_at,
		       (SELECT COUNT(DISTINCT sa2.site_id) FROM apk_analyses x2 JOIN site_apps sa2 ON sa2.app_id = x2.app_id
		        WHERE x.cert_sha256 <> '' AND x2.cert_sha256 = x.cert_sha256 AND sa2.site_id <> $1)
		FROM site_apps sa JOIN apps a ON a.id = sa.app_id JOIN apk_analyses x ON x.app_id = a.id
		WHERE sa.site_id = $1 ORDER BY x.analyzed_at DESC`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var url, pkg, ver, label, cert, subj, errS string
		var notBefore, analyzed *time.Time
		var size int64
		var perms, domains []string
		var sameCert int
		if err := rows.Scan(&url, &pkg, &ver, &label, &cert, &subj, &notBefore, &size, &perms, &domains, &errS, &analyzed, &sameCert); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"url": url, "package_name": pkg, "version_name": ver, "app_label": label, "cert_sha256": cert,
			"cert_subject": subj, "cert_not_before": notBefore, "size_bytes": size, "permissions": perms,
			"domains": domains, "error": errS, "analyzed_at": analyzed, "same_cert_sites": sameCert,
		})
	}
	return out, nil
}
