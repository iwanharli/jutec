DROP FUNCTION IF EXISTS site_is_judol(boolean, text, jsonb, text);
CREATE OR REPLACE FUNCTION site_is_judol(is_gambling boolean, url text, injection jsonb) RETURNS boolean
LANGUAGE sql IMMUTABLE AS $$ SELECT COALESCE(is_gambling, false) AND NOT site_injected(url, injection) $$;
ALTER TABLE sites DROP COLUMN IF EXISTS market;
