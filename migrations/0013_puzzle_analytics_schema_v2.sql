-- Puzzle client schema v2 adds an exact house -> wave -> interaction replay
-- and explicit developer-menu provenance. The playtest project is strict, so
-- both the new event names and every new flat property must be declared before
-- the Android build can upload them.
INSERT INTO event_catalog
    (project_id, name, kind, description, owner, first_schema_version, properties)
VALUES
    ('puzzle_gravity_test','house_run_started','product','Puzzle house run started','puzzle',2,'[]'),
    ('puzzle_gravity_test','wave_attempt_started','product','Puzzle wave attempt started','puzzle',2,'[]'),
    ('puzzle_gravity_test','attempt_recovered','product','Interrupted Puzzle attempt recovered','puzzle',2,'[]'),
    ('puzzle_gravity_test','detail_released','product','Puzzle detail released over the house','puzzle',2,'[]'),
    ('puzzle_gravity_test','detail_returned','product','Puzzle detail returned to inventory','puzzle',2,'[]'),
    ('puzzle_gravity_test','interaction_abandoned','product','Puzzle detail interaction superseded','puzzle',2,'[]'),
    ('puzzle_gravity_test','state_checkpoint','product','Committed Puzzle house state checkpoint','puzzle',2,'[]'),
    ('puzzle_gravity_test','developer_command_started','product','Developer command started','puzzle',2,'[]'),
    ('puzzle_gravity_test','developer_command_completed','product','Developer command completed','puzzle',2,'[]'),
    ('puzzle_gravity_test','developer_command_failed','product','Developer command failed','puzzle',2,'[]'),
    ('puzzle_gravity_test','developer_command_noop','product','Developer command made no change','puzzle',2,'[]'),
    ('puzzle_gravity_test','developer_menu_opened','product','Developer menu opened','puzzle',2,'[]'),
    ('puzzle_gravity_test','developer_menu_closed','product','Developer menu closed','puzzle',2,'[]'),
    ('puzzle_gravity_test','developer_progress_mutated','product','Developer command changed saved progress','puzzle',2,'[]')
ON CONFLICT (project_id, name) DO UPDATE SET
    description = EXCLUDED.description,
    owner = EXCLUDED.owner,
    first_schema_version = EXCLUDED.first_schema_version;

-- The ingest contract validates allowed keys, while individual event shapes
-- remain intentionally sparse. A shared superset preserves schema-v1 queued
-- events and allows developer events that exist outside an active attempt.
UPDATE event_catalog
SET properties = '[
  {"name":"schema_version","type":"number"},
  {"name":"attempt_id","type":"string"},
  {"name":"wave_attempt_id","type":"string"},
  {"name":"house_run_id","type":"string"},
  {"name":"content_revision","type":"string"},
  {"name":"city_id","type":"number"},
  {"name":"house_id","type":"number"},
  {"name":"wave_index","type":"number"},
  {"name":"active_elapsed_ms","type":"number"},
  {"name":"placed_block_count","type":"number"},
  {"name":"attempt_event_index","type":"number"},
  {"name":"house_event_index","type":"number"},
  {"name":"origin","type":"string"},
  {"name":"progress_origin","type":"string"},
  {"name":"interaction_id","type":"string"},
  {"name":"interaction_index","type":"number"},
  {"name":"details_total","type":"number"},
  {"name":"block_id","type":"number"},
  {"name":"inventory_group_id","type":"string"},
  {"name":"details_remaining","type":"number"},
  {"name":"candidate_target_id","type":"number"},
  {"name":"nearest_compatible_target_id","type":"number"},
  {"name":"nearest_target_distance_milli","type":"number"},
  {"name":"release_x_milli","type":"number"},
  {"name":"release_y_milli","type":"number"},
  {"name":"coordinate_space","type":"string"},
  {"name":"outcome","type":"string"},
  {"name":"rule_state","type":"string"},
  {"name":"restored_to_inventory","type":"boolean"},
  {"name":"placed_block_ids","type":"string"},
  {"name":"placed_block_ids_truncated","type":"boolean"},
  {"name":"placed_state_hash","type":"string"},
  {"name":"checkpoint_trigger","type":"string"},
  {"name":"completed_wave_index","type":"number"},
  {"name":"close_reason","type":"string"},
  {"name":"device_total_memory_mb","type":"number"},
  {"name":"graphics_memory_mb","type":"number"},
  {"name":"screen_width_px","type":"number"},
  {"name":"screen_height_px","type":"number"},
  {"name":"app_allocated_memory_mb","type":"number"},
  {"name":"app_reserved_memory_mb","type":"number"},
  {"name":"mono_used_memory_mb","type":"number"},
  {"name":"developer_action_id","type":"string"},
  {"name":"developer_command","type":"string"},
  {"name":"developer_result","type":"string"},
  {"name":"progress_before_hash","type":"string"},
  {"name":"progress_after_hash","type":"string"},
  {"name":"progress_changed","type":"boolean"},
  {"name":"before_state_hash","type":"string"},
  {"name":"after_state_hash","type":"string"},
  {"name":"before_completed","type":"boolean"},
  {"name":"after_completed","type":"boolean"},
  {"name":"before_block_ids","type":"string"},
  {"name":"after_block_ids","type":"string"},
  {"name":"before_completed_waves","type":"string"},
  {"name":"after_completed_waves","type":"string"}
]'::jsonb
WHERE project_id = 'puzzle_gravity_test';
