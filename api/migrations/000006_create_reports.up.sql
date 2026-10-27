CREATE TABLE reports (
    id               UUID PRIMARY KEY,
    user_id          UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind             TEXT        NOT NULL CHECK (kind IN ('scheduled', 'manual')),
    status           TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'done', 'failed')),
    window_start     TIMESTAMPTZ NOT NULL,
    window_end       TIMESTAMPTZ NOT NULL,
    focus            TEXT,
    title            TEXT,
    summary          TEXT,
    content_md       TEXT,
    output_lang      TEXT        NOT NULL CHECK (output_lang IN ('pt-BR', 'en')),
    model_id         UUID        REFERENCES ai_models (id) ON DELETE SET NULL,
    model_name       TEXT,
    items_considered INTEGER     NOT NULL DEFAULT 0,
    error            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at     TIMESTAMPTZ,
    CHECK (window_end > window_start)
);

CREATE INDEX reports_user_created_idx ON reports (user_id, created_at DESC);

-- Citations: [ref] in content_md points at item_id. Cited items survive retention.
CREATE TABLE report_items (
    report_id UUID    NOT NULL REFERENCES reports (id) ON DELETE CASCADE,
    item_id   UUID    NOT NULL REFERENCES items (id) ON DELETE RESTRICT,
    ref       INTEGER NOT NULL CHECK (ref > 0),
    PRIMARY KEY (report_id, item_id),
    UNIQUE (report_id, ref)
);

CREATE INDEX report_items_item_idx ON report_items (item_id);
