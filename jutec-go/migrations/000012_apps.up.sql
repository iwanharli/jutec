CREATE TABLE IF NOT EXISTS apps (
    id           SERIAL PRIMARY KEY,
    url          TEXT NOT NULL UNIQUE,
    filename     TEXT,
    platform     TEXT NOT NULL DEFAULT 'unknown', -- android, ios, unknown
    discovered_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS site_apps (
    site_id INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    app_id  INTEGER REFERENCES apps(id)  ON DELETE CASCADE,
    PRIMARY KEY (site_id, app_id)
);

CREATE INDEX IF NOT EXISTS idx_apps_platform ON apps(platform);
CREATE INDEX IF NOT EXISTS idx_site_apps_site ON site_apps(site_id);
