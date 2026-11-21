-- Two roles in a tenant: administrators do everything, viewers read. A
-- version no longer needs a second person to be approved, so editors and
-- approvers become administrators and auditors viewers.
ALTER TABLE users DROP CONSTRAINT users_tenant_role_check;
UPDATE users SET tenant_role = 'admin' WHERE tenant_role IN ('editor', 'approver');
UPDATE users SET tenant_role = 'viewer' WHERE tenant_role = 'auditor';
ALTER TABLE users ADD CONSTRAINT users_tenant_role_check CHECK (tenant_role IN ('admin', 'viewer'));

ALTER TABLE invitations DROP CONSTRAINT invitations_role_check;
UPDATE invitations SET role = 'admin' WHERE role IN ('editor', 'approver');
UPDATE invitations SET role = 'viewer' WHERE role = 'auditor';
ALTER TABLE invitations ADD CONSTRAINT invitations_role_check CHECK (role IN ('admin', 'viewer'));

-- The author may approve their own version.
ALTER TABLE map_versions DROP CONSTRAINT map_versions_check;
