-- Penanda proses otomatis: kapan arsip Wayback situs terakhir diambil (termasuk bila hasilnya kosong)
ALTER TABLE sites ADD COLUMN IF NOT EXISTS wayback_checked_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_site_artifacts_first_seen ON site_artifacts(site_id, first_seen);
