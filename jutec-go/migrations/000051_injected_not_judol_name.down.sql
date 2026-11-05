CREATE OR REPLACE FUNCTION site_injected(url text, injection jsonb) RETURNS boolean
LANGUAGE sql IMMUTABLE AS $$
  SELECT COALESCE((injection->>'detected')::boolean, false)
      OR url ~* '\.(go|ac|sch|desa|mil|gov)\.id(/|$)|\.(gov|edu|int|mil)(\.[a-z]{2})?(/|$)|\.(edu|ac|gov)\.[a-z]{2}(/|$)'
$$;
