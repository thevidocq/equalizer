-- Verifications of a map version by the agent (docs/agent.md, steps 4
-- and 5): every studied page compared with and without Equalizer, every
-- input tried with the attack set on the gateway only.
CREATE TABLE map_verifications (
    id            bigserial PRIMARY KEY,
    app_id        text NOT NULL REFERENCES apps (id),
    version       integer NOT NULL,
    status        text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'done', 'failed')),
    requested_by  bigint REFERENCES users (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    started_at    timestamptz,
    finished_at   timestamptz,
    score         jsonb,
    broken        jsonb NOT NULL DEFAULT '[]',
    open          jsonb NOT NULL DEFAULT '[]',
    error         text,
    FOREIGN KEY (app_id, version) REFERENCES map_versions (app_id, version)
);
CREATE INDEX map_verifications_version ON map_verifications (app_id, version, id DESC);
CREATE INDEX map_verifications_queued ON map_verifications (id) WHERE status = 'queued';
