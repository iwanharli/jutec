-- Domain induk situs untuk pengelompokan tabel: sub.toko.co.id → toko.co.id, a.b.com → b.com.
-- Subdomain platform hosting gratis (web.app, pages.dev, github.io…) adalah situs berbeda → host utuh.
CREATE OR REPLACE FUNCTION site_root(url text) RETURNS text
LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE
    WHEN h ~ '^[0-9.]+$' THEN h
    WHEN h ~ '\.(web\.app|firebaseapp\.com|pages\.dev|workers\.dev|vercel\.app|netlify\.app|github\.io|gitlab\.io|blogspot\.com|herokuapp\.com|glitch\.me|wixsite\.com|weebly\.com|wordpress\.com|systeme\.io|carrd\.co|s3\.amazonaws\.com)$' THEN h
    WHEN h ~ '\.(co|or|ac|go|sch|web|my|net|com|org|gov|edu|biz|desa|ponpes|mil|id)\.[a-z]{2}$' THEN substring(h from '([^.]+\.[^.]+\.[^.]+)$')
    ELSE COALESCE(substring(h from '([^.]+\.[^.]+)$'), h) END
  FROM (SELECT regexp_replace(lower(split_part(split_part(url, '://', 2), '/', 1)), '^www\.|:[0-9]+$', '', 'g') AS h) x
$$;
