-- Tracking IDs unik (GA, FB Pixel, LiveChat, dll)
CREATE TABLE tracking_ids (
    id         SERIAL PRIMARY KEY,
    platform   TEXT NOT NULL, -- GoogleAnalytics | GoogleTag | GoogleTagManager | FacebookPixel | LiveChat | TawkTo | Crisp | Hotjar
    value      TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (platform, value)
);

-- Relasi situs ↔ tracking ID
CREATE TABLE site_tracking (
    id          SERIAL PRIMARY KEY,
    site_id     INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    tracking_id INTEGER REFERENCES tracking_ids(id) ON DELETE CASCADE,
    first_seen  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (site_id, tracking_id)
);

-- IP addresses (mirror Neo4j, untuk SQL clustering)
CREATE TABLE ips (
    id         SERIAL PRIMARY KEY,
    address    TEXT UNIQUE NOT NULL,
    subnet_24  TEXT, -- "103.21.186" untuk IPv4 /24
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Relasi situs ↔ IP
CREATE TABLE site_ips (
    id       SERIAL PRIMARY KEY,
    site_id  INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    ip_id    INTEGER REFERENCES ips(id)   ON DELETE CASCADE,
    UNIQUE (site_id, ip_id)
);

-- SimHash DOM structure untuk template clustering
ALTER TABLE sites ADD COLUMN IF NOT EXISTS simhash TEXT;

CREATE INDEX idx_tracking_platform_value ON tracking_ids (platform, value);
CREATE INDEX idx_site_tracking_site      ON site_tracking (site_id);
CREATE INDEX idx_site_tracking_tid       ON site_tracking (tracking_id);
CREATE INDEX idx_ips_subnet              ON ips (subnet_24);
CREATE INDEX idx_site_ips_site           ON site_ips (site_id);
CREATE INDEX idx_site_ips_ip             ON site_ips (ip_id);
CREATE INDEX idx_sites_simhash           ON sites (simhash);
