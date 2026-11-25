-- The AI's security study of a discovery is written after the agent's
-- check of the draft, on the map as the check left it: its own state.
-- waiting: the check of the draft is not over; queued: the agent may take
-- it; running, done, failed.
ALTER TABLE discoveries ADD COLUMN study_status text
    CHECK (study_status IN ('waiting', 'queued', 'running', 'done', 'failed'));
ALTER TABLE discoveries ADD COLUMN study_started_at timestamptz;
CREATE INDEX discoveries_study ON discoveries (id) WHERE study_status IN ('waiting', 'queued', 'running');
