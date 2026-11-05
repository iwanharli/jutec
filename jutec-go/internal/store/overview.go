package store

import (
	"context"
	"time"
)

// Ringkasan dashboard: apa yang perlu dikerjakan, tren, dan peringkat yang mengarah ke operator.
// Semua hitungan "aktif" memakai site_lifecycle agar sama dengan tabel situs & repository.
const liveSite = `site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive'`

type OverviewSite struct {
	ID         int        `json:"id"`
	URL        string     `json:"url"`
	Priority   string     `json:"priority"`
	Confidence float64    `json:"confidence"`
	At         *time.Time `json:"at"` // waktu kejadian yang relevan untuk daftar tsb
}

type ActionList struct {
	Count int            `json:"count"`
	Items []OverviewSite `json:"items"`
}

type OverviewArtifact struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Value       string `json:"value"`
	ActiveSites int    `json:"active_sites"`
	TotalSites  int    `json:"total_sites"`
}

type OverviewRank struct {
	ID          int    `json:"id,omitempty"`
	Label       string `json:"label"`
	Sub         string `json:"sub,omitempty"`
	ActiveSites int    `json:"active_sites"`
	TotalSites  int    `json:"total_sites"`
}

type TrendDay struct {
	Day     string `json:"day"` // YYYY-MM-DD
	New     int    `json:"new"`
	Died    int    `json:"died"`
	Blocked int    `json:"blocked"`
}

type Overview struct {
	KPI struct {
		Live            int `json:"live"`
		LivePrev        int `json:"live_prev"` // perkiraan jumlah beroperasi 7 hari lalu
		New7d           int `json:"new_7d"`
		NewPrev7d       int `json:"new_prev_7d"`
		LiveHigh        int `json:"live_high"`
		Blocked7d       int `json:"blocked_7d"`
		BlockedPrev7d   int `json:"blocked_prev_7d"`
		Died7d          int `json:"died_7d"`
		ActiveOperators int `json:"active_operators"`
		Injected        int `json:"injected"`
		Intl            int `json:"intl"` // judi internasional yang masih beroperasi — tidak dihitung sebagai judol // situs sah yang disusupi (korban), masih beroperasi — tidak dihitung sebagai judol
		ActiveMoney     int `json:"active_money"` // rekening/e-wallet/QRIS/kripto di situs beroperasi
		Inactive        int `json:"inactive"`
		Total           int `json:"total"`
	} `json:"kpi"`
	Actions struct {
		Unverified   ActionList         `json:"unverified"`    // perlu analis: judol beroperasi, bukti lemah/sedang
		ReadyBlock   ActionList         `json:"ready_block"`   // siap diajukan blokir: judol pasti + bukti forensik + masih online
		Revived      ActionList         `json:"revived"`       // hidup lagi ≤30 hari
		BlockedAlive ActionList         `json:"blocked_alive"` // diblokir ISP tapi masih melayani
		NoEvidence   ActionList         `json:"no_evidence"`   // High beroperasi tanpa bukti forensik
		Reappeared   []OverviewArtifact `json:"reappeared"`    // rekening/kontak lama muncul di situs baru
	} `json:"actions"`
	Lifecycle map[string]int     `json:"lifecycle"` // active | blocked_alive | uncertain | inactive
	Trend     []TrendDay         `json:"trend"`     // 30 hari
	TopOps    []OverviewRank     `json:"top_operators"`
	TopMoney  []OverviewArtifact `json:"top_money"`
	TopProv   []OverviewRank     `json:"top_providers"`
	TopHosts  []OverviewRank     `json:"top_hosts"`
}

