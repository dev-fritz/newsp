CREATE TABLE ideas (
    id                UUID PRIMARY KEY,
    user_id           UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status            TEXT        NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'saved', 'discarded')),
    title             TEXT        NOT NULL,
    summary           TEXT        NOT NULL,
    current_version   INTEGER     NOT NULL DEFAULT 1 CHECK (current_version >= 1),
    embedding         vector(768),
    embedding_model   TEXT,
    merged_candidates INTEGER     NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ideas_user_updated_idx ON ideas (user_id, updated_at DESC);
CREATE INDEX ideas_user_status_idx ON ideas (user_id, status);
CREATE INDEX ideas_embedding_idx ON ideas USING hnsw (embedding vector_cosine_ops);

-- Versions are immutable; content holds the structured idea (why, for whom,
-- how, stack, plan, monetization, competitors) validated by the application.
CREATE TABLE idea_versions (
    id            UUID PRIMARY KEY,
    idea_id       UUID        NOT NULL REFERENCES ideas (id) ON DELETE CASCADE,
    version       INTEGER     NOT NULL CHECK (version >= 1),
    origin        TEXT        NOT NULL CHECK (origin IN ('generated', 'merged', 'deepened', 'manual')),
    title         TEXT        NOT NULL,
    summary       TEXT        NOT NULL,
    difficulty    TEXT        NOT NULL CHECK (difficulty IN ('beginner', 'intermediate', 'advanced')),
    effort_weeks  INTEGER     CHECK (effort_weeks > 0),
    content       JSONB       NOT NULL,
    change_note   TEXT,
    similarity    REAL,
    model_id      UUID        REFERENCES ai_models (id) ON DELETE SET NULL,
    model_name    TEXT,
    created_by    UUID        REFERENCES users (id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (idea_id, version)
);

CREATE TABLE idea_evidence (
    idea_version_id UUID NOT NULL REFERENCES idea_versions (id) ON DELETE CASCADE,
    item_id         UUID NOT NULL REFERENCES items (id) ON DELETE RESTRICT,
    ref             TEXT NOT NULL,
    PRIMARY KEY (idea_version_id, item_id),
    UNIQUE (idea_version_id, ref)
);

CREATE INDEX idea_evidence_item_idx ON idea_evidence (item_id);
