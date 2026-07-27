-- Phase 4 catalog governance: per-event rejection tracking and schema
-- drift detection. Both are day-bucketed counters (server-received UTC
-- day, since rejections in particular may have no trusted client clock
-- at all) rather than raw event rows — cheap to upsert per batch and
-- enough for "which event/property is misbehaving, and since when."

CREATE TABLE IF NOT EXISTS event_rejection_stats (
    project_id    text NOT NULL REFERENCES projects(id),
    name          text NOT NULL,
    code          text NOT NULL,
    day           date NOT NULL,
    count         bigint NOT NULL DEFAULT 0,
    last_seen_at  timestamptz NOT NULL,
    PRIMARY KEY (project_id, name, code, day)
);

CREATE TABLE IF NOT EXISTS event_property_drift (
    project_id    text NOT NULL REFERENCES projects(id),
    name          text NOT NULL,
    property_key  text NOT NULL,
    day           date NOT NULL,
    count         bigint NOT NULL DEFAULT 0,
    last_seen_at  timestamptz NOT NULL,
    PRIMARY KEY (project_id, name, property_key, day)
);
