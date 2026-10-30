-- Manajemen kasus: satu perkara memuat situs, klaster operator, artefak, IP, dan bukti
CREATE TABLE IF NOT EXISTS cases (
    id          SERIAL PRIMARY KEY,
    number      TEXT UNIQUE,            -- nomor perkara/LP (opsional)
    title       TEXT NOT NULL,
    description TEXT,
    status      TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed')),
    lead        TEXT,                   -- penyidik penanggung jawab
    created_by  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- ref: id situs / kunci klaster stabil (cluster:<id situs terkecil>) / id artefak / alamat IP / id bukti
CREATE TABLE IF NOT EXISTS case_items (
    id        SERIAL PRIMARY KEY,
    case_id   INTEGER NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    kind      TEXT NOT NULL CHECK (kind IN ('site','operator','artifact','ip','evidence')),
    ref       TEXT NOT NULL,
    label     TEXT,
    note      TEXT,
    added_by  TEXT NOT NULL,
    added_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (case_id, kind, ref)
);
CREATE INDEX IF NOT EXISTS idx_case_items_ref ON case_items(kind, ref);
