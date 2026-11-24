-- The map draft built from a scan's findings, and which finding each of its
-- changes closes.

ALTER TABLE security_scans
    ADD COLUMN draft_version int,
    ADD COLUMN draft_note    text;

CREATE TABLE security_fixes (
    scan_id   bigint NOT NULL REFERENCES security_scans (id) ON DELETE CASCADE,
    remote_id text NOT NULL,
    version   int NOT NULL,
    change    text NOT NULL,
    PRIMARY KEY (scan_id, remote_id)
);

-- Quick scans (seconds: headers, HTTPS, cookies, exposed files).
ALTER TABLE security_scans DROP CONSTRAINT security_scans_scan_type_check;
ALTER TABLE security_scans ADD CONSTRAINT security_scans_scan_type_check CHECK (scan_type IN ('quick', 'standard', 'deep'));
