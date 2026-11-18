-- The AI half of the approver's aid (the fixed rules are computed on read).
CREATE TABLE map_reviews (
    id         bigserial PRIMARY KEY,
    app_id     text NOT NULL REFERENCES apps (id),
    version    integer NOT NULL,
    status     text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'done', 'failed')),
    summary    text,
    questions  jsonb NOT NULL DEFAULT '[]',
    error      text,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz
);
CREATE INDEX map_reviews_version ON map_reviews (app_id, version, id DESC);
CREATE UNIQUE INDEX map_reviews_one_active ON map_reviews (app_id, version) WHERE status IN ('queued', 'running');
