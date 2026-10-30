-- Site monitoring columns
ALTER TABLE sites ADD COLUMN IF NOT EXISTS rescan_count  INTEGER DEFAULT 0;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS offline_since TIMESTAMPTZ;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS next_scan_at  TIMESTAMPTZ;

-- WHOIS data persistence (unique per site, upserted on each scan)
CREATE TABLE IF NOT EXISTS whois_records (
    id               SERIAL PRIMARY KEY,
    site_id          INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    domain           TEXT    NOT NULL,
    registrar        TEXT,
    registrant_name  TEXT,
    registrant_email TEXT,
    registrant_org   TEXT,
    registered_at    TIMESTAMPTZ,
    expires_at       TIMESTAMPTZ,
    domain_age_days  INTEGER DEFAULT -1,
    is_private       BOOLEAN DEFAULT FALSE,
    tld              TEXT,
    risk_score       FLOAT   DEFAULT 0,
    scanned_at       TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(site_id)
);

CREATE INDEX IF NOT EXISTS idx_whois_registrar       ON whois_records(registrar);
CREATE INDEX IF NOT EXISTS idx_whois_registrant_org  ON whois_records(registrant_org);
CREATE INDEX IF NOT EXISTS idx_whois_registrant_email ON whois_records(registrant_email);
CREATE INDEX IF NOT EXISTS idx_whois_tld             ON whois_records(tld);
CREATE INDEX IF NOT EXISTS idx_whois_risk_score      ON whois_records(risk_score DESC);
CREATE INDEX IF NOT EXISTS idx_sites_next_scan       ON sites(next_scan_at);
CREATE INDEX IF NOT EXISTS idx_sites_last_scanned    ON sites(last_scanned_at);
