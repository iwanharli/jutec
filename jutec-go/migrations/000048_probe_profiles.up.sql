-- Identitas dummy tersimpan untuk Uji Form (sebelumnya satu profil di settings probe_*)
CREATE TABLE IF NOT EXISTS probe_profiles (
    id              SERIAL PRIMARY KEY,
    label           TEXT NOT NULL,
    username_prefix TEXT NOT NULL,
    password        TEXT NOT NULL,
    fullname        TEXT NOT NULL DEFAULT '',
    email           TEXT NOT NULL DEFAULT '',
    phone           TEXT NOT NULL DEFAULT '',
    accounts        JSONB NOT NULL DEFAULT '[]', -- rekening uji per bank [{bank, number, name}]
    is_default      BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE form_probes ADD COLUMN IF NOT EXISTS profile_id INTEGER REFERENCES probe_profiles(id) ON DELETE SET NULL;

-- Profil lama (settings) → identitas pertama, sekaligus bawaan
INSERT INTO probe_profiles (label, username_prefix, password, fullname, email, phone, accounts, is_default)
SELECT 'Identitas 1',
       (SELECT TRIM(value) FROM settings WHERE key = 'probe_username_prefix'),
       (SELECT value FROM settings WHERE key = 'probe_password'),
       COALESCE((SELECT TRIM(value) FROM settings WHERE key = 'probe_fullname'), ''),
       COALESCE((SELECT TRIM(value) FROM settings WHERE key = 'probe_email'), ''),
       COALESCE((SELECT TRIM(value) FROM settings WHERE key = 'probe_phone'), ''),
       COALESCE((SELECT value::jsonb FROM settings WHERE key = 'probe_accounts' AND value LIKE '[%'), '[]'::jsonb),
       true
WHERE COALESCE((SELECT TRIM(value) FROM settings WHERE key = 'probe_username_prefix'), '') <> ''
  AND COALESCE((SELECT value FROM settings WHERE key = 'probe_password'), '') <> ''
  AND NOT EXISTS (SELECT 1 FROM probe_profiles);
