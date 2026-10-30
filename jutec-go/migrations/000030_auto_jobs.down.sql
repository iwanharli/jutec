DROP INDEX IF EXISTS idx_site_artifacts_first_seen;
ALTER TABLE sites DROP COLUMN IF EXISTS wayback_checked_at;
