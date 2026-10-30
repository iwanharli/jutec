CREATE TABLE IF NOT EXISTS sites (
    id              SERIAL PRIMARY KEY,
    url             TEXT UNIQUE NOT NULL,
    is_gambling     BOOLEAN     DEFAULT FALSE,
    priority        TEXT,
    confidence      FLOAT,
    last_scanned_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    created_at      TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS scan_logs (
    id         SERIAL PRIMARY KEY,
    site_id    INTEGER REFERENCES sites(id),
    url        TEXT    NOT NULL,
    result     JSONB   NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sites_url        ON sites (url);
CREATE INDEX IF NOT EXISTS idx_scan_logs_result ON scan_logs USING GIN (result);
