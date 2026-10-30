-- Sumber data bocoran (LeakOSINT API / VflOSINT bot) + log audit setiap kueri ke bot
ALTER TABLE leak_kominfo ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'LeakOSINT';

CREATE TABLE IF NOT EXISTS vfl_queries (
    id               SERIAL PRIMARY KEY,
    query            TEXT NOT NULL,
    kind             TEXT NOT NULL,          -- phone | nik
    depth            INT  NOT NULL DEFAULT 0, -- 0 = nomor dari situs judol, 1 = NIK pemiliknya, 2 = nomor lain milik NIK itu
    root_artifact_id INTEGER REFERENCES artifacts(id) ON DELETE SET NULL,
    parent           TEXT,
    status           TEXT NOT NULL,          -- found | not_found | error
    result_count     INT  NOT NULL DEFAULT 0,
    raw              TEXT,
    error            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_vfl_queries_query ON vfl_queries(query, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_vfl_queries_root ON vfl_queries(root_artifact_id);
