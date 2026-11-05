package store

import (
	"context"
	"fmt"
	"time"
)

type DashboardSummary struct {
	TotalSites     int `json:"total_sites"`
	ActiveSites    int `json:"active_sites"`   // judol yang masih beroperasi (aktif + diblokir tapi hidup + belum pasti)
	InactiveSites  int `json:"inactive_sites"` // judol yang sudah tidak aktif
	HighPriority   int `json:"high_priority"`
	ActiveHigh     int `json:"active_high"`
	TotalArtifacts int `json:"total_artifacts"`
	TotalApps      int `json:"total_apps"`
}

type RecentSite struct {
	ID             int     `json:"id"`
	URL            string  `json:"url"`
	Priority       string  `json:"priority"`
	Confidence     float64 `json:"confidence"`
	IsGambling     bool    `json:"is_gambling"`
	LastScannedAt  string  `json:"last_scanned_at"`
	CreatedAt      string  `json:"created_at"`
	Status         string  `json:"status"`
	Lifecycle      string  `json:"lifecycle"`
	Role           string  `json:"role"`
	Category       string  `json:"category"`
	TPListed       *bool   `json:"tp_listed"` // daftar blokir TrustPositif (nil = belum dicek)
	Root           string  `json:"root,omitempty"` // domain induk (mode kelompok per domain)
	ArtifactCount  int     `json:"artifact_count"`
	AppCount       int     `json:"app_count"`
}

// siteCategorySQL: kategori situs untuk label & filter di tabel — dihitung dari data yang sudah ada.
//   betting   — situs judi utama (form login/daftar sendiri, provider game, minta rekening)
//   landing   — pintu masuk: tombol daftar/login ke domain lain
//   intl      — judi internasional (ulasan kasino, operator legal luar negeri, provider game)
//   injected  — konten judi di domain institusi (pemerintah/kampus/sekolah/desa) yang dibobol
//   info      — situs pendukung: RTP, prediksi, bocoran, live draw, paito
//   unclear   — terdeteksi judi, perannya belum bisa ditentukan
//   clean     — tidak terdeteksi judi
const siteCategorySQL = `CASE
	WHEN NOT COALESCE(s.is_gambling, false) THEN 'clean'
	WHEN site_injected(s.url, s.injection) THEN 'injected'
	WHEN s.market = 'intl' THEN 'intl'
	WHEN s.role = 'operator' THEN 'betting'
	WHEN s.role = 'landing' THEN 'landing'
	WHEN s.url ~* '(rtp|prediksi|bocoran|live-?draw|paito|syair|result-?togel|data-?(sgp|hk|sdy))' THEN 'info'
	ELSE 'unclear' END`

// SiteFilter: filter & urutan untuk daftar situs (Repository / Dashboard).
type SiteFilter struct {
	Priority     string
	IsGambling   *bool
	Query        string
	Status       string  // active | unreachable | offline
	Lifecycle    string  // "" semua | live (masih beroperasi) | inactive
	Category     string  // betting | landing | injected | info | unclear | clean (lihat siteCategorySQL)
	GroupByDomain bool   // halaman dihitung per domain induk (site_root); semua situs dalam domain ikut dikembalikan
	JudolOnly    bool    // is_gambling=true tanpa situs susupan (korban), lihat site_is_judol
	TP           string  // listed | not_listed (sudah dicek, belum terdaftar) | unchecked
	MinConf      float64 // 0..1
	MaxConf      float64 // 0..1 (0 = tanpa batas)
	ArtifactType string  // Bank | E-Wallet | Contact | QRIS | Crypto | Phone | any
	HasApp       bool
	ScannedDays  int    // hanya yang di-scan dalam N hari terakhir
	Sort         string // last_scanned_at | created_at | confidence | priority | url | artifact_count | app_count
	Desc         bool
	Limit        int
	Offset       int
}

// siteSortColumns: daftar putih kolom urut — nilai dari klien tak pernah masuk SQL apa adanya.
var siteSortColumns = map[string]string{
	"last_scanned_at": "s.last_scanned_at",
	"created_at":      "s.created_at",
	"confidence":      "s.confidence",
	"priority":        "CASE s.priority WHEN 'High' THEN 3 WHEN 'Medium' THEN 2 ELSE 1 END",
	"url":             "regexp_replace(s.url, '^https?://(www\\.)?', '')",
	"artifact_count":  "artifact_count",
	"app_count":       "app_count",
}

