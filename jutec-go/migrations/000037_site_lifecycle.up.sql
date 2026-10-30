-- Siklus hidup situs: satu definisi "aktif / tidak aktif" yang dipakai dashboard, repository & halaman detail.
--   inactive      : domain hilang (NXDOMAIN), atau tidak bisa diakses ≥14 hari berturut-turut
--   blocked_alive : diblokir ISP tetapi server masih melayani (tetap dianggap aktif — justru prioritas)
--   uncertain     : baru gagal diakses (<14 hari) — bisa gangguan sementara
--   active        : online / belum pernah gagal
CREATE OR REPLACE FUNCTION site_lifecycle(reach TEXT, st TEXT, off TIMESTAMPTZ) RETURNS TEXT
LANGUAGE sql STABLE AS $$
  SELECT CASE
    WHEN reach = 'nxdomain' THEN 'inactive'
    WHEN off IS NOT NULL AND off < NOW() - INTERVAL '14 days' THEN 'inactive'
    WHEN reach = 'blocked_alive' THEN 'blocked_alive'
    WHEN reach IN ('offline', 'blocked') OR st = 'unreachable' THEN 'uncertain'
    ELSE 'active'
  END
$$;

-- Kapan situs yang sudah tidak aktif hidup lagi (sinyal operator memakai ulang domain lama)
ALTER TABLE sites ADD COLUMN IF NOT EXISTS revived_at TIMESTAMPTZ;

-- offline_since sebelumnya tidak pernah diisi — isi dari riwayat keterjangkauan untuk situs yang sedang mati
UPDATE sites s SET offline_since = COALESCE(
    (SELECT MAX(r.checked_at) FROM site_reachability r WHERE r.site_id = s.id AND r.status = s.reach_status),
    s.reach_checked_at, NOW())
WHERE s.reach_status IN ('offline', 'blocked', 'nxdomain') AND s.offline_since IS NULL;
