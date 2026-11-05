-- Pasar sasaran situs judi: 'id' (judol Indonesia) | 'intl' (judi internasional: ulasan kasino,
-- operator legal luar negeri, provider game). Diisi saat scan (crawler.Market); data lama diisi di sini.
ALTER TABLE sites ADD COLUMN IF NOT EXISTS market TEXT;

UPDATE sites s SET market = CASE
  WHEN s.url ~* '^https?://[^/]+\.id(/|$|:)' OR substring(s.url from '^https?://([^/:]+)') ~* 'togel|toto|gacor|situs|slot88|maxwin|judi|bandar|hoki|4d' THEN 'id'
  WHEN EXISTS (SELECT 1 FROM site_artifacts sa JOIN artifacts a ON a.id = sa.artifact_id WHERE sa.site_id = s.id
               AND (a.type IN ('Bank','E-Wallet','QRIS','VirtualAccount') OR (a.type IN ('Contact','Phone') AND a.value ~ '^\+?(628|08)'))) THEN 'id'
  WHEN EXISTS (SELECT 1 FROM (SELECT result FROM scan_logs l WHERE l.site_id = s.id ORDER BY created_at DESC LIMIT 1) l,
               jsonb_array_elements_text(CASE WHEN jsonb_typeof(l.result->'result'->'found_keywords') = 'array' THEN l.result->'result'->'found_keywords' ELSE '[]' END) k
               WHERE k ~* 'togel|gacor|judi|judol|bandar|pulsa|deposit dana|rungkad|maxwin|hoki|situs|agen|daftar|bocoran|paito|syair|prediksi|keluaran|alternatif|new member|taruhan|slot88|live draw|mix parlay') THEN 'id'
  ELSE 'intl' END
WHERE s.is_gambling;

-- Situs judol (target penindakan) = berisi judi, bukan korban penyusupan, bukan judi internasional
CREATE OR REPLACE FUNCTION site_is_judol(is_gambling boolean, url text, injection jsonb, market text) RETURNS boolean
LANGUAGE sql IMMUTABLE AS $$
  SELECT COALESCE(is_gambling, false) AND NOT site_injected(url, injection) AND COALESCE(market, 'id') <> 'intl'
$$;
DROP FUNCTION IF EXISTS site_is_judol(boolean, text, jsonb);
