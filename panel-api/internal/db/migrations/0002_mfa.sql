-- Two-step login with an authenticator app (TOTP) and recovery codes.

ALTER TABLE users
    ADD COLUMN mfa_secret      bytea,                -- encrypted, active
    ADD COLUMN mfa_pending     bytea,                -- encrypted, waiting for the first code
    ADD COLUMN mfa_last_step   bigint NOT NULL DEFAULT 0, -- last accepted time step: a code is never accepted twice
    ADD COLUMN recovery_codes  text[] NOT NULL DEFAULT '{}'; -- SHA-256 of unused recovery codes

ALTER TABLE panel_sessions
    ADD COLUMN mfa_ok boolean NOT NULL DEFAULT false;
