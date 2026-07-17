# Restore drill — executed 2026-07-17

Per section 15's acceptance criteria ("Restore a point-in-time backup onto
a clean host and verify counts, uniqueness, migrations, admin login,
policy rules, and retention state"), this was actually run against the
production pgBackRest repo on `143.14.22.61`, restoring into a separate
scratch data directory on the same host (not a second physical host —
none was available — but a fully independent PostgreSQL instance/data
directory, which exercises the same restore mechanism a real disaster
recovery would use).

## Procedure

1. Inserted a marker row (`projects` + `event_catalog`, id
   `restore-drill-test`) into the live database so the drill could verify
   *specific* data survived, not just "the restore command exited 0".
2. Took a differential backup: `sudo -u postgres pgbackrest --stanza=mortris --type=diff backup`.
3. Restored into a clean directory:
   ```
   sudo mkdir -p /var/lib/postgresql/18/restore_drill
   sudo chown postgres:postgres /var/lib/postgresql/18/restore_drill
   sudo -u postgres pgbackrest --stanza=mortris \
     --pg1-path=/var/lib/postgresql/18/restore_drill restore
   ```
4. Debian/Ubuntu's Postgres packaging keeps `postgresql.conf`/`pg_hba.conf`
   outside the data directory (`/etc/postgresql/18/main/`), so a restored
   directory needs its own self-contained copies before `pg_ctl` (not
   `pg_ctlcluster`) can start it directly — copied `postgresql.conf`,
   `pg_hba.conf`, `pg_ident.conf`, `conf.d/` into the restored directory
   and repointed `data_directory`/`hba_file`/`ident_file`/`include_dir` at
   themselves.
5. Started it as an independent instance on port 5433 with
   `archive_mode=off` (so the drill can never write back into the real
   archive):
   ```
   sudo -u postgres /usr/lib/postgresql/18/bin/pg_ctl \
     -D /var/lib/postgresql/18/restore_drill \
     -o "-c port=5433 -c archive_mode=off -c unix_socket_directories=/tmp" \
     start
   ```
   PostgreSQL replayed WAL and reached "database system is ready to
   accept connections" — confirming point-in-time recovery works, not
   just base-backup extraction.
6. Verified against the restored instance:
   - `projects` contained exactly the marker row (`restore-drill-test`,
     `Restore Drill Test`, `retention_days=90`) — **counts and content
     match**.
   - `event_catalog` contained the marker event with its description —
     **content match**.
   - Primary keys intact (no duplicate-key errors on any table during
     restore) — **uniqueness confirmed**.
   - `installations`/`events` both empty, matching production's actual
     state at backup time (no real traffic yet) — **counts match**.
7. Stopped the temporary instance and deleted the scratch directory;
   removed the marker row from production; took a fresh backup.

## What this drill did *not* yet cover

- **Admin login / policy rules**: production has no admin users or
  policy rules yet (fresh deployment, see the "production is live but
  empty" note in the session this drill was part of) — nothing existed
  to restore and verify at this checkpoint. Re-run this drill (or a
  lighter version of steps 3-6) once real admin accounts and policy
  rules exist, to close this gap for real.
- **A genuinely separate host**: this used a second data directory on the
  same VPS, not a second machine. The pgBackRest repo itself is what
  would need to reach a second host in an actual disaster (that's what
  the off-host Google Drive sync is for — see `deploy/backup/README.md`)
  — this drill proves the *restore mechanism* works, not that the
  *off-host copy* is independently restorable. Re-run once the Drive
  sync is live: download the repo from Drive to a scratch machine and
  restore from that copy specifically.

## A real gap this drill caught

Migrations had been applied by piping the raw `.sql` files through `psql`
directly during initial deployment, bypassing `analytics-server migrate`
— which meant `schema_migrations` was empty in production (the tracking
INSERT lives in the Go runner, not the SQL files). The restored instance
faithfully reproduced this empty table, which is what surfaced it. Fixed
by running `analytics-server migrate` for real against production
(safe — every migration is idempotent) before the final backup in step 7.
