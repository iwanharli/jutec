-- URL unik yang ditemukan di dalam halaman situs judol
CREATE TABLE links (
    id         SERIAL PRIMARY KEY,
    url        TEXT UNIQUE NOT NULL,
    domain     TEXT    NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Relasi many-to-many: situs → link yang ditemukan di dalamnya
CREATE TABLE site_links (
    id         SERIAL PRIMARY KEY,
    site_id    INTEGER REFERENCES sites(id)  ON DELETE CASCADE,
    link_id    INTEGER REFERENCES links(id)  ON DELETE CASCADE,
    link_type  TEXT    NOT NULL, -- internal | external | iframe
    first_seen TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (site_id, link_id)
);

CREATE INDEX idx_links_domain       ON links (domain);
CREATE INDEX idx_site_links_site    ON site_links (site_id);
CREATE INDEX idx_site_links_link    ON site_links (link_id);
CREATE INDEX idx_site_links_type    ON site_links (link_type);
