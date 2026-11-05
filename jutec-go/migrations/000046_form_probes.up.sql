-- Uji Form Terkendali: pendaftaran + login dengan data dummy milik unit (atas perintah analis)
CREATE TABLE IF NOT EXISTS form_probes (
    id           SERIAL PRIMARY KEY,
    site_id      INTEGER NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    page_url     TEXT NOT NULL,
    username     TEXT,
    status       TEXT NOT NULL DEFAULT 'pending', -- pending | submitted | skipped | error
    outcome      TEXT,                            -- accepted | rejected | unknown
    login_status TEXT,                            -- logged_in | failed | skipped
    reason       TEXT,
    result       JSONB,
    dir          TEXT,
    files        JSONB,                           -- [{name, sha256, size}]
    requested_by TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_form_probes_site ON form_probes (site_id, created_at DESC);

INSERT INTO settings (key, value, description) VALUES
 ('probe_username_prefix', '', 'Uji form: awalan username dummy (mis. "jt"); username dibuat unik per uji: awalan + 6 angka'),
 ('probe_password', '', 'Uji form: password akun dummy'),
 ('probe_fullname', '', 'Uji form: nama lengkap dummy'),
 ('probe_email', '', 'Uji form: email dummy milik unit'),
 ('probe_phone', '', 'Uji form: nomor HP/WA dummy (SIM khusus investigasi milik unit)'),
 ('probe_bank', '', 'Uji form: nama bank/e-wallet rekening uji (mis. BCA, DANA)'),
 ('probe_account_name', '', 'Uji form: nama pemilik rekening uji'),
 ('probe_account_number', '', 'Uji form: nomor rekening uji milik unit — jangan memakai nomor karangan')
ON CONFLICT (key) DO NOTHING;
