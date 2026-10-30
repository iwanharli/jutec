DROP TABLE IF EXISTS site_reachability;
DROP INDEX IF EXISTS idx_sites_reach_due;
ALTER TABLE sites DROP COLUMN IF EXISTS blocked_since, DROP COLUMN IF EXISTS reach_checked_at, DROP COLUMN IF EXISTS reach_status;
