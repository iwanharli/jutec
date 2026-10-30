-- Hasil reverse lookup ID analytics (HackerTarget): domain lain di internet yang memakai ID yang sama.
-- Disimpan karena kuota API gratis kecil — satu ID cukup ditanya ulang sesekali.
CREATE TABLE IF NOT EXISTS reverse_analytics (
    tracking_value TEXT        NOT NULL,
    domain         TEXT        NOT NULL,
    fetched_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tracking_value, domain)
);
CREATE TABLE IF NOT EXISTS reverse_analytics_runs (
    tracking_value TEXT PRIMARY KEY,
    fetched_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    result_count   INTEGER     NOT NULL DEFAULT 0,
    error          TEXT
);

-- Profil transaksi alamat USDT-TRC20 dari TronGrid (cache agar tidak membebani rate limit)
CREATE TABLE IF NOT EXISTS crypto_profiles (
    address    TEXT PRIMARY KEY,
    profile    JSONB       NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
