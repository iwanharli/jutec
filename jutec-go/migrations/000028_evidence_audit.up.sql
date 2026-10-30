-- Bukti forensik per pengambilan: file disimpan di disk (tulis-sekali), hash SHA-256 di sini.
CREATE TABLE IF NOT EXISTS evidence_captures (
    id            SERIAL PRIMARY KEY,
    site_id       INTEGER NOT NULL REFERENCES sites(id) ON DELETE RESTRICT,
    requested_url TEXT NOT NULL,
    final_url     TEXT,
    http_status   INT,
    server_ip     TEXT,
    redirects     TEXT[] NOT NULL DEFAULT '{}',
    user_agent    TEXT,
    dir           TEXT NOT NULL,               -- folder penyimpanan relatif terhadap EVIDENCE_DIR
    manifest_sha256 TEXT,                      -- hash manifest.json (mengikat semua file)
    status        TEXT NOT NULL DEFAULT 'pending', -- pending | done | error
    error         TEXT,
    requested_by  TEXT NOT NULL DEFAULT 'anonymous',
    captured_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_evidence_site ON evidence_captures(site_id, created_at DESC);

CREATE TABLE IF NOT EXISTS evidence_files (
    id          SERIAL PRIMARY KEY,
    capture_id  INTEGER NOT NULL REFERENCES evidence_captures(id) ON DELETE RESTRICT,
    name        TEXT NOT NULL,       -- page.html, screenshot.png, mobile.png, network.har, manifest.json
    kind        TEXT NOT NULL,       -- html | screenshot | har | manifest
    sha256      TEXT NOT NULL,
    size_bytes  BIGINT NOT NULL,
    UNIQUE (capture_id, name)
);

-- Log audit berantai hash: hash = sha256(prev_hash || isi baris). Mengubah/menghapus baris lama memutus rantai.
CREATE TABLE IF NOT EXISTS audit_log (
    id          BIGSERIAL PRIMARY KEY,
    at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor       TEXT NOT NULL,
    ip          TEXT,
    action      TEXT NOT NULL,       -- mis. "POST /sites/:id/evidence"
    target      TEXT,                -- mis. "site:5736"
    detail      JSONB,
    status      INT,
    prev_hash   TEXT NOT NULL,
    hash        TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_at ON audit_log(at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_target ON audit_log(target);

-- Larang UPDATE/DELETE pada log audit & catatan bukti di level database
CREATE OR REPLACE FUNCTION jutec_forbid_change() RETURNS trigger AS $$
BEGIN RAISE EXCEPTION 'tabel % bersifat tambah-saja (append-only)', TG_TABLE_NAME; END $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS audit_log_append_only ON audit_log;
CREATE TRIGGER audit_log_append_only BEFORE UPDATE OR DELETE ON audit_log FOR EACH ROW EXECUTE FUNCTION jutec_forbid_change();
DROP TRIGGER IF EXISTS evidence_files_append_only ON evidence_files;
CREATE TRIGGER evidence_files_append_only BEFORE UPDATE OR DELETE ON evidence_files FOR EACH ROW EXECUTE FUNCTION jutec_forbid_change();
