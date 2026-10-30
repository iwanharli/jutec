-- Token verifikasi domain yang diekstrak dari TXT records DNS
-- Satu token yang sama di banyak domain = satu operator
CREATE TABLE txt_tokens (
    id         SERIAL PRIMARY KEY,
    service    TEXT NOT NULL, -- GoogleVerification | FacebookVerification | MicrosoftVerification | SPFInclude | YandexVerification | AppleVerification
    token      TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (service, token)
);

CREATE TABLE site_txt (
    id       SERIAL PRIMARY KEY,
    site_id  INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    token_id INTEGER REFERENCES txt_tokens(id) ON DELETE CASCADE,
    UNIQUE (site_id, token_id)
);

CREATE INDEX idx_txt_tokens_service_token ON txt_tokens (service, token);
CREATE INDEX idx_site_txt_site            ON site_txt (site_id);
CREATE INDEX idx_site_txt_token           ON site_txt (token_id);