func (db *PostgresDB) GetDashboardSummary(ctx context.Context) (*DashboardSummary, error) {
	summary := &DashboardSummary{}

	// Total Sites
	err := db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM sites").Scan(&summary.TotalSites)
	if err != nil {
		return nil, err
	}

	// High Priority Sites
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM sites WHERE priority = 'High'").Scan(&summary.HighPriority)
	if err != nil {
		return nil, err
	}

	db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE site_lifecycle(reach_status, status, offline_since) <> 'inactive'),
		       COUNT(*) FILTER (WHERE site_lifecycle(reach_status, status, offline_since) = 'inactive'),
		       COUNT(*) FILTER (WHERE priority = 'High' AND site_lifecycle(reach_status, status, offline_since) <> 'inactive')
		FROM sites WHERE is_gambling`).Scan(&summary.ActiveSites, &summary.InactiveSites, &summary.ActiveHigh)

	// Total Artifacts
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM artifacts").Scan(&summary.TotalArtifacts)
	if err != nil {
		return nil, err
	}

	// Total Apps
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM apps").Scan(&summary.TotalApps)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

// GetRecentSites juga mengembalikan total baris yang cocok dengan filter (untuk paginasi).
func (db *PostgresDB) GetRecentSites(ctx context.Context, f SiteFilter) ([]RecentSite, int, error) {
	where := " WHERE 1=1"
	args := []interface{}{}
	add := func(cond string, v interface{}) {
		args = append(args, v)
		where += " AND " + fmt.Sprintf(cond, len(args))
	}
	if f.Priority != "" {
		add("s.priority = $%d", f.Priority)
	}
	if f.IsGambling != nil {
		if *f.IsGambling && f.Category != "injected" && f.Category != "intl" && f.JudolOnly {
			// Tabel "situs judol" (Dashboard): situs korban penyusupan tidak dihitung sebagai situs judol
			where += " AND site_is_judol(s.is_gambling, s.url, s.injection, s.market)"
		} else {
			add("s.is_gambling = $%d", *f.IsGambling)
		}
	}
	if f.Query != "" {
		add("s.url ILIKE $%d", "%"+f.Query+"%")
	}
	switch f.Status {
	case "blocked": // diblokir DNS ISP (masih hidup atau tidak)
		where += " AND s.reach_status IN ('blocked','blocked_alive')"
	case "blocked_alive":
		where += " AND s.reach_status = 'blocked_alive'"
	case "nxdomain":
		where += " AND s.reach_status = 'nxdomain'"
	case "unreachable":
		where += " AND s.status = 'unreachable'"
	case "offline":
		where += " AND s.offline_since IS NOT NULL"
	case "active":
		where += " AND COALESCE(s.status, '') <> 'unreachable' AND s.offline_since IS NULL"
	}
	if f.Category != "" {
		add("("+siteCategorySQL+") = $%d", f.Category)
	}
	switch f.TP {
	case "listed":
		where += " AND s.tp_listed"
	case "not_listed":
		where += " AND s.tp_listed = false"
	case "unchecked":
		where += " AND s.tp_listed IS NULL"
	}
	switch f.Lifecycle {
	case "live":
		where += " AND site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive'"
	case "inactive":
		where += " AND site_lifecycle(s.reach_status, s.status, s.offline_since) = 'inactive'"
	}
	if f.MinConf > 0 {
		add("s.confidence >= $%d", f.MinConf)
	}
	if f.MaxConf > 0 && f.MaxConf < 1 {
		add("s.confidence <= $%d", f.MaxConf)
	}
	if f.ArtifactType == "any" {
		where += " AND EXISTS (SELECT 1 FROM site_artifacts sa WHERE sa.site_id = s.id)"
	} else if f.ArtifactType != "" {
		add("EXISTS (SELECT 1 FROM site_artifacts sa JOIN artifacts a ON a.id = sa.artifact_id WHERE sa.site_id = s.id AND a.type = $%d)", f.ArtifactType)
	}
	if f.HasApp {
		where += " AND EXISTS (SELECT 1 FROM site_apps sp WHERE sp.site_id = s.id)"
	}
	if f.ScannedDays > 0 {
		add("s.last_scanned_at > NOW() - make_interval(days => $%d)", f.ScannedDays)
	}

	col, ok := siteSortColumns[f.Sort]
	if !ok {
		col = siteSortColumns["last_scanned_at"]
	}
	dir := "ASC"
	if f.Desc {
		dir = "DESC"
	}
	if f.GroupByDomain {
		return db.getSiteGroups(ctx, where, args, col, dir, f)
	}
	args = append(args, f.Limit, f.Offset)
	sql := `
		SELECT s.id, s.url, COALESCE(s.priority, 'Low'), COALESCE(s.confidence, 0), COALESCE(s.is_gambling, false),
		       s.last_scanned_at, s.created_at,
		       CASE WHEN s.reach_status IN ('blocked_alive','blocked','nxdomain') THEN s.reach_status
		            WHEN s.status = 'unreachable' THEN 'unreachable' WHEN s.offline_since IS NOT NULL THEN 'offline' ELSE 'active' END,
		       site_lifecycle(s.reach_status, s.status, s.offline_since), COALESCE(s.role, ''), `+siteCategorySQL+`, s.tp_listed,
		       (SELECT COUNT(*) FROM site_artifacts sa WHERE sa.site_id = s.id) AS artifact_count,
		       (SELECT COUNT(*) FROM site_apps sp WHERE sp.site_id = s.id) AS app_count,
		       COUNT(*) OVER() AS total
		FROM sites s` + where +
		fmt.Sprintf(" ORDER BY %s %s NULLS LAST, s.id DESC LIMIT $%d OFFSET $%d", col, dir, len(args)-1, len(args))

	rows, err := db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	results := []RecentSite{}
	total := 0
	for rows.Next() {
		var s RecentSite
		var scanned, created *time.Time
		if err := rows.Scan(&s.ID, &s.URL, &s.Priority, &s.Confidence, &s.IsGambling, &scanned, &created,
			&s.Status, &s.Lifecycle, &s.Role, &s.Category, &s.TPListed, &s.ArtifactCount, &s.AppCount, &total); err != nil {
			return nil, 0, err
		}
		if scanned != nil {
			s.LastScannedAt = scanned.Format(time.RFC3339)
		}
		if created != nil {
			s.CreatedAt = created.Format(time.RFC3339)
		}
		results = append(results, s)
	}
	return results, total, nil
}

// siteRowCols: kolom baris tabel situs (urutan = urutan Scan di scanSiteRow).
const siteRowCols = `s.id, s.url, COALESCE(s.priority, 'Low'), COALESCE(s.confidence, 0), COALESCE(s.is_gambling, false),
		       s.last_scanned_at, s.created_at,
		       CASE WHEN s.reach_status IN ('blocked_alive','blocked','nxdomain') THEN s.reach_status
		            WHEN s.status = 'unreachable' THEN 'unreachable' WHEN s.offline_since IS NOT NULL THEN 'offline' ELSE 'active' END,
		       site_lifecycle(s.reach_status, s.status, s.offline_since), COALESCE(s.role, ''), ` + siteCategorySQL + `, s.tp_listed,
		       (SELECT COUNT(*) FROM site_artifacts sa WHERE sa.site_id = s.id) AS artifact_count,
		       (SELECT COUNT(*) FROM site_apps sp WHERE sp.site_id = s.id) AS app_count`

// getSiteGroups: paginasi per domain induk — urutan domain mengikuti situs "terbaik" di dalamnya
// (sesuai kolom urut), lalu semua situs domain itu dikembalikan berurutan.
func (db *PostgresDB) getSiteGroups(ctx context.Context, where string, args []interface{}, col, dir string, f SiteFilter) ([]RecentSite, int, error) {
	switch col { // alias kolom tidak terlihat di ekspresi SELECT yang sama
	case "artifact_count":
		col = "(SELECT COUNT(*) FROM site_artifacts sa WHERE sa.site_id = s.id)"
	case "app_count":
		col = "(SELECT COUNT(*) FROM site_apps sp WHERE sp.site_id = s.id)"
	}
	agg := "MAX"
	if dir == "ASC" {
		agg = "MIN"
	}
	args = append(args, f.Limit, f.Offset)
	sql := `
		WITH base AS (
		  SELECT ` + siteRowCols + `, site_root(s.url) AS root, (` + col + `) AS sk
		  FROM sites s` + where + `),
		g AS (SELECT root, ` + agg + `(sk) AS sk FROM base GROUP BY root),
		pg AS (SELECT root, COUNT(*) OVER() AS total, ROW_NUMBER() OVER (ORDER BY sk ` + dir + ` NULLS LAST, root) AS rn FROM g
		       ORDER BY rn LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args)) + `)
		SELECT base.*, pg.total FROM base JOIN pg USING (root)
		ORDER BY pg.rn, base.sk ` + dir + ` NULLS LAST, base.id DESC`
	rows, err := db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	results := []RecentSite{}
	total := 0
	for rows.Next() {
		var s RecentSite
		var scanned, created *time.Time
		var sk any
		if err := rows.Scan(&s.ID, &s.URL, &s.Priority, &s.Confidence, &s.IsGambling, &scanned, &created,
			&s.Status, &s.Lifecycle, &s.Role, &s.Category, &s.TPListed, &s.ArtifactCount, &s.AppCount, &s.Root, &sk, &total); err != nil {
			return nil, 0, err
		}
		if scanned != nil {
			s.LastScannedAt = scanned.Format(time.RFC3339)
		}
		if created != nil {
			s.CreatedAt = created.Format(time.RFC3339)
		}
		results = append(results, s)
	}
	return results, total, rows.Err()
}

// SiteCategoryCounts: jumlah situs per kategori (untuk chip filter di tabel).
func (db *PostgresDB) SiteCategoryCounts(ctx context.Context, lifecycle string) map[string]int {
	where := ""
	switch lifecycle {
	case "live":
		where = " WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive'"
	case "inactive":
		where = " WHERE site_lifecycle(s.reach_status, s.status, s.offline_since) = 'inactive'"
	}
	out := map[string]int{}
	rows, err := db.Pool.Query(ctx, `SELECT `+siteCategorySQL+`, COUNT(*) FROM sites s`+where+` GROUP BY 1`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int
		if rows.Scan(&k, &n) == nil {
			out[k] = n
		}
	}
	return out
}
