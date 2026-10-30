-- Tabel untuk data leak KomInfo Indonesia
CREATE TABLE IF NOT EXISTS leak_kominfo (
    id          SERIAL PRIMARY KEY,
    phone       TEXT NOT NULL,
    nik         TEXT NOT NULL,
    provider    TEXT,
    reg_date    TEXT,
    artifact_id INTEGER REFERENCES artifacts(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(phone, nik, reg_date)
);

CREATE INDEX idx_kominfo_phone ON leak_kominfo(phone);
CREATE INDEX idx_kominfo_nik ON leak_kominfo(nik);

-- Tabel untuk data leak Dukcapil
CREATE TABLE IF NOT EXISTS leak_dukcapil (
    id          SERIAL PRIMARY KEY,
    phone       TEXT NOT NULL,
    nik         TEXT NOT NULL,
    full_name   TEXT,
    gender      TEXT,
    birth_date  TEXT,
    address     TEXT,
    district    TEXT,
    location    TEXT,
    region      TEXT,
    artifact_id INTEGER REFERENCES artifacts(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(phone, nik)
);

CREATE INDEX idx_dukcapil_phone ON leak_dukcapil(phone);
CREATE INDEX idx_dukcapil_nik ON leak_dukcapil(nik);
CREATE INDEX idx_dukcapil_name ON leak_dukcapil(full_name);
