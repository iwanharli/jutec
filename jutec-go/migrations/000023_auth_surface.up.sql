-- Peta login & pendaftaran (URL, form, field, pilihan bank, captcha/OTP, kode referral)
ALTER TABLE sites ADD COLUMN IF NOT EXISTS auth_surface JSONB;
