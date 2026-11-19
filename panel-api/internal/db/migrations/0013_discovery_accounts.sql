-- Up to three test accounts per discovery, sealed together (JSON list of
-- label, username, password). Erased when the discovery ends, like
-- account_secret, which stays for discoveries started before.
ALTER TABLE discoveries ADD COLUMN accounts_secret bytea;
