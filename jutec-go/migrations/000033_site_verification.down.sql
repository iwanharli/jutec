DROP TABLE IF EXISTS site_verifications;
DROP INDEX IF EXISTS idx_sites_verify_queue;
ALTER TABLE sites DROP COLUMN IF EXISTS verified_note, DROP COLUMN IF EXISTS verified_at, DROP COLUMN IF EXISTS verified_by, DROP COLUMN IF EXISTS verified_status;
