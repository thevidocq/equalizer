-- Deleting a site removes everything that belongs to it. The app-referencing
-- foreign keys that were not already ON DELETE CASCADE are switched to it, so
-- a single DELETE FROM apps takes its maps, connectors, discoveries, reviews,
-- verifications and recorded hits with it. The inter-child key
-- (map_verifications -> map_versions) is NO ACTION, checked at end of
-- statement, so it is satisfied when both rows are cascaded together.
--
-- The real constraint name is read from the catalogue (it is not assumed),
-- then dropped and re-added as ON DELETE CASCADE with a stable name.

DO $$
DECLARE
    child  text;
    cname  text;
    tables text[] := ARRAY['connectors', 'map_versions', 'map_reviews', 'discoveries', 'unmapped_hits', 'map_verifications'];
BEGIN
    FOREACH child IN ARRAY tables LOOP
        SELECT con.conname INTO cname
        FROM pg_constraint con
        JOIN pg_class rel ON rel.oid = con.conrelid
        JOIN pg_class frel ON frel.oid = con.confrelid
        WHERE con.contype = 'f'
          AND rel.relname = child
          AND frel.relname = 'apps'
          AND (SELECT attname FROM pg_attribute WHERE attrelid = con.conrelid AND attnum = con.conkey[1]) = 'app_id'
        LIMIT 1;

        IF cname IS NOT NULL THEN
            EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', child, cname);
            EXECUTE format(
                'ALTER TABLE %I ADD CONSTRAINT %I FOREIGN KEY (app_id) REFERENCES apps (id) ON DELETE CASCADE',
                child, child || '_app_id_fkey');
        END IF;
    END LOOP;
END $$;
