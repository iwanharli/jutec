-- Domain bernama judol (keraton4d, opesiatoto, ...) milik operator, bukan korban: sinyal cloaking /
-- link tersembunyi di domain seperti ini biasanya salah baca (versi pengunjung diblokir ISP/bot).
-- Halaman sisipan di domain institusi tetap dihitung korban (host-nya tidak bernama judol).
CREATE OR REPLACE FUNCTION site_injected(url text, injection jsonb) RETURNS boolean
LANGUAGE sql IMMUTABLE AS $$
  SELECT (COALESCE((injection->>'detected')::boolean, false)
          AND COALESCE(substring(url from '^https?://([^/:]+)'), '') !~* '(toto|togel|slot|gacor|4d|judi|bandar|maxwin|casino|kasino|poker|sbobet|qq|jackpot|scatter|hoki|303|777)')
      OR url ~* '\.(go|ac|sch|desa|mil|gov)\.id(/|$)|\.(gov|edu|int|mil)(\.[a-z]{2})?(/|$)|\.(edu|ac|gov)\.[a-z]{2}(/|$)'
$$;
