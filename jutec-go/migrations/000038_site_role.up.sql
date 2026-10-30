-- Peran situs dalam jaringan judol: 'operator' (situs utama: form login/daftar sendiri, provider game,
-- minta rekening) atau 'landing' (pintu masuk: tombol daftar/login mengarah ke domain lain).
-- entry_targets = host tujuan tombol daftar/login di domain lain (dipakai menautkan landing → situs utama).
ALTER TABLE sites ADD COLUMN IF NOT EXISTS role TEXT;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS entry_targets TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX IF NOT EXISTS idx_sites_entry_targets ON sites USING GIN (entry_targets);
