-- Email alerts: settings per tenant, and the last state seen and notified
-- for each check of each site (so a restart never sends an alert twice).
CREATE TABLE tenant_alerts (
    tenant_id    text PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
    enabled      boolean NOT NULL DEFAULT true,
    extra_emails text[] NOT NULL DEFAULT '{}'
);

CREATE TABLE site_alerts (
    app_id     text NOT NULL REFERENCES apps (id) ON DELETE CASCADE,
    check_name text NOT NULL,
    state      text NOT NULL,
    since      timestamptz NOT NULL DEFAULT now(),
    notified   text NOT NULL DEFAULT '',
    PRIMARY KEY (app_id, check_name)
);
