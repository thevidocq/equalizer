-- Connectors belong to the tenant: one per server, shared by its sites.
ALTER TABLE connectors ADD COLUMN tenant_id text REFERENCES tenants (id);
UPDATE connectors c SET tenant_id = a.tenant_id FROM apps a WHERE a.id = c.app_id;
ALTER TABLE connectors ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE connectors DROP COLUMN app_id;
CREATE INDEX connectors_tenant ON connectors (tenant_id);

-- Exposure check: does the site still answer directly on its old public IP?
ALTER TABLE apps
    ADD COLUMN legacy_ip text,
    ADD COLUMN exposure_status text NOT NULL DEFAULT 'not_configured'
        CHECK (exposure_status IN ('not_configured', 'pending', 'closed', 'exposed', 'error')),
    ADD COLUMN exposure_reason text,
    ADD COLUMN exposure_checked_at timestamptz;
