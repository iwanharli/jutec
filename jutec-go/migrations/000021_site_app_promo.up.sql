-- Situs yang mempromosikan aplikasi (tombol/banner APK) meski link unduhnya tidak tersedia
ALTER TABLE sites ADD COLUMN IF NOT EXISTS app_promo BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS app_promo_evidence TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE sites ADD COLUMN IF NOT EXISTS app_promo_platforms TEXT[] NOT NULL DEFAULT '{}';
