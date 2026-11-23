-- Changes the assistant proposed on a site's map. The model never writes a
-- map: it stores a checked change here, and an administrator turns it into
-- a draft with a click (draft_version, once done). Kept 30 days.

CREATE TABLE assistant_proposals (
    id            text PRIMARY KEY,
    app_id        text NOT NULL REFERENCES apps (id) ON DELETE CASCADE,
    change        jsonb NOT NULL,
    summary       text NOT NULL,
    asked_by      bigint REFERENCES users (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    draft_version int
);

CREATE INDEX assistant_proposals_app ON assistant_proposals (app_id, created_at);
