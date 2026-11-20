-- Verifications behind test accounts' logins, and the agent's corrections
-- (docs/agent.md). The accounts are sealed like a discovery's and erased
-- when the verification ends.
ALTER TABLE map_verifications
    ADD COLUMN accounts_secret   bytea,
    ADD COLUMN accounts          text[] NOT NULL DEFAULT '{}',
    ADD COLUMN correct           boolean NOT NULL DEFAULT false,
    ADD COLUMN author_id         bigint REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN corrected_version integer,
    ADD COLUMN corrections       jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN before            jsonb;
