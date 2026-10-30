-- Satu situs = satu baris: tolak URL yang hanya beda "/" di akhir atau huruf besar-kecil.
-- SaveScanResult memakai indeks ini sebagai target ON CONFLICT, jadi varian penulisan memperbarui baris yang ada.
UPDATE sites SET url = rtrim(url, '/') WHERE url LIKE '%/' AND rtrim(url, '/') ~ '^https?://[^/]+';
CREATE UNIQUE INDEX IF NOT EXISTS sites_url_canonical_idx ON sites (lower(rtrim(url, '/')));
