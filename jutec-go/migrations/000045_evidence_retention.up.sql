INSERT INTO settings (key, value, description) VALUES ('evidence_retention_days', '0',
 'Hapus FILE bukti forensik (screenshot, HTML, jaringan) situs yang sudah tidak aktif lebih dari N hari. 0 = tidak pernah. Catatan bukti & hash SHA-256 tetap disimpan. Bukti yang masuk kasus atau milik situs terverifikasi judol tidak pernah dihapus.')
ON CONFLICT (key) DO NOTHING;