func (db *PostgresDB) actionList(ctx context.Context, where, at, order string) (ActionList, error) {
	out := ActionList{Items: []OverviewSite{}}
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM sites s WHERE `+where).Scan(&out.Count); err != nil {
		return out, err
	}
	rows, err := db.Pool.Query(ctx, `SELECT s.id, s.url, COALESCE(s.priority,'Low'), COALESCE(s.confidence,0), `+at+`
		FROM sites s WHERE `+where+` ORDER BY `+order+` LIMIT 5`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var x OverviewSite
		if err := rows.Scan(&x.ID, &x.URL, &x.Priority, &x.Confidence, &x.At); err != nil {
			return out, err
		}
		out.Items = append(out.Items, x)
	}
	return out, rows.Err()
}

func (db *PostgresDB) GetOverview(ctx context.Context) (*Overview, error) {
	o := &Overview{Lifecycle: map[string]int{}}
	k := &o.KPI
	// Situs "mati" pada hari X = hari ia mulai dianggap tidak aktif: NXDOMAIN langsung, offline/blocked setelah 14 hari.
	const diedAt = `CASE WHEN s.reach_status = 'nxdomain' THEN s.offline_since ELSE s.offline_since + INTERVAL '14 days' END`
	err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE `+liveSite+`),
		       COUNT(*) FILTER (WHERE s.created_at > NOW() - INTERVAL '7 days'),
		       COUNT(*) FILTER (WHERE s.created_at BETWEEN NOW() - INTERVAL '14 days' AND NOW() - INTERVAL '7 days'),
		       COUNT(*) FILTER (WHERE `+liveSite+` AND s.priority = 'High'),
		       COUNT(*) FILTER (WHERE s.blocked_since > NOW() - INTERVAL '7 days'),
		       COUNT(*) FILTER (WHERE s.blocked_since BETWEEN NOW() - INTERVAL '14 days' AND NOW() - INTERVAL '7 days'),
		       COUNT(*) FILTER (WHERE NOT (`+liveSite+`) AND `+diedAt+` > NOW() - INTERVAL '7 days'),
		       COUNT(*) FILTER (WHERE NOT (`+liveSite+`)),
		       COUNT(*)
		FROM sites s WHERE site_is_judol(s.is_gambling, s.url, s.injection, s.market)`).Scan(&k.Live, &k.New7d, &k.NewPrev7d, &k.LiveHigh, &k.Blocked7d, &k.BlockedPrev7d, &k.Died7d, &k.Inactive, &k.Total)
	if err != nil {
		return nil, err
	}
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM sites s WHERE s.is_gambling AND site_injected(s.url, s.injection) AND `+liveSite).Scan(&k.Injected)
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM sites s WHERE s.is_gambling AND s.market = 'intl' AND NOT site_injected(s.url, s.injection) AND `+liveSite).Scan(&k.Intl)
	// Beroperasi 7 hari lalu ≈ sekarang − baru + yang mati dalam 7 hari
	k.LivePrev = k.Live - k.New7d + k.Died7d

	db.Pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT so.operator_id) FROM site_operators so JOIN sites s ON s.id = so.site_id
		WHERE site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND `+liveSite).Scan(&k.ActiveOperators)
	db.Pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT a.id) FROM artifacts a JOIN site_artifacts sa ON sa.artifact_id = a.id JOIN sites s ON s.id = sa.site_id
		WHERE a.type IN ('Bank','E-Wallet','VirtualAccount','QRIS','Crypto') AND site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND `+liveSite).Scan(&k.ActiveMoney)

	if o.Actions.Unverified, err = db.actionList(ctx,
		`site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND s.verified_status IS NULL AND NOT `+StrongEvidenceSQL+` AND `+liveSite,
		`s.created_at`, `s.confidence DESC, s.created_at DESC`); err != nil {
		return nil, err
	}
	// Judol pasti (diverifikasi analis atau bukti kuat) + bukti forensik ada + masih bisa diakses biasa (belum diblokir)
	if o.Actions.ReadyBlock, err = db.actionList(ctx,
		`site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND s.verified_status IS DISTINCT FROM 'bukan_judol'
		 AND (s.verified_status = 'judol' OR `+StrongEvidenceSQL+`)
		 AND EXISTS (SELECT 1 FROM evidence_captures e WHERE e.site_id = s.id AND e.status = 'done')
		 AND COALESCE(s.reach_status, 'online') = 'online' AND NOT COALESCE(s.tp_listed, false) AND `+liveSite,
		`s.created_at`, `s.priority = 'High' DESC, s.confidence DESC`); err != nil {
		return nil, err
	}
	if o.Actions.Revived, err = db.actionList(ctx,
		`site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND s.revived_at > NOW() - INTERVAL '30 days' AND `+liveSite,
		`s.revived_at`, `s.revived_at DESC`); err != nil {
		return nil, err
	}
	if o.Actions.BlockedAlive, err = db.actionList(ctx,
		`site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND s.reach_status = 'blocked_alive'`,
		`s.blocked_since`, `s.priority = 'High' DESC, s.blocked_since DESC NULLS LAST`); err != nil {
		return nil, err
	}
	if o.Actions.NoEvidence, err = db.actionList(ctx,
		`site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND s.priority = 'High' AND `+liveSite+` AND NOT EXISTS (SELECT 1 FROM evidence_captures e WHERE e.site_id = s.id AND e.status = 'done')`,
		`s.created_at`, `s.confidence DESC, s.created_at DESC`); err != nil {
		return nil, err
	}
	if o.Actions.Reappeared, err = db.overviewArtifacts(ctx, `
		HAVING BOOL_OR(NOT (`+liveSite+`)) AND BOOL_OR(`+liveSite+` AND sa.first_seen > NOW() - INTERVAL '30 days')
		ORDER BY MAX(sa.first_seen) DESC LIMIT 5`, ""); err != nil {
		return nil, err
	}
	if o.TopMoney, err = db.overviewArtifacts(ctx, `
		HAVING COUNT(DISTINCT sa.site_id) FILTER (WHERE `+liveSite+`) > 0
		ORDER BY 4 DESC, 5 DESC LIMIT 5`, `AND a.type IN ('Bank','E-Wallet','VirtualAccount','QRIS','Crypto')`); err != nil {
		return nil, err
	}

	rows, err := db.Pool.Query(ctx, `SELECT site_lifecycle(s.reach_status, s.status, s.offline_since), COUNT(*) FROM sites s WHERE site_is_judol(s.is_gambling, s.url, s.injection, s.market) GROUP BY 1`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var l string
		var n int
		rows.Scan(&l, &n)
		o.Lifecycle[l] = n
	}
	rows.Close()

	rows, err = db.Pool.Query(ctx, `
		WITH days AS (SELECT generate_series(CURRENT_DATE - 29, CURRENT_DATE, INTERVAL '1 day')::date AS d)
		SELECT to_char(d, 'YYYY-MM-DD'),
		       (SELECT COUNT(*) FROM sites s WHERE site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND s.created_at::date = d),
		       (SELECT COUNT(*) FROM sites s WHERE site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND NOT (`+liveSite+`) AND (`+diedAt+`)::date = d),
		       (SELECT COUNT(*) FROM sites s WHERE site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND s.blocked_since::date = d)
		FROM days ORDER BY d`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t TrendDay
		rows.Scan(&t.Day, &t.New, &t.Died, &t.Blocked)
		o.Trend = append(o.Trend, t)
	}
	rows.Close()

	if o.TopOps, err = db.overviewRanks(ctx, `
		SELECT o.id, COALESCE(NULLIF(o.name, ''), o.operator_key), o.platform,
		       COUNT(*) FILTER (WHERE `+liveSite+`), COUNT(*)
		FROM operators o JOIN site_operators so ON so.operator_id = o.id JOIN sites s ON s.id = so.site_id
		WHERE site_is_judol(s.is_gambling, s.url, s.injection, s.market) GROUP BY o.id HAVING COUNT(*) FILTER (WHERE `+liveSite+`) > 0
		ORDER BY 4 DESC, 5 DESC LIMIT 5`); err != nil {
		return nil, err
	}
	// Provider game dari scan lengkap terakhir tiap situs (hanya tersimpan di payload scan_logs)
	if o.TopProv, err = db.overviewRanks(ctx, `
		WITH last AS (
			SELECT DISTINCT ON (l.site_id) l.site_id, l.result->'result'->'network_signals'->'game_providers' AS gp
			FROM scan_logs l WHERE NOT (l.result ? 'error') ORDER BY l.site_id, l.created_at DESC)
		SELECT 0, p, '', COUNT(*) FILTER (WHERE `+liveSite+`), COUNT(*)
		FROM last JOIN sites s ON s.id = last.site_id,
		     LATERAL jsonb_array_elements_text(CASE WHEN jsonb_typeof(last.gp) = 'array' THEN last.gp ELSE '[]' END) p
		WHERE site_is_judol(s.is_gambling, s.url, s.injection, s.market) GROUP BY p ORDER BY 4 DESC, 5 DESC LIMIT 5`); err != nil {
		return nil, err
	}
	// IP CDN (Cloudflare dkk.) dipakai ribuan situs tak terkait — tidak informatif, jadi dikecualikan
	if o.TopHosts, err = db.overviewRanks(ctx, `
		SELECT 0, i.address, COALESCE(NULLIF(x.data->>'as_name', ''), x.data->>'org', ''),
		       COUNT(*) FILTER (WHERE `+liveSite+`), COUNT(*)
		FROM ips i JOIN site_ips si ON si.ip_id = i.id JOIN sites s ON s.id = si.site_id
		LEFT JOIN ip_intel x ON x.ip = i.address
		WHERE site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND COALESCE(x.data->>'cdn', '') = ''
		GROUP BY i.id, x.data HAVING COUNT(*) FILTER (WHERE `+liveSite+`) > 1
		ORDER BY 4 DESC, 5 DESC LIMIT 5`); err != nil {
		return nil, err
	}
	return o, nil
}

func (db *PostgresDB) overviewArtifacts(ctx context.Context, tail, where string) ([]OverviewArtifact, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT a.type, COALESCE(a.name, ''), a.value,
		       COUNT(DISTINCT sa.site_id) FILTER (WHERE `+liveSite+`), COUNT(DISTINCT sa.site_id)
		FROM artifacts a JOIN site_artifacts sa ON sa.artifact_id = a.id JOIN sites s ON s.id = sa.site_id
		WHERE site_is_judol(s.is_gambling, s.url, s.injection, s.market) `+where+` GROUP BY a.id `+tail)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OverviewArtifact{}
	for rows.Next() {
		var a OverviewArtifact
		if err := rows.Scan(&a.Type, &a.Name, &a.Value, &a.ActiveSites, &a.TotalSites); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (db *PostgresDB) overviewRanks(ctx context.Context, sql string) ([]OverviewRank, error) {
	rows, err := db.Pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OverviewRank{}
	for rows.Next() {
		var r OverviewRank
		if err := rows.Scan(&r.ID, &r.Label, &r.Sub, &r.ActiveSites, &r.TotalSites); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
