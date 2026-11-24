-- What the scanner said of the key at the last real call: a refused key is
-- shown in Settings instead of being found when a scan fails.
ALTER TABLE tenant_scanner ADD COLUMN key_refused boolean NOT NULL DEFAULT false;
ALTER TABLE tenant_scanner ADD COLUMN checked_at timestamptz;

-- A site in Docker: the reader takes the volumes of its container.
ALTER TABLE source_readers ADD COLUMN container text;

-- Where a running verification is: shown in the panel, erased at the end.
ALTER TABLE map_verifications ADD COLUMN progress jsonb;
