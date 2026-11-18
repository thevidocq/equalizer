-- People join a tenant by email invitation and choose their own password.

CREATE TABLE invitations (
    id           bigserial PRIMARY KEY,
    tenant_id    text NOT NULL REFERENCES tenants (id),
    email        text NOT NULL,
    name         text NOT NULL,
    role         text NOT NULL CHECK (role IN ('admin', 'editor', 'approver', 'auditor')),
    token_sha256 text NOT NULL UNIQUE,
    invited_by   bigint REFERENCES users (id),
    sent         boolean NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    accepted_at  timestamptz,
    revoked      boolean NOT NULL DEFAULT false
);
CREATE INDEX invitations_tenant ON invitations (tenant_id) WHERE accepted_at IS NULL AND NOT revoked;
