-- Runtime pools use least-privilege roles rather than the migration owner.
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
    puzzle_content_revisions,
    puzzle_content_blocks,
    puzzle_content_targets,
    puzzle_content_rules
TO analytics_writer;

GRANT SELECT ON TABLE
    puzzle_content_revisions,
    puzzle_content_blocks,
    puzzle_content_targets,
    puzzle_content_rules
TO analytics_reader;
