-- Other domains of a site (e.g. the bare domain next to www), redirected to
-- its public domain by the gateway. A domain belongs to one site only.
CREATE TABLE app_aliases (
    domain text PRIMARY KEY,
    app_id text NOT NULL REFERENCES apps (id) ON DELETE CASCADE
);
CREATE INDEX app_aliases_app ON app_aliases (app_id);
