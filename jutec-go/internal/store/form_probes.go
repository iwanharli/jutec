package store

import (
	"context"
	"encoding/json"
	"time"
)

type FormProbe struct {
	ID          int             `json:"id"`
	SiteID      int             `json:"site_id"`
	PageURL     string          `json:"page_url"`
	Username    string          `json:"username"`
	Status      string          `json:"status"`
	Outcome     string          `json:"outcome"`
	LoginStatus string          `json:"login_status"`
	Reason      string          `json:"reason"`
	Result      json.RawMessage `json:"result"`
	Dir         string          `json:"-"`
	Files       json.RawMessage `json:"files"`
	RequestedBy string          `json:"requested_by"`
	CreatedAt   time.Time       `json:"created_at"`
	FinishedAt  *time.Time      `json:"finished_at"`
}

// ProbeProfileSettings: data dummy (probe_*) dari Pengaturan.
func (db *PostgresDB) ProbeProfileSettings(ctx context.Context) map[string]string {
	out := map[string]string{}
	rows, err := db.Pool.Query(ctx, `SELECT key, value FROM settings WHERE key LIKE 'probe\_%'`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil {
			out[k] = v
		}
	}
	return out
}

func (db *PostgresDB) CreateFormProbe(ctx context.Context, siteID int, pageURL, username, by string) (int, error) {
	var id int
	err := db.Pool.QueryRow(ctx, `INSERT INTO form_probes (site_id, page_url, username, requested_by) VALUES ($1,$2,$3,$4) RETURNING id`,
		siteID, pageURL, username, by).Scan(&id)
	return id, err
}

func (db *PostgresDB) GetFormProbe(ctx context.Context, id int) (*FormProbe, error) {
	p := &FormProbe{}
	err := db.Pool.QueryRow(ctx, `SELECT id, site_id, page_url, COALESCE(username,''), status, COALESCE(outcome,''), COALESCE(login_status,''),
		COALESCE(reason,''), result, COALESCE(dir,''), files, COALESCE(requested_by,''), created_at, finished_at FROM form_probes WHERE id = $1`, id).
		Scan(&p.ID, &p.SiteID, &p.PageURL, &p.Username, &p.Status, &p.Outcome, &p.LoginStatus, &p.Reason, &p.Result, &p.Dir, &p.Files, &p.RequestedBy, &p.CreatedAt, &p.FinishedAt)
	return p, err
}

func (db *PostgresDB) FinishFormProbe(ctx context.Context, id int, status, outcome, loginStatus, reason, dir string, result, files any) error {
	_, err := db.Pool.Exec(ctx, `UPDATE form_probes SET status=$2, outcome=NULLIF($3,''), login_status=NULLIF($4,''), reason=NULLIF($5,''),
		dir=NULLIF($6,''), result=$7, files=$8, finished_at=NOW() WHERE id=$1`, id, status, outcome, loginStatus, reason, dir, result, files)
	return err
}

func (db *PostgresDB) ListFormProbes(ctx context.Context, siteID int) ([]FormProbe, error) {
	rows, err := db.Pool.Query(ctx, `SELECT id FROM form_probes WHERE site_id = $1 ORDER BY created_at DESC LIMIT 20`, siteID)
	if err != nil {
		return nil, err
	}
	var ids []int
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	out := []FormProbe{}
	for _, id := range ids {
		if p, err := db.GetFormProbe(ctx, id); err == nil {
			out = append(out, *p)
		}
	}
	return out, nil
}

type ProbeAccount struct {
	ProbeID     int             `json:"probe_id"`
	SiteID      int             `json:"site_id"`
	URL         string          `json:"url"`
	Category    string          `json:"category"`
	Lifecycle   string          `json:"lifecycle"`
	PageURL     string          `json:"page_url"`
	Username    string          `json:"username"`
	Outcome     string          `json:"outcome"`
	LoginStatus string          `json:"login_status"`
	Message     string          `json:"message"`
	Platform    json.RawMessage `json:"platform"`
	Artifacts   json.RawMessage `json:"artifacts"`   // rekening/kontak setelah daftar & login
	DepositURLs json.RawMessage `json:"deposit_urls"`
	RequestedBy string          `json:"requested_by"`
	CreatedAt   time.Time       `json:"created_at"`
}

