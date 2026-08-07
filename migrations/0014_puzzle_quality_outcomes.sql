-- Build-qualified ingestion outcomes make current quality ranges exact.
-- Existing aggregate rows remain legacy/unscoped rather than being guessed.
ALTER TABLE ingestion_stats ADD COLUMN IF NOT EXISTS app_version text NOT NULL DEFAULT '';
ALTER TABLE ingestion_stats ADD COLUMN IF NOT EXISTS build_number text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_ingestion_stats_project_build_received
    ON ingestion_stats (project_id, app_version, build_number, received_at);

-- Rejected input has no trusted effective_at. Preserve each received-time
-- outcome instead of deriving arbitrary time windows from daily counters.
CREATE TABLE IF NOT EXISTS event_rejection_occurrences (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id   text NOT NULL REFERENCES projects(id),
    install_id   uuid NOT NULL,
    event_id     text NOT NULL DEFAULT '',
    name         text NOT NULL DEFAULT '',
    code         text NOT NULL,
    app_version  text NOT NULL DEFAULT '',
    build_number text NOT NULL DEFAULT '',
    received_at  timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rejection_occurrences_project_build_received
    ON event_rejection_occurrences (project_id, app_version, build_number, received_at);

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE event_rejection_occurrences TO analytics_writer;
GRANT SELECT ON TABLE event_rejection_occurrences TO analytics_reader;
GRANT USAGE, SELECT ON SEQUENCE event_rejection_occurrences_id_seq TO analytics_writer;
