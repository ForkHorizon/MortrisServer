-- Central dashboard ownership, project-scoped roles, archives, and SDK test
-- controls. The oldest enabled administrator becomes the Owner; existing
-- project grants from administrators become project-admin grants. Existing
-- viewers retain viewer grants.

ALTER TABLE projects ADD COLUMN IF NOT EXISTS archived_at timestamptz;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS sdk_test_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS sdk_test_token_hash bytea;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS sdk_test_scenario text NOT NULL DEFAULT ''
    CHECK (sdk_test_scenario IN ('', 'lost_acknowledgement', 'unauthorized_once', 'payload_too_large', 'rate_limited', 'policy_active', 'policy_pause_upload', 'policy_disable_collection'));

ALTER TABLE admin_users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE admin_users ADD COLUMN IF NOT EXISTS username text;
CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_users_username_unique
    ON admin_users (lower(username)) WHERE username IS NOT NULL;

ALTER TABLE admin_user_projects ADD COLUMN IF NOT EXISTS access_role text NOT NULL DEFAULT 'viewer'
    CHECK (access_role IN ('project_admin', 'viewer'));
UPDATE admin_user_projects p
SET access_role = 'project_admin'
FROM admin_users u
WHERE p.admin_user_id = u.id AND u.role = 'admin';

ALTER TABLE admin_users DROP CONSTRAINT IF EXISTS admin_users_role_check;
UPDATE admin_users
SET role = CASE
    WHEN id = (
        SELECT id FROM admin_users
        WHERE role = 'admin' AND NOT disabled
        ORDER BY created_at, id
        LIMIT 1
    ) THEN 'owner'
    ELSE 'member'
END;
ALTER TABLE admin_users ALTER COLUMN role SET DEFAULT 'member';
ALTER TABLE admin_users ADD CONSTRAINT admin_users_role_check CHECK (role IN ('owner', 'member'));
