DROP INDEX IF EXISTS idx_sites_injected;
ALTER TABLE sites DROP COLUMN IF EXISTS injection;
DELETE FROM search_keywords WHERE keyword LIKE 'site:%';
