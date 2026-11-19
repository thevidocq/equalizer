-- Forgotten password links: only the SHA-256 of the token is kept; a link
-- works once, for one hour.
CREATE TABLE password_resets (
    id           bigserial PRIMARY KEY,
    user_id      bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_sha256 text NOT NULL UNIQUE,
    expires_at   timestamptz NOT NULL,
    used_at      timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX password_resets_user ON password_resets (user_id);
