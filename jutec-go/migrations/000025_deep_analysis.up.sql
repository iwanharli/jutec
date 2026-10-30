-- Analisis statis APK (tanpa menjalankan): paket, sertifikat penanda tangan, URL/domain di dalam kode
CREATE TABLE IF NOT EXISTS apk_analyses (
    app_id          INTEGER PRIMARY KEY REFERENCES apps(id) ON DELETE CASCADE,
    sha256          TEXT,
    size_bytes      BIGINT,
    package_name    TEXT,
    version_name    TEXT,
    app_label       TEXT,
    cert_sha256     TEXT,
    cert_subject    TEXT,
    cert_issuer     TEXT,
    cert_not_before TIMESTAMPTZ,
    permissions     TEXT[] NOT NULL DEFAULT '{}',
    urls            TEXT[] NOT NULL DEFAULT '{}',
    domains         TEXT[] NOT NULL DEFAULT '{}',
    error           TEXT,
    analyzed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_apk_cert ON apk_analyses(cert_sha256);

-- Snapshot Wayback Machine: artefak/kontak lama yang mungkin sudah dihapus dari situs
CREATE TABLE IF NOT EXISTS wayback_snapshots (
    id          SERIAL PRIMARY KEY,
    site_id     INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    snapshot_ts TEXT NOT NULL,
    url         TEXT NOT NULL,
    artifacts   JSONB NOT NULL DEFAULT '[]',
    register    TEXT[] NOT NULL DEFAULT '{}',
    tracking    JSONB NOT NULL DEFAULT '[]',
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (site_id, snapshot_ts)
);

-- Bukti pengelompokan operator (atribut kuat yang dibagi antar situs dalam klaster)
ALTER TABLE operators ADD COLUMN IF NOT EXISTS evidence JSONB NOT NULL DEFAULT '[]';
