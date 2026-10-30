-- Riwayat intel IP: setiap versi yang berbeda disimpan (tidak pernah ditimpa/dihapus),
-- agar perubahan pemilik blok, ASN, port, atau hostname bisa ditelusuri dari waktu ke waktu.
CREATE TABLE IF NOT EXISTS ip_intel_history (
    id          BIGSERIAL PRIMARY KEY,
    ip          TEXT NOT NULL,
    data        JSONB NOT NULL,
    fingerprint TEXT NOT NULL,          -- hash isi yang relevan (tanpa waktu ambil)
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ip_intel_history_ip ON ip_intel_history(ip, fetched_at DESC);
DROP TRIGGER IF EXISTS ip_intel_history_append_only ON ip_intel_history;
CREATE TRIGGER ip_intel_history_append_only BEFORE UPDATE OR DELETE ON ip_intel_history
  FOR EACH ROW EXECUTE FUNCTION jutec_forbid_change();
-- Isi awal dari cache yang sudah ada
INSERT INTO ip_intel_history (ip, data, fingerprint, fetched_at)
SELECT ip, data, md5((data - 'fetched_at' - 'errors')::text), fetched_at FROM ip_intel;
