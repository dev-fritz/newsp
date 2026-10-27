ALTER TABLE instance_settings
    DROP COLUMN inactivity_pause_days,
    DROP COLUMN signup_enabled;

DELETE FROM user_tokens WHERE kind = 'email_verification';

ALTER TABLE user_tokens DROP CONSTRAINT user_tokens_check;
ALTER TABLE user_tokens DROP CONSTRAINT user_tokens_kind_check;
ALTER TABLE user_tokens ADD CONSTRAINT user_tokens_kind_check
    CHECK (kind IN ('invite', 'password_reset'));
ALTER TABLE user_tokens ADD CONSTRAINT user_tokens_check CHECK (
    (kind = 'invite' AND role IS NOT NULL AND user_id IS NULL)
    OR (kind = 'password_reset' AND user_id IS NOT NULL AND role IS NULL)
);

ALTER TABLE user_tokens DROP COLUMN email;
ALTER TABLE users DROP COLUMN email_verified_at;