// ProbeAccounts: akun dummy yang berhasil dibuat (pendaftaran diterima atau berhasil login), terbaru dulu.
func (db *PostgresDB) ProbeAccounts(ctx context.Context, q string) ([]ProbeAccount, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT p.id, s.id, s.url, `+siteCategorySQL+`, site_lifecycle(s.reach_status, s.status, s.offline_since), p.page_url,
		       COALESCE(p.username,''), COALESCE(p.outcome,''), COALESCE(p.login_status,''),
		       COALESCE(p.result->>'message',''), COALESCE(p.result->'platform','[]'),
		       COALESCE(p.result->'artifacts','[]') || COALESCE(p.result->'login'->'artifacts','[]'),
		       COALESCE(p.result->'login'->'deposit_urls','[]'), COALESCE(p.requested_by,''), p.created_at
		FROM form_probes p JOIN sites s ON s.id = p.site_id
		WHERE p.status = 'submitted' AND (p.outcome = 'accepted' OR p.login_status = 'logged_in')
		  AND ($1 = '' OR s.url ILIKE '%' || $1 || '%' OR p.username ILIKE '%' || $1 || '%')
		ORDER BY p.created_at DESC LIMIT 500`, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProbeAccount{}
	for rows.Next() {
		var a ProbeAccount
		if err := rows.Scan(&a.ProbeID, &a.SiteID, &a.URL, &a.Category, &a.Lifecycle, &a.PageURL, &a.Username, &a.Outcome, &a.LoginStatus,
			&a.Message, &a.Platform, &a.Artifacts, &a.DepositURLs, &a.RequestedBy, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type ProbeProfileRow struct {
	ID             int             `json:"id"`
	Label          string          `json:"label"`
	UsernamePrefix string          `json:"username_prefix"`
	Password       string          `json:"password"`
	FullName       string          `json:"fullname"`
	Email          string          `json:"email"`
	Phone          string          `json:"phone"`
	Accounts       json.RawMessage `json:"accounts"`
	IsDefault      bool            `json:"is_default"`
	Uses           int             `json:"uses"`     // jumlah uji yang memakai identitas ini
	Accepted       int             `json:"accepted"` // akun yang diterima situs
	LastUsed       *time.Time      `json:"last_used"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

const probeProfileCols = `p.id, p.label, p.username_prefix, p.password, p.fullname, p.email, p.phone, p.accounts, p.is_default,
	(SELECT COUNT(*) FROM form_probes f WHERE f.profile_id = p.id),
	(SELECT COUNT(*) FROM form_probes f WHERE f.profile_id = p.id AND (f.outcome = 'accepted' OR f.login_status = 'logged_in')),
	(SELECT MAX(created_at) FROM form_probes f WHERE f.profile_id = p.id), p.updated_at`

func scanProbeProfile(row interface{ Scan(...any) error }) (*ProbeProfileRow, error) {
	p := &ProbeProfileRow{}
	err := row.Scan(&p.ID, &p.Label, &p.UsernamePrefix, &p.Password, &p.FullName, &p.Email, &p.Phone, &p.Accounts, &p.IsDefault, &p.Uses, &p.Accepted, &p.LastUsed, &p.UpdatedAt)
	return p, err
}

func (db *PostgresDB) ListProbeProfiles(ctx context.Context) ([]ProbeProfileRow, error) {
	rows, err := db.Pool.Query(ctx, `SELECT `+probeProfileCols+` FROM probe_profiles p ORDER BY p.is_default DESC, p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProbeProfileRow{}
	for rows.Next() {
		p, err := scanProbeProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// GetProbeProfile: id 0 = identitas bawaan (atau yang pertama).
func (db *PostgresDB) GetProbeProfile(ctx context.Context, id int) (*ProbeProfileRow, error) {
	return scanProbeProfile(db.Pool.QueryRow(ctx, `SELECT `+probeProfileCols+` FROM probe_profiles p
		WHERE ($1 = 0 OR p.id = $1) ORDER BY p.is_default DESC, p.id LIMIT 1`, id))
}

// SaveProbeProfile: buat (id 0) atau ubah. Identitas pertama otomatis jadi bawaan.
func (db *PostgresDB) SaveProbeProfile(ctx context.Context, p *ProbeProfileRow) (int, error) {
	if len(p.Accounts) == 0 {
		p.Accounts = json.RawMessage("[]")
	}
	if p.ID == 0 {
		err := db.Pool.QueryRow(ctx, `INSERT INTO probe_profiles (label, username_prefix, password, fullname, email, phone, accounts, is_default)
			VALUES ($1,$2,$3,$4,$5,$6,$7, NOT EXISTS (SELECT 1 FROM probe_profiles)) RETURNING id`,
			p.Label, p.UsernamePrefix, p.Password, p.FullName, p.Email, p.Phone, p.Accounts).Scan(&p.ID)
		return p.ID, err
	}
	_, err := db.Pool.Exec(ctx, `UPDATE probe_profiles SET label=$2, username_prefix=$3, password=$4, fullname=$5, email=$6, phone=$7,
		accounts=$8, updated_at=NOW() WHERE id=$1`, p.ID, p.Label, p.UsernamePrefix, p.Password, p.FullName, p.Email, p.Phone, p.Accounts)
	return p.ID, err
}

func (db *PostgresDB) SetDefaultProbeProfile(ctx context.Context, id int) error {
	_, err := db.Pool.Exec(ctx, `UPDATE probe_profiles SET is_default = (id = $1)`, id)
	return err
}

// DeleteProbeProfile: riwayat uji tetap ada (profile_id jadi NULL); bawaan pindah ke identitas lain.
func (db *PostgresDB) DeleteProbeProfile(ctx context.Context, id int) error {
	if _, err := db.Pool.Exec(ctx, `DELETE FROM probe_profiles WHERE id = $1`, id); err != nil {
		return err
	}
	_, err := db.Pool.Exec(ctx, `UPDATE probe_profiles SET is_default = true
		WHERE id = (SELECT id FROM probe_profiles ORDER BY id LIMIT 1) AND NOT EXISTS (SELECT 1 FROM probe_profiles WHERE is_default)`)
	return err
}

func (db *PostgresDB) SetFormProbeProfile(ctx context.Context, probeID, profileID int) {
	db.Pool.Exec(ctx, `UPDATE form_probes SET profile_id = $2 WHERE id = $1`, probeID, profileID)
}

func (db *PostgresDB) FormProbeProfileID(ctx context.Context, probeID int) int {
	var id *int
	db.Pool.QueryRow(ctx, `SELECT profile_id FROM form_probes WHERE id = $1`, probeID).Scan(&id)
	if id == nil {
		return 0
	}
	return *id
}

// BatchProbeTarget: satu situs kandidat uji massal + form daftar yang bisa diotomasi.
type BatchProbeTarget struct {
	SiteID  int    `json:"site_id"`
	URL     string `json:"url"`
	PageURL string `json:"page_url"`
	Action  string `json:"action"`
	Login   string `json:"login"`
}

// BatchProbeCandidates: situs judol valid dengan form daftar tanpa captcha/OTP, yang BELUM pernah diuji
// profil ini (satu profil mendaftar maksimal sekali per situs; profil lain tetap boleh).
// Pemilihan form dilakukan pemanggil (PickRegisterForm) karena ada di JSON auth_surface.
func (db *PostgresDB) BatchProbeCandidates(ctx context.Context, profileID int) ([]struct {
	ID   int
	URL  string
	Auth json.RawMessage
}, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT s.id, s.url, s.auth_surface FROM sites s
		WHERE s.auth_surface IS NOT NULL
		  AND (s.verified_status = 'judol' OR (COALESCE(s.verified_status,'') <> 'bukan_judol'
		       AND site_is_judol(s.is_gambling, s.url, s.injection, s.market) AND `+StrongEvidenceSQL+`))
		  AND site_lifecycle(s.reach_status, s.status, s.offline_since) <> 'inactive'
		  AND NOT EXISTS (SELECT 1 FROM form_probes p WHERE p.site_id = s.id AND p.profile_id = $1
		                  AND (p.outcome = 'accepted' OR p.login_status = 'logged_in'))
		ORDER BY s.confidence DESC NULLS LAST`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		ID   int
		URL  string
		Auth json.RawMessage
	}
	for rows.Next() {
		var r struct {
			ID   int
			URL  string
			Auth json.RawMessage
		}
		if err := rows.Scan(&r.ID, &r.URL, &r.Auth); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PickRegisterForm memilih form daftar yang bisa diotomasi (tanpa captcha/OTP) + URL login dari auth_surface.
// pageURL kosong = tidak ada form yang bisa diuji.
// PickAnyRegisterForm memilih form daftar APA PUN (termasuk bercaptcha/OTP) — untuk Mode Pantau,
// karena analis yang menyelesaikan captcha. hasCaptcha menandakan form perlu diselesaikan manual.
func PickAnyRegisterForm(auth json.RawMessage) (pageURL, action, login string, hasCaptcha bool) {
	var as struct {
		LoginURLs []string `json:"login_urls"`
		Forms     []struct {
			Type       string `json:"type"`
			PageURL    string `json:"page_url"`
			Action     string `json:"action"`
			HasCaptcha bool   `json:"has_captcha"`
			HasOTP     bool   `json:"has_otp"`
		} `json:"forms"`
	}
	json.Unmarshal(auth, &as)
	for _, f := range as.Forms {
		if f.Type == "login" && login == "" {
			login = f.PageURL
		}
		if f.Type != "register" || pageURL != "" {
			continue
		}
		pageURL, action, hasCaptcha = f.PageURL, f.Action, f.HasCaptcha || f.HasOTP
	}
	if login == "" && len(as.LoginURLs) > 0 {
		login = as.LoginURLs[0]
	}
	return
}

func PickRegisterForm(auth json.RawMessage) (pageURL, action, login string, skippedCaptcha bool) {
	var as struct {
		LoginURLs []string `json:"login_urls"`
		Forms     []struct {
			Type       string `json:"type"`
			PageURL    string `json:"page_url"`
			Action     string `json:"action"`
			HasCaptcha bool   `json:"has_captcha"`
			HasOTP     bool   `json:"has_otp"`
		} `json:"forms"`
	}
	json.Unmarshal(auth, &as)
	for _, f := range as.Forms {
		if f.Type == "login" && login == "" {
			login = f.PageURL
		}
		if f.Type != "register" || pageURL != "" {
			continue
		}
		if f.HasCaptcha || f.HasOTP {
			skippedCaptcha = true
			continue
		}
		pageURL, action = f.PageURL, f.Action
	}
	if login == "" && len(as.LoginURLs) > 0 {
		login = as.LoginURLs[0]
	}
	return
}

type BatchRun struct {
	ID        int       `json:"id"`
	ProfileID int       `json:"profile_id"`
	Total     int       `json:"total"`
	DelaySec  int       `json:"delay_sec"`
	Status    string    `json:"status"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Done      int       `json:"done"`
	Submitted int       `json:"submitted"`
	Accepted  int       `json:"accepted"`
	Skipped   int       `json:"skipped"`
	Failed    int       `json:"failed"`
	OTP       int       `json:"otp"`
}

func (db *PostgresDB) CreateBatch(ctx context.Context, profileID, total, delaySec int, by string) (int, error) {
	var id int
	err := db.Pool.QueryRow(ctx, `INSERT INTO form_probe_batches (profile_id, total, delay_sec, created_by) VALUES (NULLIF($1,0),$2,$3,$4) RETURNING id`,
		profileID, total, delaySec, by).Scan(&id)
	return id, err
}

func (db *PostgresDB) CreateBatchProbe(ctx context.Context, batchID, siteID, profileID int, pageURL, username, by string) (int, error) {
	var id int
	err := db.Pool.QueryRow(ctx, `INSERT INTO form_probes (site_id, page_url, username, requested_by, profile_id, batch_id)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, siteID, pageURL, username, by, profileID, batchID).Scan(&id)
	return id, err
}

func (db *PostgresDB) BatchStatus(ctx context.Context, id int) (*BatchRun, error) {
	b := &BatchRun{}
	err := db.Pool.QueryRow(ctx, `SELECT id, COALESCE(profile_id,0), total, delay_sec, status, COALESCE(created_by,''), created_at FROM form_probe_batches WHERE id=$1`, id).
		Scan(&b.ID, &b.ProfileID, &b.Total, &b.DelaySec, &b.Status, &b.CreatedBy, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	db.Pool.QueryRow(ctx, `SELECT
		COUNT(*) FILTER (WHERE status <> 'pending'),
		COUNT(*) FILTER (WHERE status = 'submitted'),
		COUNT(*) FILTER (WHERE outcome = 'accepted' OR login_status = 'logged_in'),
		COUNT(*) FILTER (WHERE status = 'skipped'),
		COUNT(*) FILTER (WHERE status = 'error'),
		COUNT(*) FILTER (WHERE outcome = 'otp_required')
		FROM form_probes WHERE batch_id = $1`, id).Scan(&b.Done, &b.Submitted, &b.Accepted, &b.Skipped, &b.Failed, &b.OTP)
	// Uji yang menggantung > 10 menit (situs lambat/task mati) ditandai gagal agar batch tidak stuck
	if b.Status == "running" {
		db.Pool.Exec(ctx, `UPDATE form_probes SET status='error', reason='timeout — tidak selesai diproses', finished_at=NOW()
			WHERE batch_id=$1 AND status='pending' AND created_at < NOW() - INTERVAL '10 minutes'`, id)
		db.Pool.QueryRow(ctx, `SELECT
			COUNT(*) FILTER (WHERE status <> 'pending'),
			COUNT(*) FILTER (WHERE status = 'submitted'),
			COUNT(*) FILTER (WHERE outcome = 'accepted' OR login_status = 'logged_in'),
			COUNT(*) FILTER (WHERE status = 'skipped'),
			COUNT(*) FILTER (WHERE status = 'error')
			FROM form_probes WHERE batch_id = $1`, id).Scan(&b.Done, &b.Submitted, &b.Accepted, &b.Skipped, &b.Failed)
	}
	// Tandai selesai bila semua sudah diproses
	if b.Status == "running" && b.Done >= b.Total {
		db.Pool.Exec(ctx, `UPDATE form_probe_batches SET status='done' WHERE id=$1 AND status='running'`, id)
		b.Status = "done"
	}
	return b, nil
}

func (db *PostgresDB) LatestBatch(ctx context.Context) (*BatchRun, error) {
	var id int
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM form_probe_batches ORDER BY id DESC LIMIT 1`).Scan(&id); err != nil {
		return nil, err
	}
	return db.BatchStatus(ctx, id)
}

func (db *PostgresDB) StopBatch(ctx context.Context, id int) error {
	_, err := db.Pool.Exec(ctx, `UPDATE form_probe_batches SET status='stopped' WHERE id=$1 AND status='running'`, id)
	return err
}

// BatchStopped: dipakai task untuk melewati sisa uji bila batch dihentikan.
func (db *PostgresDB) BatchStopped(ctx context.Context, batchID int) bool {
	if batchID == 0 {
		return false
	}
	var st string
	db.Pool.QueryRow(ctx, `SELECT status FROM form_probe_batches WHERE id=$1`, batchID).Scan(&st)
	return st == "stopped"
}

func (db *PostgresDB) FormProbeBatchID(ctx context.Context, probeID int) int {
	var id *int
	db.Pool.QueryRow(ctx, `SELECT batch_id FROM form_probes WHERE id=$1`, probeID).Scan(&id)
	if id == nil {
		return 0
	}
	return *id
}

// ProfileTestedSite: profil ini sudah pernah berhasil/terkirim menguji situs ini (bukan status error).
func (db *PostgresDB) ProfileTestedSite(ctx context.Context, profileID, siteID int) bool {
	var n int
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM form_probes WHERE profile_id=$1 AND site_id=$2 AND status <> 'error'`, profileID, siteID).Scan(&n)
	return n > 0
}

// ProfileProbedFormHosts: host halaman pendaftaran yang sudah pernah diuji profil ini (bukan status error).
// Banyak domain judol berbagi satu backend daftar — dedup per host, bukan per situs.
func (db *PostgresDB) ProfileProbedFormHosts(ctx context.Context, profileID int) map[string]bool {
	out := map[string]bool{}
	rows, err := db.Pool.Query(ctx, `SELECT DISTINCT page_url FROM form_probes
		WHERE profile_id=$1 AND (outcome = 'accepted' OR login_status = 'logged_in')`, profileID)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var u string
		if rows.Scan(&u) == nil {
			out[hostOf(u)] = true
		}
	}
	return out
}
