CREATE TABLE IF NOT EXISTS operators (
    id            SERIAL PRIMARY KEY,
    name          TEXT,
    platform      TEXT NOT NULL DEFAULT 'unknown',
    operator_key  TEXT NOT NULL,
    site_count    INTEGER DEFAULT 0,
    avg_confidence FLOAT DEFAULT 0,
    discovered_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(platform, operator_key)
);

CREATE TABLE IF NOT EXISTS site_operators (
    site_id     INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    operator_id INTEGER REFERENCES operators(id) ON DELETE CASCADE,
    PRIMARY KEY (site_id, operator_id)
);

CREATE INDEX IF NOT EXISTS idx_operators_platform   ON operators(platform);
CREATE INDEX IF NOT EXISTS idx_operators_site_count ON operators(site_count DESC);
CREATE INDEX IF NOT EXISTS idx_site_operators_site  ON site_operators(site_id);
CREATE INDEX IF NOT EXISTS idx_site_operators_op    ON site_operators(operator_id);
