CREATE TABLE IF NOT EXISTS telegram_channels (
    id              SERIAL PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,          -- tanpa @
    telegram_id     BIGINT,                        -- numeric ID dari Telegram
    display_name    TEXT,
    bio             TEXT,
    member_count    INTEGER,
    channel_type    TEXT,                          -- 'user', 'bot', 'channel', 'group'
    is_active       BOOLEAN DEFAULT true,
    linked_url      TEXT,                          -- URL situs dari bio/deskripsi
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS telegram_messages (
    id              SERIAL PRIMARY KEY,
    channel_id      INTEGER REFERENCES telegram_channels(id) ON DELETE CASCADE,
    message_id      BIGINT NOT NULL,
    text            TEXT,
    date            TIMESTAMPTZ,
    has_artifacts   BOOLEAN DEFAULT false,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(channel_id, message_id)
);

-- Link channel ke artifacts yang ditemukan dari pesannya
CREATE TABLE IF NOT EXISTS telegram_artifacts (
    channel_id      INTEGER REFERENCES telegram_channels(id) ON DELETE CASCADE,
    artifact_id     INTEGER REFERENCES artifacts(id) ON DELETE CASCADE,
    PRIMARY KEY (channel_id, artifact_id)
);

CREATE INDEX IF NOT EXISTS idx_tg_channels_username ON telegram_channels(username);
CREATE INDEX IF NOT EXISTS idx_tg_channels_telegram_id ON telegram_channels(telegram_id);
CREATE INDEX IF NOT EXISTS idx_tg_messages_channel ON telegram_messages(channel_id, date DESC);
