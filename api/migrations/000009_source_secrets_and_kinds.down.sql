ALTER TABLE sources DROP COLUMN IF EXISTS secret_enc;

-- NOT VALID keeps sources of the newer kinds instead of deleting them (and
-- their cited items); only new rows are checked against the old list.
ALTER TABLE sources DROP CONSTRAINT sources_kind_check;
ALTER TABLE sources ADD CONSTRAINT sources_kind_check
    CHECK (kind IN ('hackernews', 'tabnews', 'reddit', 'youtube', 'rss')) NOT VALID;
