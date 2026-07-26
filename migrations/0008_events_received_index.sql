-- Live Event Feed (Phase 1a) reads newest-first on received_at, not
-- effective_at (which idx_events_project_effective already covers) —
-- received_at is server receipt time and is what "live tail" means.
--
-- Plain CREATE INDEX, not CONCURRENTLY: the migration runner
-- (internal/store/migrate.go) wraps each file in a transaction, and
-- Postgres disallows CONCURRENTLY inside one. On a large prod events
-- table this will briefly lock writes — if that matters, run the
-- CONCURRENTLY version by hand outside a migration transaction first,
-- then let this file's IF NOT EXISTS no-op.
CREATE INDEX IF NOT EXISTS idx_events_project_received
    ON events (project_id, received_at DESC);
