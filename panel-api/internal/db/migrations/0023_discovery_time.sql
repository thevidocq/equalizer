-- Continuous discovery runs at a chosen hour (and weekday for weekly), in
-- the time zone of the administrator who set it. set_at keeps a schedule
-- from firing at once for a slot that passed before it was set.
ALTER TABLE apps
    ADD COLUMN discovery_hour smallint NOT NULL DEFAULT 3 CHECK (discovery_hour BETWEEN 0 AND 23),
    ADD COLUMN discovery_weekday smallint NOT NULL DEFAULT 1 CHECK (discovery_weekday BETWEEN 0 AND 6),
    ADD COLUMN discovery_zone text NOT NULL DEFAULT 'UTC',
    ADD COLUMN discovery_set_at timestamptz NOT NULL DEFAULT now();
