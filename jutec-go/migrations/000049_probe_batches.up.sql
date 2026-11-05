CREATE TABLE IF NOT EXISTS form_probe_batches (
    id         SERIAL PRIMARY KEY,
    profile_id INTEGER REFERENCES probe_profiles(id) ON DELETE SET NULL,
    total      INTEGER NOT NULL DEFAULT 0,
    delay_sec  INTEGER NOT NULL DEFAULT 25,
    status     TEXT NOT NULL DEFAULT 'running', -- running | done | stopped
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE form_probes ADD COLUMN IF NOT EXISTS batch_id INTEGER REFERENCES form_probe_batches(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_form_probes_batch ON form_probes (batch_id);
