-- Status pembacaan channel & kesalahan terakhir per username
ALTER TABLE telegram_channels ADD COLUMN IF NOT EXISTS last_scraped_at TIMESTAMPTZ;
ALTER TABLE telegram_channels ADD COLUMN IF NOT EXISTS last_error TEXT;

-- Penanda "unresolved" lama dibuat saat sesi Telegram mati (bukan karena username tidak ada) →
-- hapus agar diperiksa ulang. Status baru: 'not_found' hanya bila Telegram menyatakan username tak ada.
DELETE FROM telegram_channels WHERE channel_type = 'unresolved';
