CREATE TABLE sources (
    id                   UUID PRIMARY KEY,
    kind                 TEXT        NOT NULL CHECK (kind IN ('hackernews', 'tabnews', 'reddit', 'youtube', 'rss')),
    name                 TEXT        NOT NULL CHECK (name <> ''),
    config               JSONB       NOT NULL DEFAULT '{}',
    interval_minutes     INTEGER     NOT NULL DEFAULT 30 CHECK (interval_minutes BETWEEN 5 AND 1440),
    enabled              BOOLEAN     NOT NULL DEFAULT true,
    next_run_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_run_at          TIMESTAMPTZ,
    last_success_at      TIMESTAMPTZ,
    last_error           TEXT,
    consecutive_failures INTEGER     NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sources_due_idx ON sources (next_run_at) WHERE enabled;

CREATE TABLE items (
    id                 UUID PRIMARY KEY,
    source_id          UUID        NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    external_id        TEXT        NOT NULL,
    url                TEXT        NOT NULL,
    title              TEXT        NOT NULL,
    summary            TEXT        NOT NULL DEFAULT '',
    author             TEXT        NOT NULL DEFAULT '',
    published_at       TIMESTAMPTZ NOT NULL,
    collected_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    score              INTEGER,
    comments           INTEGER,
    metrics            JSONB       NOT NULL DEFAULT '{}',
    content_text       TEXT,
    content_fetched_at TIMESTAMPTZ,
    content_error      TEXT,
    embedding          vector(768),
    embedding_model    TEXT,
    embedded_at        TIMESTAMPTZ,
    search             tsvector GENERATED ALWAYS AS (to_tsvector('simple', title || ' ' || summary || ' ' || author)) STORED,
    UNIQUE (source_id, external_id)
);

CREATE INDEX items_published_idx ON items (published_at DESC);
CREATE INDEX items_source_published_idx ON items (source_id, published_at DESC);
CREATE INDEX items_search_idx ON items USING gin (search);
CREATE INDEX items_embedding_idx ON items USING hnsw (embedding vector_cosine_ops);
CREATE INDEX items_pending_embedding_idx ON items (collected_at) WHERE embedding IS NULL;
