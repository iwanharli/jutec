CREATE TABLE IF NOT EXISTS sweep_jobs (
    id          TEXT PRIMARY KEY,
    status      TEXT NOT NULL DEFAULT 'running', -- running, paused, done, error, cancelled
    keywords    TEXT[] NOT NULL DEFAULT '{}',
    config      JSONB NOT NULL DEFAULT '{}',
    total       INTEGER DEFAULT 0,
    processed   INTEGER DEFAULT 0,
    found       INTEGER DEFAULT 0,
    error_msg   TEXT,
    started_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sweep_jobs_status ON sweep_jobs(status);
