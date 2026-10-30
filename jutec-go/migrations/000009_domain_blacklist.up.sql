CREATE TABLE IF NOT EXISTS domain_blacklist (
    id          SERIAL PRIMARY KEY,
    domain      TEXT NOT NULL UNIQUE,
    category    TEXT NOT NULL DEFAULT 'other',
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_blacklist_active ON domain_blacklist(is_active) WHERE is_active = true;

-- Seed: CDN & cloud storage
INSERT INTO domain_blacklist (domain, category) VALUES
    ('amazonaws.com', 'cdn'),
    ('cloudfront.net', 'cdn'),
    ('cloudflare.com', 'cdn'),
    ('cdn.jsdelivr.net', 'cdn'),
    ('unpkg.com', 'cdn'),
    ('cdnjs.com', 'cdn'),
    ('fastly.net', 'cdn'),
    ('akamai.com', 'cdn'),
    ('azureedge.net', 'cdn'),
    ('digitaloceanspaces.com', 'cdn'),
    ('googleapis.com', 'cdn'),
    ('gstatic.com', 'cdn'),
    ('bootstrapcdn.com', 'cdn'),
    ('fontawesome.com', 'cdn'),
    ('ampproject.org', 'cdn')
ON CONFLICT (domain) DO NOTHING;

-- Seed: Social media
INSERT INTO domain_blacklist (domain, category) VALUES
    ('facebook.com', 'social'),
    ('twitter.com', 'social'),
    ('instagram.com', 'social'),
    ('youtube.com', 'social'),
    ('tiktok.com', 'social'),
    ('linkedin.com', 'social'),
    ('pinterest.com', 'social'),
    ('reddit.com', 'social'),
    ('t.me', 'social'),
    ('wa.me', 'social')
ON CONFLICT (domain) DO NOTHING;

-- Seed: Chat & support services
INSERT INTO domain_blacklist (domain, category) VALUES
    ('livechat.com', 'chat'),
    ('livechatinc.com', 'chat'),
    ('tawk.to', 'chat'),
    ('crisp.chat', 'chat'),
    ('zendesk.com', 'chat'),
    ('intercom.io', 'chat'),
    ('freshdesk.com', 'chat')
ON CONFLICT (domain) DO NOTHING;

-- Seed: Analytics & tracking
INSERT INTO domain_blacklist (domain, category) VALUES
    ('google.com', 'analytics'),
    ('google-analytics.com', 'analytics'),
    ('googletagmanager.com', 'analytics'),
    ('googlesyndication.com', 'analytics'),
    ('doubleclick.net', 'analytics'),
    ('hotjar.com', 'analytics'),
    ('facebook.net', 'analytics'),
    ('fbcdn.net', 'analytics')
ON CONFLICT (domain) DO NOTHING;

-- Seed: Link shorteners
INSERT INTO domain_blacklist (domain, category) VALUES
    ('bit.ly', 'shortener'),
    ('t.co', 'shortener'),
    ('goo.gl', 'shortener'),
    ('ow.ly', 'shortener'),
    ('tinyurl.com', 'shortener')
ON CONFLICT (domain) DO NOTHING;

-- Seed: Major platforms
INSERT INTO domain_blacklist (domain, category) VALUES
    ('apple.com', 'platform'),
    ('microsoft.com', 'platform'),
    ('github.com', 'platform'),
    ('gitlab.com', 'platform'),
    ('wordpress.com', 'platform'),
    ('wordpress.org', 'platform'),
    ('wp.com', 'platform'),
    ('blogger.com', 'platform'),
    ('medium.com', 'platform'),
    ('notion.so', 'platform'),
    ('slack.com', 'platform')
ON CONFLICT (domain) DO NOTHING;

-- Seed: Schema & standards
INSERT INTO domain_blacklist (domain, category) VALUES
    ('schema.org', 'standard'),
    ('w3.org', 'standard'),
    ('ogp.me', 'standard')
ON CONFLICT (domain) DO NOTHING;

-- Seed: Email services
INSERT INTO domain_blacklist (domain, category) VALUES
    ('mailchimp.com', 'email'),
    ('sendgrid.net', 'email'),
    ('mailgun.com', 'email')
ON CONFLICT (domain) DO NOTHING;

-- Seed: Payment gateways (legitimate)
INSERT INTO domain_blacklist (domain, category) VALUES
    ('paypal.com', 'payment'),
    ('stripe.com', 'payment')
ON CONFLICT (domain) DO NOTHING;

-- Seed: Link aggregators
INSERT INTO domain_blacklist (domain, category) VALUES
    ('heylink.me', 'shortener'),
    ('linktr.ee', 'shortener'),
    ('linkin.bio', 'shortener'),
    ('shorte.st', 'shortener'),
    ('shoorthener.com', 'shortener'),
    ('shorturlonline.com', 'shortener'),
    ('s.id', 'shortener')
ON CONFLICT (domain) DO NOTHING;
