-- Role bootstrap (section 8.1). Run once per cluster by a superuser:
--   psql -v migrator_password="$MORTRIS_MIGRATOR_PASSWORD" \
--        -v writer_password="$MORTRIS_WRITER_PASSWORD" \
--        -v reader_password="$MORTRIS_READER_PASSWORD" \
--        -v backup_password="$MORTRIS_BACKUP_PASSWORD" \
--        -f deploy/db/roles.sql
-- Passwords are never committed — they come from deployment secret storage
-- (section 13.2) via the -v arguments above. Safe to re-run: role/grant
-- statements are guarded so a second run only rotates passwords.

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'analytics_migrator') THEN
        CREATE ROLE analytics_migrator LOGIN CREATEDB;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'analytics_writer') THEN
        CREATE ROLE analytics_writer LOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'analytics_reader') THEN
        CREATE ROLE analytics_reader LOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'analytics_backup') THEN
        CREATE ROLE analytics_backup LOGIN;
    END IF;
END
$$;

ALTER ROLE analytics_migrator PASSWORD :'migrator_password';
ALTER ROLE analytics_writer   PASSWORD :'writer_password';
ALTER ROLE analytics_reader   PASSWORD :'reader_password';
ALTER ROLE analytics_backup   PASSWORD :'backup_password';

-- analytics_reader is dashboard-only: read-only by default, so a bug in
-- query-building code can't turn into a write.
ALTER ROLE analytics_reader SET default_transaction_read_only = on;

-- analytics_migrator owns the schema; it's the only role that runs
-- migrations/*.sql and is not used by the running service afterward.
GRANT ALL ON SCHEMA public TO analytics_migrator;

-- analytics_writer: registration, ingestion, policy reads, bounded
-- maintenance writes, and dashboard auth (login/session/audit are service
-- writes too, not analytics queries — see analytics_reader below).
-- No DDL.
GRANT USAGE ON SCHEMA public TO analytics_writer;
GRANT SELECT, INSERT, UPDATE, DELETE ON
    projects, installations, events, event_catalog, client_policy_rules,
    maintenance_runs, daily_registration_counters, ingestion_stats,
    admin_users, admin_user_projects, admin_sessions, admin_audit_log
    TO analytics_writer;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO analytics_writer;

-- analytics_reader: dashboard analytics queries only (section 10.1) — read
-- only, statement timeout, no access to auth tables at all. A bug in
-- query-building code can produce neither a write nor a credential leak.
-- Includes maintenance_runs/ingestion_stats for the System Health screen
-- (Phase S3) — still read-only, still no auth-table access.
GRANT USAGE ON SCHEMA public TO analytics_reader;
GRANT SELECT ON
    projects, installations, events, event_catalog, client_policy_rules,
    maintenance_runs, ingestion_stats
    TO analytics_reader;
ALTER ROLE analytics_reader SET statement_timeout = '5s';

-- analytics_backup: minimum privilege for a logical/physical backup tool
-- (pgBackRest or pg_dump). pg_read_all_data is the native predefined role
-- for exactly this (Postgres 14+) — no reason to hand-grant every table.
GRANT pg_read_all_data TO analytics_backup;
