package store

import (
	"context"
	"time"
)

type SweepJob struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	Keywords   []string   `json:"keywords"`
	Total      int        `json:"total"`
	Processed  int        `json:"processed"`
	Found      int        `json:"found"`
	ErrorMsg   string     `json:"error_msg,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

func (db *PostgresDB) CreateSweepJob(ctx context.Context, id string, keywords []string, config map[string]any) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO sweep_jobs (id, status, keywords, config)
		VALUES ($1, 'running', $2, $3)
	`, id, keywords, config)
	return err
}

func (db *PostgresDB) GetSweepJob(ctx context.Context, id string) (*SweepJob, error) {
	var j SweepJob
	err := db.Pool.QueryRow(ctx, `
		SELECT id, status, keywords, total, processed, found,
		       COALESCE(error_msg,''), started_at, updated_at, finished_at
		FROM sweep_jobs WHERE id = $1
	`, id).Scan(&j.ID, &j.Status, &j.Keywords, &j.Total, &j.Processed, &j.Found,
		&j.ErrorMsg, &j.StartedAt, &j.UpdatedAt, &j.FinishedAt)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (db *PostgresDB) UpdateSweepJobStatus(ctx context.Context, id, status string) error {
	_, err := db.Pool.Exec(ctx, `
		UPDATE sweep_jobs SET status = $2, updated_at = NOW() WHERE id = $1
	`, id, status)
	return err
}

func (db *PostgresDB) UpdateSweepJobProgress(ctx context.Context, id string, processed, found, total int) error {
	_, err := db.Pool.Exec(ctx, `
		UPDATE sweep_jobs SET processed = $2, found = $3, total = $4, updated_at = NOW()
		WHERE id = $1
	`, id, processed, found, total)
	return err
}

func (db *PostgresDB) FinishSweepJob(ctx context.Context, id, status, errMsg string) error {
	_, err := db.Pool.Exec(ctx, `
		UPDATE sweep_jobs
		SET status = $2, error_msg = $3, finished_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, id, status, errMsg)
	return err
}

func (db *PostgresDB) GetActiveSweepJob(ctx context.Context) (*SweepJob, error) {
	var j SweepJob
	err := db.Pool.QueryRow(ctx, `
		SELECT id, status, keywords, total, processed, found,
		       COALESCE(error_msg,''), started_at, updated_at, finished_at
		FROM sweep_jobs
		WHERE status IN ('running','paused')
		ORDER BY started_at DESC LIMIT 1
	`).Scan(&j.ID, &j.Status, &j.Keywords, &j.Total, &j.Processed, &j.Found,
		&j.ErrorMsg, &j.StartedAt, &j.UpdatedAt, &j.FinishedAt)
	if err != nil {
		return nil, err
	}
	return &j, nil
}
