-- Client secret of the identity provider a site logs in with (OIDC),
-- sealed with EQ_MFA_KEY. The rest of the OIDC settings is in the map.
CREATE TABLE app_oidc (
    app_id        text PRIMARY KEY REFERENCES apps (id) ON DELETE CASCADE,
    client_secret bytea NOT NULL,
    updated_at    timestamptz NOT NULL DEFAULT now()
);
