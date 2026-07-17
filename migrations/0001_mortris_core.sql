-- Core schema (server_implementation_plan.md section 8.2/8.3).
-- Every statement must be safe to re-run: CREATE TABLE/INDEX use
-- IF NOT EXISTS. This file will never use a bare ALTER TABLE ADD COLUMN —
-- if a later migration needs one, it must guard the runner for the
-- "duplicate column name" case instead (see internal/store/migrate.go).

CREATE TABLE IF NOT EXISTS schema_migrations (
    version     text PRIMARY KEY,
    applied_at  timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE IF NOT EXISTS projects (
    id              text PRIMARY KEY,
    environment     text NOT NULL,
    display_name    text NOT NULL,
    retention_days  integer NOT NULL DEFAULT 90,
    strict_catalog  boolean NOT NULL DEFAULT true,
    enabled         boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at      timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE IF NOT EXISTS installations (
    project_id              text NOT NULL REFERENCES projects(id),
    install_id              uuid NOT NULL,
    credential_hash         bytea NOT NULL,
    registered_at           timestamptz NOT NULL DEFAULT clock_timestamp(),
    activated_at            timestamptz,
    first_product_event_at  timestamptz,
    last_seen_at            timestamptz,
    last_app_version        text,
    last_build_number       text,
    last_sdk_version        text,
    status                  text NOT NULL DEFAULT 'active',
    PRIMARY KEY (project_id, install_id)
);

-- Supports the unactivated-registration cleanup sweep (section 5.2): find
-- rows older than 7 days that never activated, without scanning the whole
-- table.
CREATE INDEX IF NOT EXISTS idx_installations_unactivated
    ON installations (project_id, registered_at)
    WHERE activated_at IS NULL;

CREATE TABLE IF NOT EXISTS events (
    project_id              text NOT NULL,
    event_id                uuid NOT NULL,
    install_id              uuid NOT NULL,
    session_id              uuid NOT NULL,
    sequence                bigint NOT NULL,
    session_elapsed_ms      bigint NOT NULL,
    name                    text NOT NULL,
    event_kind              text NOT NULL CHECK (event_kind IN ('product', 'system')),
    occurred_at_client      timestamptz NOT NULL,
    sent_at_client          timestamptz NOT NULL,
    received_at             timestamptz NOT NULL DEFAULT clock_timestamp(),
    effective_at            timestamptz NOT NULL,
    clock_skew_ms           bigint NOT NULL,
    time_quality            text NOT NULL CHECK (time_quality IN ('client', 'batch_adjusted', 'untrusted')),
    app_version             text NOT NULL,
    build_number            text NOT NULL,
    platform                text NOT NULL,
    os_version              text NOT NULL,
    device_class            text NOT NULL,
    locale                  text NOT NULL,
    timezone_offset_minutes integer NOT NULL,
    properties              jsonb NOT NULL,
    PRIMARY KEY (project_id, event_id),
    FOREIGN KEY (project_id, install_id) REFERENCES installations(project_id, install_id)
);

-- Deliberately no uniqueness on sequence (section 8.2) — a repeated
-- sequence with different event IDs is a client anomaly to surface, not a
-- reason to discard data.

CREATE INDEX IF NOT EXISTS idx_events_project_effective
    ON events (project_id, effective_at);
CREATE INDEX IF NOT EXISTS idx_events_project_name_effective
    ON events (project_id, name, effective_at);
CREATE INDEX IF NOT EXISTS idx_events_project_install_effective
    ON events (project_id, install_id, effective_at);
-- (project_id, session_id, effective_at) is deliberately NOT added yet —
-- section 8.3 says add it only if the session-duration query benchmark
-- needs it. Add it then, with the EXPLAIN ANALYZE that justified it.

CREATE TABLE IF NOT EXISTS event_catalog (
    project_id            text NOT NULL REFERENCES projects(id),
    name                  text NOT NULL,
    kind                  text NOT NULL CHECK (kind IN ('product', 'system')),
    description           text NOT NULL DEFAULT '',
    owner                 text NOT NULL DEFAULT '',
    first_schema_version  integer NOT NULL DEFAULT 1,
    properties            jsonb NOT NULL DEFAULT '[]',
    first_seen_at         timestamptz,
    last_seen_at          timestamptz,
    created_at            timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at            timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (project_id, name)
);

CREATE TABLE IF NOT EXISTS client_policy_rules (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id          text NOT NULL REFERENCES projects(id),
    environment         text,
    app_version         text,
    build_number        text,
    sdk_version         text,
    mode                text NOT NULL CHECK (mode IN ('active', 'pause_upload', 'disable_collection')),
    next_check_seconds  integer NOT NULL,
    discard_pending     boolean NOT NULL DEFAULT false,
    reason              text NOT NULL DEFAULT '',
    enabled             boolean NOT NULL DEFAULT true,
    created_at          timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at          timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS idx_client_policy_rules_project
    ON client_policy_rules (project_id) WHERE enabled;

CREATE TABLE IF NOT EXISTS admin_users (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email          text NOT NULL UNIQUE,
    password_hash  text NOT NULL,
    role           text NOT NULL CHECK (role IN ('admin', 'viewer')),
    disabled       boolean NOT NULL DEFAULT false,
    created_at     timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- Explicit per-project scoping for admin_users (section 10.3): a row here
-- grants that admin_user access to that project. No rows = no access.
CREATE TABLE IF NOT EXISTS admin_user_projects (
    admin_user_id  bigint NOT NULL REFERENCES admin_users(id),
    project_id     text NOT NULL REFERENCES projects(id),
    PRIMARY KEY (admin_user_id, project_id)
);

CREATE TABLE IF NOT EXISTS admin_sessions (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_user_id  bigint NOT NULL REFERENCES admin_users(id),
    token_hash     bytea NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT clock_timestamp(),
    expires_at     timestamptz NOT NULL,
    last_seen_at   timestamptz NOT NULL DEFAULT clock_timestamp(),
    revoked_at     timestamptz
);

CREATE TABLE IF NOT EXISTS admin_audit_log (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_user_id  bigint REFERENCES admin_users(id),
    action         text NOT NULL,
    detail         jsonb NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE IF NOT EXISTS maintenance_runs (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind           text NOT NULL,
    started_at     timestamptz NOT NULL,
    finished_at    timestamptz,
    rows_affected  bigint NOT NULL DEFAULT 0,
    error          text
);

CREATE TABLE IF NOT EXISTS daily_registration_counters (
    project_id  text NOT NULL REFERENCES projects(id),
    day         date NOT NULL,
    count       bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (project_id, day)
);
