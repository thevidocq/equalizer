-- A site born behind Equalizer never had a public address: nothing to
-- probe, and the status page should not wait for an old IP.
ALTER TABLE apps DROP CONSTRAINT apps_exposure_status_check;
ALTER TABLE apps ADD CONSTRAINT apps_exposure_status_check
    CHECK (exposure_status IN ('not_configured', 'never_public', 'pending', 'closed', 'exposed', 'error'));
