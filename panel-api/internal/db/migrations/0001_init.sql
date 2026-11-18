-- Initial control-plane schema.

CREATE TABLE tenants (
    id         text PRIMARY KEY,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id            bigserial PRIMARY KEY,
    email         text NOT NULL UNIQUE,
    name          text NOT NULL,
    password_hash text NOT NULL,
    platform_role text CHECK (platform_role IN ('admin', 'operator')),
    tenant_id     text REFERENCES tenants (id),
    tenant_role   text CHECK (tenant_role IN ('admin', 'editor', 'approver', 'auditor')),
    disabled      boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    -- A user is either a platform user or a tenant user, never both.
    CHECK ((platform_role IS NOT NULL AND tenant_id IS NULL AND tenant_role IS NULL)
        OR (platform_role IS NULL AND tenant_id IS NOT NULL AND tenant_role IS NOT NULL))
);

CREATE TABLE panel_sessions (
    id_hash    text PRIMARY KEY,           -- SHA-256 of the cookie value
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX panel_sessions_expires ON panel_sessions (expires_at);

CREATE TABLE apps (
    id            text PRIMARY KEY,
    tenant_id     text NOT NULL REFERENCES tenants (id),
    name          text NOT NULL,
    public_domain text NOT NULL UNIQUE,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE connectors (
    id           text PRIMARY KEY,
    app_id       text NOT NULL REFERENCES apps (id),
    token_sha256 text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    rotated_at   timestamptz
);

CREATE TABLE map_versions (
    app_id       text NOT NULL REFERENCES apps (id),
    version      integer NOT NULL,
    status       text NOT NULL CHECK (status IN ('draft', 'in_review', 'approved', 'rejected', 'published', 'retired')),
    document     jsonb NOT NULL,
    note         text NOT NULL DEFAULT '',
    author_id    bigint NOT NULL REFERENCES users (id),
    approver_id  bigint REFERENCES users (id),
    created_at   timestamptz NOT NULL DEFAULT now(),
    decided_at   timestamptz,
    published_at timestamptz,
    payload      text,                     -- base64 of the signed bytes
    signature    text,
    key_id       text,
    PRIMARY KEY (app_id, version),
    CHECK (approver_id IS NULL OR approver_id <> author_id)
);
-- At most one published version per app.
CREATE UNIQUE INDEX map_versions_one_published ON map_versions (app_id) WHERE status = 'published';

CREATE TABLE audit (
    id        bigserial PRIMARY KEY,
    ts        timestamptz NOT NULL DEFAULT now(),
    user_id   bigint REFERENCES users (id),
    tenant_id text,
    action    text NOT NULL,
    target    text NOT NULL DEFAULT '',
    detail    jsonb NOT NULL DEFAULT '{}'
);
CREATE INDEX audit_tenant_ts ON audit (tenant_id, ts DESC);
