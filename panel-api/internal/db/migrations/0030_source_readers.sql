-- The source reader of a site: how it reaches the site's files, its token
-- (a hash), its public key, and its reads with their summaries.

CREATE TABLE source_readers (
    app_id     text PRIMARY KEY REFERENCES apps (id) ON DELETE CASCADE,
    method     text CHECK (method IN ('sftp', 'smb', 'local')),
    host       text,
    port       int,
    username   text,
    path       text,
    share      text,
    password   bytea,        -- smb only, sealed with EQ_MFA_KEY, label source:<app>
    token_hash text UNIQUE,  -- sha256 of the reader's token, hex
    public_key text,
    version    text,
    last_seen  timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE source_reads (
    id           bigserial PRIMARY KEY,
    app_id       text NOT NULL REFERENCES apps (id) ON DELETE CASCADE,
    status       text NOT NULL CHECK (status IN ('queued', 'running', 'done', 'failed')),
    error        text,
    summary      jsonb,
    discovery_id bigint,
    requested_by bigint REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    started_at   timestamptz,
    finished_at  timestamptz
);

CREATE INDEX source_reads_app ON source_reads (app_id, id DESC);
-- One read going on per site.
CREATE UNIQUE INDEX source_reads_one_going ON source_reads (app_id) WHERE status IN ('queued', 'running');

ALTER TABLE discoveries ADD COLUMN source_read bigint REFERENCES source_reads (id) ON DELETE SET NULL;
