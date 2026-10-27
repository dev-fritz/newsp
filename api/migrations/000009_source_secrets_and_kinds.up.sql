ALTER TABLE sources DROP CONSTRAINT sources_kind_check;
ALTER TABLE sources ADD CONSTRAINT sources_kind_check CHECK (kind IN (
    'hackernews', 'tabnews', 'reddit', 'youtube', 'rss',
    'lobsters', 'devto', 'mastodon', 'bluesky', 'github', 'producthunt'
));

-- Credential of the source (API token, app password), AES-256-GCM sealed with
-- ENCRYPTION_KEY like provider keys. NULL when the source has none.
ALTER TABLE sources ADD COLUMN secret_enc BYTEA;
