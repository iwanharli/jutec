-- Verifikasi manual status judol oleh penyidik. Keputusan manusia mengunci is_gambling:
-- scan ulang, OCR, dan rescore tidak boleh menimpanya.
ALTER TABLE sites ADD COLUMN IF NOT EXISTS verified_status TEXT CHECK (verified_status IN ('judol', 'bukan_judol'));
ALTER TABLE sites ADD COLUMN IF NOT EXISTS verified_by TEXT;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS verified_note TEXT;
CREATE INDEX IF NOT EXISTS idx_sites_verify_queue ON sites (confidence DESC) WHERE is_gambling AND verified_status IS NULL;

-- Riwayat setiap keputusan (tambah-saja)
CREATE TABLE IF NOT EXISTS site_verifications (
    id       BIGSERIAL PRIMARY KEY,
    site_id  INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    status   TEXT NOT NULL,          -- judol | bukan_judol | reset
    note     TEXT,
    actor    TEXT NOT NULL,
    prev_is_gambling BOOLEAN,
    prev_confidence  DOUBLE PRECISION,
    at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_site_verifications_site ON site_verifications(site_id, at DESC);
DROP TRIGGER IF EXISTS site_verifications_append_only ON site_verifications;
CREATE TRIGGER site_verifications_append_only BEFORE UPDATE OR DELETE ON site_verifications
  FOR EACH ROW WHEN (current_setting('jutec.allow_cascade', true) IS DISTINCT FROM 'on')
  EXECUTE FUNCTION jutec_forbid_change();
