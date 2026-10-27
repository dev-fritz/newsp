CREATE TABLE user_preferences (
    user_id                UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    topics                 TEXT[]      NOT NULL DEFAULT '{}',
    topics_embedding       vector(768),
    topics_embedding_model TEXT,
    output_lang            TEXT        NOT NULL DEFAULT 'pt-BR' CHECK (output_lang IN ('pt-BR', 'en')),
    timezone               TEXT        NOT NULL DEFAULT 'America/Sao_Paulo',
    report_enabled         BOOLEAN     NOT NULL DEFAULT true,
    report_time            TIME        NOT NULL DEFAULT '08:00',
    next_report_at         TIMESTAMPTZ,
    ideas_per_day          INTEGER     NOT NULL DEFAULT 3 CHECK (ideas_per_day BETWEEN 0 AND 10),
    report_model_id        UUID        REFERENCES ai_models (id) ON DELETE SET NULL,
    idea_model_id          UUID        REFERENCES ai_models (id) ON DELETE SET NULL,
    theme                  TEXT        NOT NULL DEFAULT 'system' CHECK (theme IN ('light', 'dark', 'system')),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX user_preferences_next_report_idx ON user_preferences (next_report_at) WHERE report_enabled;

-- Sources a user opted out of. New sources are included for everyone by default.
CREATE TABLE user_source_exclusions (
    user_id   UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    source_id UUID NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, source_id)
);
