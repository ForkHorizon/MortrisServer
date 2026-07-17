# Upgrade/rollback drill — executed 2026-07-17

Per section 15: "Upgrade and roll back one application version with
queued clients retrying."

## Procedure

1. Saved the running binary: `cp bin/analytics-server bin/analytics-server.v0.1.0-backup`.
2. Bumped `internal/analytics.Version` to `"0.1.1-drill"` (local,
   uncommitted — this file is a drill record, not a real release),
   rebuilt, deployed, restarted `mortris.service`.
3. Confirmed the new build was actually running: `strings
   bin/analytics-server | grep -c "0.1.1-drill"` → `1`; health endpoints
   both `200` within ~1s of restart.
4. Registered a real installation (`rollback-drill-test` project) against
   the "new" version via `POST /v1/installs/register` — this is the
   "queued client" stand-in: a real SDK write landing during this
   version.
5. Rolled back: stopped the service, swapped the binary back to the
   `v0.1.0-backup` copy, restarted. Confirmed via `strings` that the
   rolled-back binary does *not* contain `"0.1.1-drill"`.
6. Proved data continuity: the installation registered in step 4 (under
   the "new" version) successfully authenticated a `POST
   /v1/client/policy` request *after* the rollback to the "old" version —
   same database, same schema, the version swap didn't touch data at all.
   This is what "queued clients retrying" actually depends on: retries
   land on whichever binary is running, against the same durable state.
7. Cleaned up the drill's test project/installations, reverted the local
   `Version` edit, and did a final clean `make build` + redeploy from
   tracked source (not the manually-preserved binary) to leave production
   on verifiably-correct code rather than a hand-preserved artifact.
8. Re-verified all 5 ForkHorizon hostnames + `mortris.forkhorizon.com`
   still `200`/`401` as expected after the whole drill.

## What this proved

- A restart (upgrade or rollback) takes ~1 second — well within what an
  SDK's normal retry behavior (section 5.4: retryable on `5xx`,
  transport failures) already tolerates without any special handling.
- Rollback doesn't require a data migration step in the current schema
  (no destructive migration has shipped yet) — this drill doesn't cover
  the harder case of rolling back *across* a schema-changing migration,
  which section 13.1 flags separately ("Destructive migrations require a
  maintenance plan, verified backup, and tested rollback/forward
  recovery"). Re-run a version of this drill specifically when the first
  such migration ships.
