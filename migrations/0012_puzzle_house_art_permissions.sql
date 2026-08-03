-- Runtime pools use least-privilege roles rather than the migration owner,
-- so every newly created table needs its own grants — same reason
-- 0006_puzzle_gravity_permissions.sql exists for the 0004 tables.
--
-- Without this the art import fails with "permission denied for table
-- puzzle_house_art" even though the migration itself succeeded, because
-- CREATE TABLE grants nothing to anyone but its owner. Caught in
-- production on 2026-08-03; a migration that adds a table but not its
-- grants is only half a migration.
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE puzzle_house_art TO analytics_writer;
GRANT SELECT ON TABLE puzzle_house_art TO analytics_reader;
