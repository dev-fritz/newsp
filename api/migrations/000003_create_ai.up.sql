CREATE TABLE ai_providers (
    id            UUID PRIMARY KEY,
    name          TEXT        NOT NULL CHECK (name <> ''),
    kind          TEXT        NOT NULL CHECK (kind IN ('openai_compatible', 'openai', 'anthropic', 'gemini')),
    base_url      TEXT        NOT NULL,
    api_key_enc   BYTEA,
    enabled       BOOLEAN     NOT NULL DEFAULT true,
    last_check_at TIMESTAMPTZ,
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ai_providers_name_key ON ai_providers (lower(name));

CREATE TABLE ai_models (
    id           UUID PRIMARY KEY,
    provider_id  UUID        NOT NULL REFERENCES ai_providers (id) ON DELETE CASCADE,
    name         TEXT        NOT NULL CHECK (name <> ''),
    display_name TEXT        NOT NULL CHECK (display_name <> ''),
    capability   TEXT        NOT NULL CHECK (capability IN ('chat', 'embedding')),
    enabled      BOOLEAN     NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider_id, name, capability)
);

-- Exactly one row: the singleton column is unique and can only be true.
CREATE TABLE instance_settings (
    id                        UUID PRIMARY KEY,
    singleton                 BOOLEAN     NOT NULL DEFAULT true UNIQUE CHECK (singleton),
    embedding_model_id        UUID        REFERENCES ai_models (id) ON DELETE SET NULL,
    default_report_model_id   UUID        REFERENCES ai_models (id) ON DELETE SET NULL,
    default_idea_model_id     UUID        REFERENCES ai_models (id) ON DELETE SET NULL,
    retention_days            INTEGER     NOT NULL DEFAULT 90 CHECK (retention_days BETWEEN 7 AND 3650),
    idea_similarity_threshold REAL        NOT NULL DEFAULT 0.80 CHECK (idea_similarity_threshold BETWEEN 0.50 AND 0.99),
    setup_completed_at        TIMESTAMPTZ,
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO instance_settings (id) VALUES (gen_random_uuid());
