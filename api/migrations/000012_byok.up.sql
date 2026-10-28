-- BYOK (bring your own key): AI providers can belong to a user (user_id set) or
-- to the instance (user_id NULL, admin-managed). User providers are limited to
-- the hosted kinds, one per kind per user. Admins can switch BYOK off.
ALTER TABLE ai_providers ADD COLUMN user_id UUID REFERENCES users (id) ON DELETE CASCADE;
ALTER TABLE ai_providers ADD CONSTRAINT ai_providers_user_kind_check
    CHECK (user_id IS NULL OR kind IN ('openai', 'anthropic', 'gemini'));

DROP INDEX ai_providers_name_key;
CREATE UNIQUE INDEX ai_providers_instance_name_key ON ai_providers (lower(name)) WHERE user_id IS NULL;
CREATE UNIQUE INDEX ai_providers_user_kind_key ON ai_providers (user_id, kind) WHERE user_id IS NOT NULL;

ALTER TABLE instance_settings ADD COLUMN byok_enabled BOOLEAN NOT NULL DEFAULT true;
