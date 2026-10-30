CREATE TABLE IF NOT EXISTS urlscan_results (
    id             SERIAL PRIMARY KEY,
    site_id        INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    scan_uuid      TEXT NOT NULL,
    verdict        TEXT    DEFAULT 'unknown', -- malicious, suspicious, benign, unknown
    malicious      BOOLEAN DEFAULT FALSE,
    score          INTEGER DEFAULT 0,
    screenshot_url TEXT,
    categories     TEXT[]  DEFAULT '{}',
    technologies   TEXT[]  DEFAULT '{}',
    scanned_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(site_id, scan_uuid)
);

CREATE TABLE IF NOT EXISTS virustotal_results (
    id               SERIAL PRIMARY KEY,
    site_id          INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    resource         TEXT NOT NULL,
    resource_type    TEXT NOT NULL DEFAULT 'url', -- url, domain, apk
    malicious_count  INTEGER DEFAULT 0,
    suspicious_count INTEGER DEFAULT 0,
    harmless_count   INTEGER DEFAULT 0,
    total_engines    INTEGER DEFAULT 0,
    categories       TEXT[]  DEFAULT '{}',
    permalink        TEXT,
    analysis_date    TIMESTAMPTZ,
    created_at       TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(site_id, resource, resource_type)
);

CREATE INDEX IF NOT EXISTS idx_urlscan_site    ON urlscan_results(site_id);
CREATE INDEX IF NOT EXISTS idx_urlscan_verdict ON urlscan_results(verdict);
CREATE INDEX IF NOT EXISTS idx_vt_site         ON virustotal_results(site_id);
CREATE INDEX IF NOT EXISTS idx_vt_malicious    ON virustotal_results(malicious_count);
