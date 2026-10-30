-- Cache intel IP (RDAP, ASN, lokasi, port pasif) & hasil pencarian IP asli per situs
CREATE TABLE IF NOT EXISTS ip_intel (
    ip         TEXT PRIMARY KEY,
    data       JSONB NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS site_origin (
    site_id    INTEGER PRIMARY KEY REFERENCES sites(id) ON DELETE CASCADE,
    data       JSONB NOT NULL,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
