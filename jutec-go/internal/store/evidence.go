package store

import (
	"context"
	"time"
)

type EvidenceFile struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size_bytes"`
}

type EvidenceCapture struct {
	ID             int            `json:"id"`
	SiteID         int            `json:"site_id"`
	RequestedURL   string         `json:"requested_url"`
	FinalURL       string         `json:"final_url"`
	HTTPStatus     int            `json:"http_status"`
	ServerIP       string         `json:"server_ip"`
	Redirects      []string       `json:"redirects"`
	UserAgent      string         `json:"user_agent"`
	Dir            string         `json:"-"`
	ManifestSHA256 string         `json:"manifest_sha256"`
	Status         string         `json:"status"`
	Error          string         `json:"error,omitempty"`
	RequestedBy    string         `json:"requested_by"`
	CapturedAt     *time.Time     `json:"captured_at"`
	CreatedAt      time.Time      `json:"created_at"`
	Files          []EvidenceFile `json:"files"`
}

// CreateEvidenceCapture mencatat permintaan pengambilan bukti (status pending) dan mengembalikan id-nya.
func (db *PostgresDB) CreateEvidenceCapture(ctx context.Context, siteID int, url, dir, requestedBy string) (int, error) {
	var id int
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO evidence_captures (site_id, requested_url, dir, requested_by) VALUES ($1,$2,$3,$4) RETURNING id`,
		siteID, url, dir, requestedBy).Scan(&id)
	return id, err
}

func (db *PostgresDB) SetEvidenceDir(ctx context.Context, id int, dir string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE evidence_captures SET dir = $2 WHERE id = $1 AND status = 'pending'`, id, dir)
	return err
}

func (db *PostgresDB) CompleteEvidenceCapture(ctx context.Context, id int, finalURL string, status int, ip string, redirects []string,
	ua, manifestSHA string, capturedAt time.Time, files []EvidenceFile) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if redirects == nil {
		redirects = []string{}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE evidence_captures SET final_url=$2, http_status=$3, server_ip=$4, redirects=$5, user_agent=$6,
		       manifest_sha256=$7, captured_at=$8, status='done' WHERE id=$1 AND status='pending'`,
		id, finalURL, status, ip, redirects, ua, manifestSHA, capturedAt); err != nil {
		return err
	}
	for _, f := range files {
		if _, err := tx.Exec(ctx, `INSERT INTO evidence_files (capture_id, name, kind, sha256, size_bytes) VALUES ($1,$2,$3,$4,$5)`,
			id, f.Name, f.Kind, f.SHA256, f.Size); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (db *PostgresDB) FailEvidenceCapture(ctx context.Context, id int, msg string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE evidence_captures SET status='error', error=$2 WHERE id=$1 AND status='pending'`, id, msg)
	return err
}

func (db *PostgresDB) scanCaptures(ctx context.Context, where string, arg any) ([]EvidenceCapture, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, site_id, requested_url, COALESCE(final_url,''), COALESCE(http_status,0), COALESCE(server_ip,''), redirects,
		       COALESCE(user_agent,''), dir, COALESCE(manifest_sha256,''), status, COALESCE(error,''), requested_by, captured_at, created_at
		FROM evidence_captures WHERE `+where+` ORDER BY created_at DESC LIMIT 50`, arg)
	if err != nil {
		return nil, err
	}
	out := []EvidenceCapture{}
	for rows.Next() {
		var c EvidenceCapture
		if err := rows.Scan(&c.ID, &c.SiteID, &c.RequestedURL, &c.FinalURL, &c.HTTPStatus, &c.ServerIP, &c.Redirects, &c.UserAgent,
			&c.Dir, &c.ManifestSHA256, &c.Status, &c.Error, &c.RequestedBy, &c.CapturedAt, &c.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		c.Files = []EvidenceFile{}
		out = append(out, c)
	}
	rows.Close()
	for i := range out {
		fr, err := db.Pool.Query(ctx, `SELECT name, kind, sha256, size_bytes FROM evidence_files WHERE capture_id=$1 ORDER BY id`, out[i].ID)
		if err != nil {
			return nil, err
		}
		for fr.Next() {
			var f EvidenceFile
			if fr.Scan(&f.Name, &f.Kind, &f.SHA256, &f.Size) == nil {
				out[i].Files = append(out[i].Files, f)
			}
		}
		fr.Close()
	}
	return out, nil
}

func (db *PostgresDB) ListEvidence(ctx context.Context, siteID int) ([]EvidenceCapture, error) {
	return db.scanCaptures(ctx, "site_id = $1", siteID)
}

func (db *PostgresDB) GetEvidence(ctx context.Context, id int) (*EvidenceCapture, error) {
	l, err := db.scanCaptures(ctx, "id = $1", id)
	if err != nil || len(l) == 0 {
		return nil, err
	}
	return &l[0], nil
}
