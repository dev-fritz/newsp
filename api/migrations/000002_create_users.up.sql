CREATE TABLE users (
    id            UUID PRIMARY KEY,
    email         TEXT        NOT NULL CHECK (email <> ''),
    name          TEXT        NOT NULL CHECK (name <> ''),
    password_hash TEXT        NOT NULL,
    role          TEXT        NOT NULL CHECK (role IN ('admin', 'member')),
    status        TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    locale        TEXT        NOT NULL DEFAULT 'pt-BR' CHECK (locale IN ('pt-BR', 'en')),
    last_seen_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_key ON users (lower(email));

-- One-time links created by admins: invites (no user yet) and password resets.
CREATE TABLE user_tokens (
    id          UUID PRIMARY KEY,
    kind        TEXT        NOT NULL CHECK (kind IN ('invite', 'password_reset')),
    token_hash  BYTEA       NOT NULL UNIQUE,
    user_id     UUID        REFERENCES users (id) ON DELETE CASCADE,
    role        TEXT        CHECK (role IN ('admin', 'member')),
    created_by  UUID        REFERENCES users (id) ON DELETE SET NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (kind = 'invite' AND role IS NOT NULL AND user_id IS NULL)
        OR (kind = 'password_reset' AND user_id IS NOT NULL AND role IS NULL)
    )
);

CREATE INDEX user_tokens_pending_idx ON user_tokens (kind, created_at DESC)
    WHERE used_at IS NULL AND revoked_at IS NULL;

-- Opaque refresh tokens, stored hashed and rotated on every use. A token
-- reused after rotation revokes its whole family.
CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    family_id   UUID        NOT NULL,
    token_hash  BYTEA       NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    rotated_at  TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);
