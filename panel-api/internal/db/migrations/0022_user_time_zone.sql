-- Each person sees times in their own time zone (an IANA name); UTC until
-- they choose one.
ALTER TABLE users ADD COLUMN time_zone text NOT NULL DEFAULT 'UTC';
