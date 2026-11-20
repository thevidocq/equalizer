-- Maps no longer have a "risk" field: it was decorative (nothing enforced
-- it). Stored documents lose it; the signed payloads the gateways load are
-- left as they are (the gateway ignores the field). A field or parameter
-- named "risk" holds an object and stays.
CREATE FUNCTION eq_drop_risk(j jsonb) RETURNS jsonb LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
    out jsonb;
    k text;
    v jsonb;
BEGIN
    IF jsonb_typeof(j) = 'object' THEN
        out := '{}'::jsonb;
        FOR k, v IN SELECT * FROM jsonb_each(j) LOOP
            CONTINUE WHEN k = 'risk' AND v IN ('"low"', '"medium"', '"high"');
            out := out || jsonb_build_object(k, eq_drop_risk(v));
        END LOOP;
        RETURN out;
    ELSIF jsonb_typeof(j) = 'array' THEN
        SELECT coalesce(jsonb_agg(eq_drop_risk(e) ORDER BY n), '[]'::jsonb) INTO out
        FROM jsonb_array_elements(j) WITH ORDINALITY AS a(e, n);
        RETURN out;
    END IF;
    RETURN j;
END
$$;

UPDATE map_versions SET document = eq_drop_risk(document) WHERE document::text LIKE '%"risk"%';

DROP FUNCTION eq_drop_risk(jsonb);
