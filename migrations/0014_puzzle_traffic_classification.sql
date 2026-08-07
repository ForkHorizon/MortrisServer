-- Stage 3 (docs/puzzle-analytics-remaining-plan.md section 6): one shared
-- definition of "was this wave attempt / house run touched by a developer
-- command, or is any of its progress non-natural" — reused by every
-- completion, abandonment, duration and funnel query instead of each one
-- re-deriving a slightly different natural/developer split.
--
-- fully_natural is BOOL_AND across every event carrying that ID: false the
-- moment a single event has origin != 'player' (a developer-menu event) or
-- progress_origin != 'natural' (a player event made after a progress-
-- changing developer command, which the client never un-taints). That one
-- boolean is deliberately how both halves of the plan's rule collapse into
-- one check — see the Go package doc in puzzle_traffic.go for the full
-- reasoning, including why a cheated completion's own diagnostic events
-- never join back into the run they cheated (the client closes the
-- attempt and clears the house run before emitting them).
--
-- house_run_id/attempt_id live only inside the JSONB properties payload
-- (schema v2), so both views scan `events` directly; there is no separate
-- house-run/attempt table to join against.

CREATE OR REPLACE VIEW puzzle_wave_attempt_classification AS
SELECT
    project_id,
    properties->>'attempt_id' AS attempt_id,
    (array_agg(properties->>'house_run_id') FILTER (WHERE properties ? 'house_run_id'))[1] AS house_run_id,
    (array_agg((properties->>'city_id')::int))[1] AS city_id,
    (array_agg((properties->>'house_id')::int))[1] AS house_id,
    (array_agg((properties->>'wave_index')::int))[1] AS wave_index,
    (array_agg(install_id))[1] AS install_id,
    BOOL_AND(COALESCE(properties->>'origin', 'player') = 'player'
             AND COALESCE(properties->>'progress_origin', 'natural') = 'natural') AS fully_natural,
    BOOL_OR(name = 'attempt_recovered') AS recovered,
    BOOL_OR(name IN ('wave_completed', 'house_completed')) AS completed,
    BOOL_OR(name = 'attempt_closed') AS closed,
    MIN(effective_at) AS first_event_at,
    MAX(effective_at) AS last_event_at
FROM events
WHERE properties ? 'attempt_id'
GROUP BY 1, 2;

CREATE OR REPLACE VIEW puzzle_house_run_classification AS
SELECT
    project_id,
    properties->>'house_run_id' AS house_run_id,
    (array_agg((properties->>'city_id')::int))[1] AS city_id,
    (array_agg((properties->>'house_id')::int))[1] AS house_id,
    (array_agg(install_id))[1] AS install_id,
    BOOL_AND(COALESCE(properties->>'origin', 'player') = 'player'
             AND COALESCE(properties->>'progress_origin', 'natural') = 'natural') AS fully_natural,
    BOOL_OR(name = 'house_completed') AS completed,
    MIN(effective_at) AS first_event_at,
    MAX(effective_at) AS last_event_at,
    -- Stage 4 house-coverage view (section 7) shows "last natural play
    -- build" per house; ordering by effective_at picks the build of this
    -- run's most recent event, not an arbitrary aggregation order.
    (array_agg(build_number ORDER BY effective_at DESC))[1] AS last_build_number
FROM events
WHERE properties ? 'house_run_id'
GROUP BY 1, 2;

GRANT SELECT ON puzzle_wave_attempt_classification, puzzle_house_run_classification TO analytics_reader, analytics_writer;
