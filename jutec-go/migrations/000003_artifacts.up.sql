-- Artefak unik: rekening, e-wallet, crypto, QRIS, kontak CS
CREATE TABLE artifacts (
    id         SERIAL PRIMARY KEY,
    type       TEXT    NOT NULL, -- Bank | E-Wallet | Crypto | QRIS | Contact
    value      TEXT    NOT NULL, -- nomor rekening, nomor HP, wallet address, dll
    name       TEXT,             -- nama bank/wallet/platform (BCA, DANA, WhatsApp, dll)
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (type, value)
);

-- Relasi many-to-many antara situs dan artefak
CREATE TABLE site_artifacts (
    id          SERIAL PRIMARY KEY,
    site_id     INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    artifact_id INTEGER REFERENCES artifacts(id) ON DELETE CASCADE,
    first_seen  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (site_id, artifact_id)
);

CREATE INDEX idx_artifacts_type_value  ON artifacts (type, value);
CREATE INDEX idx_artifacts_value       ON artifacts (value);
CREATE INDEX idx_site_artifacts_site   ON site_artifacts (site_id);
CREATE INDEX idx_site_artifacts_artifact ON site_artifacts (artifact_id);
