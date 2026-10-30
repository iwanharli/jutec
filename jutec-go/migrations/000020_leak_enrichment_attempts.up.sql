CREATE TABLE IF NOT EXISTS leak_enrichment_attempts (
    artifact_id    INTEGER PRIMARY KEY REFERENCES artifacts(id) ON DELETE CASCADE,
    phone          TEXT NOT NULL,
    status         TEXT NOT NULL,
    error_message  TEXT,
    attempted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_leak_attempts_phone ON leak_enrichment_attempts(phone);
CREATE INDEX IF NOT EXISTS idx_leak_attempts_status ON leak_enrichment_attempts(status);
