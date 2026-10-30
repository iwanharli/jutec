-- Status situs di daftar blokir TrustPositif (Komdigi), dicocokkan harian dari mirror publik
ALTER TABLE sites ADD COLUMN IF NOT EXISTS tp_listed BOOLEAN;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS tp_listed_since TIMESTAMPTZ;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS tp_checked_at TIMESTAMPTZ;
