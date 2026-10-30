-- Hapus data image yang salah masuk ke tabel links
DELETE FROM site_links WHERE link_type = 'image';
DELETE FROM links
  WHERE id NOT IN (SELECT DISTINCT link_id FROM site_links)
  AND url ~ '\.(webp|png|jpg|jpeg|gif|svg|ico|bmp|avif)(\?.*)?$';

-- Tabel khusus untuk URL gambar dari domain eksternal
CREATE TABLE images (
    id         SERIAL PRIMARY KEY,
    url        TEXT UNIQUE NOT NULL,
    domain     TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE site_images (
    id         SERIAL PRIMARY KEY,
    site_id    INTEGER REFERENCES sites(id) ON DELETE CASCADE,
    image_id   INTEGER REFERENCES images(id) ON DELETE CASCADE,
    first_seen TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (site_id, image_id)
);

CREATE INDEX idx_images_domain         ON images (domain);
CREATE INDEX idx_site_images_site      ON site_images (site_id);
CREATE INDEX idx_site_images_image     ON site_images (image_id);
