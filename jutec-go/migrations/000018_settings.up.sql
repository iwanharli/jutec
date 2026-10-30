CREATE TABLE IF NOT EXISTS settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO settings (key, value, description) VALUES
    ('serper_pages',    '1',  'Jumlah halaman hasil Serper per keyword (1 halaman = 10 URL, 1 credit)'),
    ('serper_enabled',  'true', 'Aktifkan pencarian via Serper API'),
    ('crtsh_enabled',   'true', 'Aktifkan pencarian via CT Logs (crt.sh)'),
    ('auto_analyze',    'false', 'Otomatis analisis domain setelah sweep'),
    ('sweep_delay_ms',  '500',  'Jeda antar request saat sweep (millisecond)')
ON CONFLICT (key) DO NOTHING;
