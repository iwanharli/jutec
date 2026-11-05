-- Situs sah yang disusupi (cloaking/link tersembunyi, atau halaman judol di domain institusi) adalah KORBAN,
-- bukan situs operator: tetap is_gambling (berisi konten judol) tapi tidak dihitung sebagai situs judol.
CREATE OR REPLACE FUNCTION site_injected(url text, injection jsonb) RETURNS boolean
LANGUAGE sql IMMUTABLE AS $$
  SELECT COALESCE((injection->>'detected')::boolean, false)
      OR url ~* '\.(go|ac|sch|desa|mil|gov)\.id(/|$)|\.(gov|edu|int|mil)(\.[a-z]{2})?(/|$)|\.(edu|ac|gov)\.[a-z]{2}(/|$)'
$$;

CREATE OR REPLACE FUNCTION site_is_judol(is_gambling boolean, url text, injection jsonb) RETURNS boolean
LANGUAGE sql IMMUTABLE AS $$
  SELECT COALESCE(is_gambling, false) AND NOT site_injected(url, injection)
$$;
