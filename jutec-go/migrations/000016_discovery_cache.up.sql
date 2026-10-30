CREATE TABLE IF NOT EXISTS discovery_cache (
    id           SERIAL PRIMARY KEY,
    source       TEXT NOT NULL,        -- 'serper', 'crtsh'
    query        TEXT NOT NULL,        -- keyword atau domain pattern
    urls         TEXT[] NOT NULL,      -- hasil URL yang ditemukan
    result_count INTEGER DEFAULT 0,
    discovered_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL,
    UNIQUE(source, query)
);

CREATE INDEX IF NOT EXISTS idx_discovery_cache_lookup ON discovery_cache(source, query, expires_at);
CREATE INDEX IF NOT EXISTS idx_discovery_cache_expires ON discovery_cache(expires_at);
