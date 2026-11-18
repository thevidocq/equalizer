-- The tenant's own AI model. The platform has none.
CREATE TABLE tenant_llm (
    tenant_id  text PRIMARY KEY REFERENCES tenants (id),
    provider   text NOT NULL CHECK (provider IN ('anthropic', 'openai', 'local')),
    model      text NOT NULL,
    endpoint   text,
    api_key    bytea,                 -- sealed with EQ_MFA_KEY, label llm:<tenant>
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Discoveries: the agent explores a site and proposes a draft map.
CREATE TABLE discoveries (
    id             bigserial PRIMARY KEY,
    app_id         text NOT NULL REFERENCES apps (id),
    status         text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'done', 'failed')),
    trigger        text NOT NULL DEFAULT 'manual' CHECK (trigger IN ('manual', 'schedule')),
    use_ai         boolean NOT NULL,
    max_pages      integer NOT NULL,
    reach          jsonb NOT NULL,     -- base_url and legacy.reach for the run
    account_user   text,
    account_secret bytea,              -- sealed, erased when the discovery ends
    requested_by   bigint REFERENCES users (id),
    created_at     timestamptz NOT NULL DEFAULT now(),
    started_at     timestamptz,
    finished_at    timestamptz,
    progress       jsonb NOT NULL DEFAULT '{}',
    report         jsonb,
    draft_version  integer,
    error          text
);
CREATE UNIQUE INDEX discoveries_one_active ON discoveries (app_id) WHERE status IN ('queued', 'running');
CREATE INDEX discoveries_app ON discoveries (app_id, id DESC);

-- Continuous discovery.
ALTER TABLE apps
    ADD COLUMN discovery_schedule text NOT NULL DEFAULT 'off' CHECK (discovery_schedule IN ('off', 'daily', 'weekly')),
    ADD COLUMN discovery_owner bigint REFERENCES users (id);

-- Paths the gateway refused as not mapped, per site (never the query).
CREATE TABLE unmapped_hits (
    app_id     text NOT NULL REFERENCES apps (id),
    method     text NOT NULL,
    path       text NOT NULL,
    hits       bigint NOT NULL,
    clients    bigint NOT NULL,        -- approximate: summed per gateway batch
    first_seen timestamptz NOT NULL DEFAULT now(),
    last_seen  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (app_id, method, path)
);
