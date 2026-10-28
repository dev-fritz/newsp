ALTER TABLE instance_settings DROP COLUMN byok_enabled;

DELETE FROM ai_providers WHERE user_id IS NOT NULL;

DROP INDEX ai_providers_user_kind_key;
DROP INDEX ai_providers_instance_name_key;
CREATE UNIQUE INDEX ai_providers_name_key ON ai_providers (lower(name));

ALTER TABLE ai_providers DROP CONSTRAINT ai_providers_user_kind_check;
ALTER TABLE ai_providers DROP COLUMN user_id;
