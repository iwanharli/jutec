-- Domain bekas situs sah (bisnis/instansi) yang kini dipakai judol — lihat wayback.Takeover
ALTER TABLE sites ADD COLUMN IF NOT EXISTS takeover JSONB;
