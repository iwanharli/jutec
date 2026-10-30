DROP INDEX IF EXISTS idx_sites_entry_targets;
ALTER TABLE sites DROP COLUMN IF EXISTS entry_targets;
ALTER TABLE sites DROP COLUMN IF EXISTS role;
