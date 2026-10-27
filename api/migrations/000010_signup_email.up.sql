-- Public signup with e-mail verification. Accounts created before this
-- migration (setup and invites) count as verified.
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;
UPDATE users SET email_verified_at = created_at;

-- Invites now remember the e-mail they were sent to.
ALTER TABLE user_tokens ADD COLUMN email TEXT CHECK (email IS NULL OR email <> '');

ALTER TABLE user_tokens DROP CONSTRAINT user_tokens_kind_check;
ALTER TABLE user_tokens DROP CONSTRAINT user_tokens_check;
ALTER TABLE user_tokens ADD CONSTRAINT user_tokens_kind_check
    CHECK (kind IN ('invite', 'password_reset', 'email_verification'));
ALTER TABLE user_tokens ADD CONSTRAINT user_tokens_check CHECK (
    (kind = 'invite' AND role IS NOT NULL AND user_id IS NULL)
    OR (kind = 'password_reset' AND user_id IS NOT NULL AND role IS NULL)
    OR (kind = 'email_verification' AND user_id IS NOT NULL AND role IS NULL)
);

ALTER TABLE instance_settings
    ADD COLUMN signup_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN inactivity_pause_days INTEGER NOT NULL DEFAULT 30
        CHECK (inactivity_pause_days BETWEEN 0 AND 3650);
