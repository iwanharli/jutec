CREATE TABLE detection_keywords (
    id         SERIAL PRIMARY KEY,
    keyword    TEXT UNIQUE NOT NULL,
    weight     FLOAT   DEFAULT 0.2,
    is_active  BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE search_keywords (
    id         SERIAL PRIMARY KEY,
    keyword    TEXT UNIQUE NOT NULL,
    is_active  BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Seed: detection keywords (dipakai untuk scoring konten HTML)
INSERT INTO detection_keywords (keyword, weight) VALUES
    ('slot',           0.2),
    ('gacor',          0.2),
    ('maxwin',         0.2),
    ('zeus',           0.2),
    ('olympus',        0.2),
    ('mahjong ways',   0.2),
    ('deposit pulsa',  0.2),
    ('depo 10k',       0.2),
    ('wd lancar',      0.2),
    ('bandar',         0.2),
    ('togel',          0.2),
    ('jackpot',        0.2),
    ('pragmatic',      0.2),
    ('hoki',           0.2),
    ('judol',          0.2),
    ('taruhan',        0.2);

-- Seed: search keywords (dipakai untuk Google Discovery)
INSERT INTO search_keywords (keyword) VALUES
    ('slot gacor hari ini'),
    ('slot gacor maxwin 2024'),
    ('slot gacor pragmatic play'),
    ('rtp slot live gacor hari ini'),
    ('pola slot mahjong ways gacor'),
    ('slot zeus olympus gacor maxwin'),
    ('depo 10k bonus new member slot'),
    ('deposit pulsa tanpa potongan slot'),
    ('agen slot deposit dana ovo gopay'),
    ('bonus new member 100 slot'),
    ('togel online terpercaya 2024'),
    ('bandar togel resmi hadiah terbesar'),
    ('prediksi togel jitu hari ini'),
    ('casino online indonesia terpercaya'),
    ('sbobet88 daftar resmi'),
    ('link alternatif sbobet terbaru'),
    ('bandar bola online terpercaya'),
    ('situs judi slot online terpercaya'),
    ('daftar situs judol gacor'),
    ('agen judi online resmi indonesia');
