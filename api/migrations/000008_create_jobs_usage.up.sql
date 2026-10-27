CREATE TABLE jobs (
    id           UUID PRIMARY KEY,
    kind         TEXT        NOT NULL,
    payload      JSONB       NOT NULL DEFAULT '{}',
    user_id      UUID        REFERENCES users (id) ON DELETE CASCADE,
    status       TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'done', 'failed', 'canceled')),
    priority     SMALLINT    NOT NULL DEFAULT 0,
    run_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts     INTEGER     NOT NULL DEFAULT 0,
    max_attempts INTEGER     NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
    last_error   TEXT,
    result       JSONB,
    dedupe_key   TEXT,
    locked_by    TEXT,
    locked_at    TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at  TIMESTAMPTZ
);

-- At most one pending or running job per dedupe key.
CREATE UNIQUE INDEX jobs_dedupe_active_key ON jobs (dedupe_key)
    WHERE dedupe_key IS NOT NULL AND status IN ('pending', 'running');
CREATE INDEX jobs_ready_idx ON jobs (priority DESC, run_at) WHERE status = 'pending';
CREATE INDEX jobs_running_idx ON jobs (locked_at) WHERE status = 'running';
CREATE INDEX jobs_user_created_idx ON jobs (user_id, created_at DESC);
CREATE INDEX jobs_finished_idx ON jobs (finished_at) WHERE finished_at IS NOT NULL;

CREATE TABLE ai_usage (
    id            UUID PRIMARY KEY,
    user_id       UUID        REFERENCES users (id) ON DELETE SET NULL,
    model_id      UUID        REFERENCES ai_models (id) ON DELETE SET NULL,
    model_name    TEXT        NOT NULL,
    purpose       TEXT        NOT NULL CHECK (purpose IN ('report', 'idea', 'idea_judge', 'idea_merge', 'idea_deepen', 'embedding', 'connection_test')),
    input_tokens  INTEGER     NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens INTEGER     NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ai_usage_model_created_idx ON ai_usage (model_id, created_at DESC);
CREATE INDEX ai_usage_created_idx ON ai_usage (created_at DESC);
