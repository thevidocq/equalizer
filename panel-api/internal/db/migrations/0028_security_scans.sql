-- Security scans of a site by the tenant's vulnerability scanner (CVE Radar),
-- through Equalizer (the public domain) or of the original site. The key is
-- the tenant's, sealed like the AI key; findings are a copy of the scanner's,
-- read against the map when shown.

CREATE TABLE tenant_scanner (
    tenant_id  text PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
    endpoint   text NOT NULL,
    api_key    bytea NOT NULL,        -- sealed with EQ_MFA_KEY, label scanner:<tenant>
    key_hint   text NOT NULL,         -- last 4 characters, to recognise it
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE security_scans (
    id             bigserial PRIMARY KEY,
    app_id         text NOT NULL REFERENCES apps (id) ON DELETE CASCADE,
    target         text NOT NULL CHECK (target IN ('protected', 'original')),
    domain         text NOT NULL,
    scan_type      text NOT NULL CHECK (scan_type IN ('standard', 'deep')),
    status         text NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    remote_target  text NOT NULL,     -- the scanner's target id
    remote_scan    text NOT NULL,     -- the scanner's scan id
    findings_count int,
    risk_score     int,
    error          text,
    requested_by   bigint REFERENCES users (id) ON DELETE SET NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    polled_at      timestamptz,
    completed_at   timestamptz
);

CREATE INDEX security_scans_app ON security_scans (app_id, target, id DESC);
CREATE INDEX security_scans_open ON security_scans (status) WHERE status IN ('pending', 'running');
-- One scan going on per target of a site: two clicks cannot start two.
CREATE UNIQUE INDEX security_scans_one_going ON security_scans (app_id, target) WHERE status IN ('pending', 'running');

CREATE TABLE security_findings (
    scan_id       bigint NOT NULL REFERENCES security_scans (id) ON DELETE CASCADE,
    remote_id     text NOT NULL,
    source        text NOT NULL,
    title         text NOT NULL,
    severity      text NOT NULL CHECK (severity IN ('critical', 'high', 'medium', 'low', 'info')),
    cvss          double precision,
    cve           text[] NOT NULL DEFAULT '{}',
    cwe           text[] NOT NULL DEFAULT '{}',
    url           text,
    http_method   text,
    parameter     text,
    evidence      text,
    first_seen_at timestamptz,
    last_seen_at  timestamptz,
    PRIMARY KEY (scan_id, remote_id)
);
