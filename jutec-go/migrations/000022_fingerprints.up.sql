-- Sidik jari infrastruktur: ID distribusi CDN, hash favicon, host API backend.
-- Situs white-label satu platform hampir selalu berbagi penanda ini.
CREATE TABLE IF NOT EXISTS fingerprints (
    id         SERIAL PRIMARY KEY,
    kind       TEXT NOT NULL,           -- cdn | favicon | api
    value      TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (kind, value)
);

CREATE TABLE IF NOT EXISTS site_fingerprints (
    id             SERIAL PRIMARY KEY,
    site_id        INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    fingerprint_id INTEGER NOT NULL REFERENCES fingerprints(id) ON DELETE CASCADE,
    first_seen     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (site_id, fingerprint_id)
);

CREATE INDEX IF NOT EXISTS idx_site_fingerprints_site ON site_fingerprints(site_id);
CREATE INDEX IF NOT EXISTS idx_site_fingerprints_fp ON site_fingerprints(fingerprint_id);

-- Halaman internal yang ikut dianalisis pada scan terakhir
ALTER TABLE sites ADD COLUMN IF NOT EXISTS internal_pages TEXT[] NOT NULL DEFAULT '{}';
