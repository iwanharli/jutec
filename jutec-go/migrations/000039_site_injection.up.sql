-- Tanda situs sah yang disusupi judol (cloaking, link judol tersembunyi) — lihat crawler.Injection
ALTER TABLE sites ADD COLUMN IF NOT EXISTS injection JSONB;
CREATE INDEX IF NOT EXISTS idx_sites_injected ON sites (((injection->>'detected')::boolean)) WHERE injection IS NOT NULL;

-- Dork: cari halaman judol yang disusupkan ke situs institusi & organisasi
INSERT INTO search_keywords (keyword) VALUES
  ('site:go.id slot gacor'), ('site:go.id situs toto'), ('site:go.id togel'),
  ('site:ac.id slot gacor'), ('site:ac.id situs toto'), ('site:ac.id slot maxwin'),
  ('site:sch.id slot gacor'), ('site:desa.id slot gacor'), ('site:desa.id togel'),
  ('site:or.id slot gacor'), ('site:co.id slot gacor maxwin')
ON CONFLICT (keyword) DO NOTHING;
