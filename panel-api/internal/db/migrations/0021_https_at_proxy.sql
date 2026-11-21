-- https may end at a proxy or CDN in front of Equalizer, which reaches it
-- in http: addresses in https, no certificate here.
ALTER TABLE apps DROP CONSTRAINT apps_public_scheme_check;
ALTER TABLE apps ADD CONSTRAINT apps_public_scheme_check CHECK (public_scheme IN ('https', 'https_proxy', 'http'));
