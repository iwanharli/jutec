CREATE TABLE nameservers (
    id         SERIAL PRIMARY KEY,
    host       TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE site_ns (
    id       SERIAL PRIMARY KEY,
    site_id  INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    ns_id    INTEGER REFERENCES nameservers(id) ON DELETE CASCADE,
    UNIQUE (site_id, ns_id)
);

CREATE TABLE ssl_certs (
    id          SERIAL PRIMARY KEY,
    thumbprint  TEXT UNIQUE NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE site_ssl (
    id       SERIAL PRIMARY KEY,
    site_id  INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    cert_id  INTEGER REFERENCES ssl_certs(id) ON DELETE CASCADE,
    UNIQUE (site_id, cert_id)
);

CREATE INDEX idx_nameservers_host      ON nameservers (host);
CREATE INDEX idx_site_ns_site          ON site_ns (site_id);
CREATE INDEX idx_site_ns_ns            ON site_ns (ns_id);
CREATE INDEX idx_ssl_certs_thumbprint  ON ssl_certs (thumbprint);
CREATE INDEX idx_site_ssl_site         ON site_ssl (site_id);
CREATE INDEX idx_site_ssl_cert         ON site_ssl (cert_id);
