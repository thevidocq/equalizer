-- Each kind of alert can be turned off on its own: the kinds listed here
-- are not emailed (connector, legacy_site, isolation, certificate,
-- certificate_expiry, recovered).
ALTER TABLE tenant_alerts ADD COLUMN muted text[] NOT NULL DEFAULT '{}';
