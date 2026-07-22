-- Dedicated, anonymous Puzzle gravity-playtest project and revisioned content catalogue.
INSERT INTO projects (id, environment, display_name, retention_days, strict_catalog, enabled)
VALUES ('puzzle_gravity_test', 'test', 'Puzzle gravity playtest', 90, true, true)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS puzzle_content_revisions (
    project_id       text NOT NULL REFERENCES projects(id),
    content_revision text NOT NULL,
    schema_version   integer NOT NULL,
    catalog          jsonb NOT NULL,
    imported_at      timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (project_id, content_revision)
);

CREATE TABLE IF NOT EXISTS puzzle_content_blocks (
    project_id       text NOT NULL,
    content_revision text NOT NULL,
    city_id          integer NOT NULL,
    house_id         integer NOT NULL,
    wave_index       integer NOT NULL,
    block_id         integer NOT NULL,
    order_in_layer   integer NOT NULL,
    visual_key       text NOT NULL DEFAULT '',
    is_ground        boolean NOT NULL DEFAULT false,
    local_x_milli    integer NOT NULL DEFAULT 0,
    local_y_milli    integer NOT NULL DEFAULT 0,
    PRIMARY KEY (project_id, content_revision, city_id, house_id, block_id),
    FOREIGN KEY (project_id, content_revision) REFERENCES puzzle_content_revisions(project_id, content_revision) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_puzzle_content_blocks_scope
    ON puzzle_content_blocks (project_id, content_revision, city_id, house_id, wave_index);

CREATE TABLE IF NOT EXISTS puzzle_content_targets (
    project_id       text NOT NULL,
    content_revision text NOT NULL,
    city_id          integer NOT NULL,
    house_id         integer NOT NULL,
    wave_index       integer NOT NULL,
    target_id        integer NOT NULL,
    compatible_block_ids jsonb NOT NULL,
    local_x_milli    integer NOT NULL DEFAULT 0,
    local_y_milli    integer NOT NULL DEFAULT 0,
    PRIMARY KEY (project_id, content_revision, city_id, house_id, target_id),
    FOREIGN KEY (project_id, content_revision) REFERENCES puzzle_content_revisions(project_id, content_revision) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS puzzle_content_rules (
    project_id       text NOT NULL,
    content_revision text NOT NULL,
    city_id          integer NOT NULL,
    house_id         integer NOT NULL,
    target_id        integer NOT NULL,
    alternative_groups jsonb NOT NULL,
    PRIMARY KEY (project_id, content_revision, city_id, house_id, target_id),
    FOREIGN KEY (project_id, content_revision) REFERENCES puzzle_content_revisions(project_id, content_revision) ON DELETE CASCADE
);

-- The strict project must recognize every client event before a test build runs.
INSERT INTO event_catalog (project_id, name, kind, description, owner, first_schema_version, properties)
VALUES
('puzzle_gravity_test','house_opened','product','Puzzle house became playable','puzzle',1,'[]'),
('puzzle_gravity_test','wave_presented','product','Puzzle wave inventory became visible','puzzle',1,'[]'),
('puzzle_gravity_test','detail_taken','product','Player took a puzzle detail','puzzle',1,'[]'),
('puzzle_gravity_test','placement_resolved','product','Puzzle placement resolved as placed or fell','puzzle',1,'[]'),
('puzzle_gravity_test','hint_used','product','Puzzle hint was applied','puzzle',1,'[]'),
('puzzle_gravity_test','app_backgrounded','product','Puzzle app moved to background','puzzle',1,'[]'),
('puzzle_gravity_test','app_foregrounded','product','Puzzle app returned to foreground','puzzle',1,'[]'),
('puzzle_gravity_test','attempt_closed','product','Puzzle wave attempt closed','puzzle',1,'[]'),
('puzzle_gravity_test','wave_completed','product','Puzzle wave completed','puzzle',1,'[]'),
('puzzle_gravity_test','house_completed','product','Puzzle house completed','puzzle',1,'[]')
ON CONFLICT (project_id, name) DO UPDATE SET
    description = EXCLUDED.description, owner = EXCLUDED.owner, first_schema_version = EXCLUDED.first_schema_version;
