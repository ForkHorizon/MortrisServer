-- Per-batch accepted/duplicate/rejected counts (section 11, 13.4). The
-- events table alone can't answer "how many duplicates/rejections
-- happened" retroactively — duplicates are never inserted and rejections
-- are never stored — so parity-report needs its own durable counter.

CREATE TABLE IF NOT EXISTS ingestion_stats (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id       text NOT NULL REFERENCES projects(id),
    install_id       uuid NOT NULL,
    received_at      timestamptz NOT NULL DEFAULT clock_timestamp(),
    accepted_count   integer NOT NULL,
    duplicate_count  integer NOT NULL,
    rejected_count   integer NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ingestion_stats_project_received
    ON ingestion_stats (project_id, received_at);
