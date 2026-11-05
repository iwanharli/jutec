-- Deep Sweep harian otomatis: keyword bawaan dirotasi tiap hari agar seluruh daftar tercakup bergiliran
INSERT INTO settings (key, value, description) VALUES
 ('sweep_daily_enabled', 'true', 'Deep Sweep otomatis setiap hari (true/false)'),
 ('sweep_daily_hour', '2', 'Jam mulai Deep Sweep harian (0–23, waktu server)'),
 ('sweep_daily_keywords', '20', 'Jumlah keyword pencarian per hari — dirotasi dari daftar keyword aktif. Tiap keyword ±5 kueri Serper'),
 ('sweep_daily_depth', '2', 'Kedalaman crawl Deep Sweep harian (1–3)'),
 ('sweep_daily_max_urls', '300', 'Batas URL yang dianalisis per Deep Sweep harian'),
 ('sweep_daily_last', '', 'Tanggal terakhir Deep Sweep harian berjalan (diisi sistem)'),
 ('sweep_daily_offset', '0', 'Posisi rotasi keyword harian (diisi sistem)')
ON CONFLICT (key) DO NOTHING;
