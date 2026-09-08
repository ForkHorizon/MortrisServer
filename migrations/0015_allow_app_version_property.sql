-- Compatibility for Puzzle clients that duplicate the top-level app_version
-- inside event properties.
UPDATE event_catalog
SET properties = properties || '[{"name":"app_version","type":"string"}]'::jsonb
WHERE project_id = 'puzzle_gravity_test'
  AND NOT EXISTS (
      SELECT 1
      FROM jsonb_array_elements(properties) AS property
      WHERE property->>'name' = 'app_version'
  );
