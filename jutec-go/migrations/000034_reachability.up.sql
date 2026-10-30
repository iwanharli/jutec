-- Pemantauan keterjangkauan & blokir ISP situs judol
ALTER TABLE sites ADD COLUMN IF NOT EXISTS reach_status TEXT;        -- online | blocked_alive | blocked | offline | nxdomain
ALTER TABLE sites ADD COLUMN IF NOT EXISTS reach_checked_at TIMESTAMPTZ;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS blocked_since TIMESTAMPTZ; -- pertama kali terdeteksi diblokir DNS ISP
CREATE INDEX IF NOT EXISTS idx_sites_reach_due ON sites (reach_checked_at NULLS FIRST) WHERE is_gambling;

-- Riwayat: satu baris tiap kali status berubah
CREATE TABLE IF NOT EXISTS site_reachability (
    id          BIGSERIAL PRIMARY KEY,
    site_id     INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    status      TEXT NOT NULL,
    http_status INT,
    isp_blocked BOOLEAN NOT NULL DEFAULT false,
    nxdomain    BOOLEAN NOT NULL DEFAULT false,
    error       TEXT,
    checked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_site_reachability_site ON site_reachability(site_id, checked_at DESC);
