-- Capability and memory telemetry for the opted-in gravity-test project only.
-- Both event schemas include the existing common attempt fields plus only the
-- fields their corresponding client event emits.
WITH base_schema AS (
    SELECT properties
    FROM event_catalog
    WHERE project_id = 'puzzle_gravity_test' AND name = 'house_opened'
    LIMIT 1
)
INSERT INTO event_catalog (project_id, name, kind, description, owner, first_schema_version, properties)
SELECT 'puzzle_gravity_test', 'device_profile', 'product',
       'Anonymous device capacity for the gravity test', 'puzzle', 1,
       properties || '[
         {"name":"device_total_memory_mb","type":"number"},
         {"name":"graphics_memory_mb","type":"number"},
         {"name":"screen_width_px","type":"number"},
         {"name":"screen_height_px","type":"number"}
       ]'::jsonb
FROM base_schema
ON CONFLICT (project_id, name) DO UPDATE SET
    description = EXCLUDED.description,
    owner = EXCLUDED.owner,
    properties = EXCLUDED.properties;

WITH base_schema AS (
    SELECT properties
    FROM event_catalog
    WHERE project_id = 'puzzle_gravity_test' AND name = 'house_opened'
    LIMIT 1
)
INSERT INTO event_catalog (project_id, name, kind, description, owner, first_schema_version, properties)
SELECT 'puzzle_gravity_test', 'memory_sample', 'product',
       'Ten-minute active-play Unity memory sample', 'puzzle', 1,
       properties || '[
         {"name":"app_allocated_memory_mb","type":"number"},
         {"name":"app_reserved_memory_mb","type":"number"},
         {"name":"mono_used_memory_mb","type":"number"}
       ]'::jsonb
FROM base_schema
ON CONFLICT (project_id, name) DO UPDATE SET
    description = EXCLUDED.description,
    owner = EXCLUDED.owner,
    properties = EXCLUDED.properties;
