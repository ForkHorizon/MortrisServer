# Multi-project access control

The dashboard has one global **Owner** and project-scoped accounts.

- Owners can create, archive, restore, and permanently remove any project;
  manage every account; and operate SDK test controls.
- Project admins can update the settings, policies, and team membership of
  their assigned projects only. They can create/reset a shared login that is
  exclusive to that project.
- Viewers can see analytics only for their assigned projects.

No membership means no project is returned by `/api/v1/auth/session` and all
analytics requests for it receive `403`. Membership is reloaded for every
authenticated request, so removal takes effect immediately.

## Owner API

- `GET|POST /api/v1/projects` lists active/archived projects and creates an
  immutable generated project ID.
- `POST /api/v1/projects/{id}/archive` and `/restore` stop/re-enable
  collection. `DELETE /api/v1/projects/{id}` permanently removes an archived
  project's data.
- `GET|POST /api/v1/accounts` creates/lists named accounts; `PATCH
  /api/v1/accounts/{id}` resets a password or enables/disables an account.

## Project API

- `GET|PATCH /api/v1/projects/{id}` reads/updates a current project.
- `GET|POST /api/v1/projects/{id}/members` lists or grants membership.
- `PATCH|DELETE /api/v1/projects/{id}/members/{accountID}` resets a permitted
  shared login, changes its project role, or revokes access.

The dashboard uses `username` for new shared logins. Existing owner email
credentials remain accepted during migration.

## SDK test project

An owner may create a `test` environment project with SDK test controls. The
creation response displays its test token once; only a SHA-256 hash is stored.
`POST /api/v1/projects/{id}/sdk-test` changes the active fault scenario.
The ingestion path accepts these controls only for an active test project with
that token, never for a production project.
