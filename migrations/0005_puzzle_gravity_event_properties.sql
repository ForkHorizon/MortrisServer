-- A non-empty declaration makes strict_catalog validate flat payload keys too.
-- Required base keys are checked by the test build contract; the server keeps
-- raw rows unchanged and only rejects keys outside this allowlist.
UPDATE event_catalog
SET properties = '[
  {"name":"attempt_id","type":"string","required":true},
  {"name":"content_revision","type":"string","required":true},
  {"name":"city_id","type":"number","required":true},
  {"name":"house_id","type":"number","required":true},
  {"name":"wave_index","type":"number","required":true},
  {"name":"active_elapsed_ms","type":"number","required":true},
  {"name":"placed_block_count","type":"number","required":true},
  {"name":"attempt_event_index","type":"number","required":true},
  {"name":"details_total","type":"number"},
  {"name":"block_id","type":"number"},
  {"name":"inventory_group_id","type":"string"},
  {"name":"details_remaining","type":"number"},
  {"name":"candidate_target_id","type":"number"},
  {"name":"nearest_compatible_target_id","type":"number"},
  {"name":"nearest_target_distance_milli","type":"number"},
  {"name":"release_x_milli","type":"number"},
  {"name":"release_y_milli","type":"number"},
  {"name":"outcome","type":"string"},
  {"name":"rule_state","type":"string"},
  {"name":"placed_block_ids","type":"string"},
  {"name":"placed_block_ids_truncated","type":"boolean"},
  {"name":"placed_state_hash","type":"string"},
  {"name":"completed_wave_index","type":"number"},
  {"name":"close_reason","type":"string"}
]'::jsonb
WHERE project_id = 'puzzle_gravity_test';
