# Deploy configs

Production runs on `143.14.22.61` — the **same VPS as ForkHorizon**
(forkhorizon.com, CI Scope, House Puzzle, Finance), sharing one nginx
config and one TLS certificate (now covering 6 hostnames) across both
repos. This was a deliberate deviation from section 3's original "Docker
Compose + Caddy" tech baseline: the box has no Docker installed, only
3.8GB RAM shared across five live products, and nginx/certbot/systemd
already work there — see the commit history around 2026-07-17 for the
decision record. `Caddyfile` and `compose.yaml` in this directory are
kept for **local dev only** (`docker compose -f deploy/compose.yaml up`)
— they are not what's running in production.

## Files

- `nginx/mortris.conf` → `/etc/nginx/sites-available/mortris` (symlinked
  from `sites-enabled/`) — HTTP+HTTPS server blocks for
  `mortris.forkhorizon.com`, proxying to the app on `127.0.0.1:8090`.
- `systemd/mortris.service` → `/etc/systemd/system/mortris.service` — the
  Go binary itself (`analytics-server serve`).
- `systemd/mortris-backup-full.{service,timer}`,
  `mortris-backup-diff.{service,timer}`,
  `mortris-backup-sync.{service,timer}` → `/etc/systemd/system/` — see
  `backup/README.md`.
- `systemd/journald-mortris-cap.conf` →
  `/etc/systemd/journald.conf.d/mortris-cap.conf` — box-wide journal size
  cap (500MB / 90 days), there was no explicit one before.
- `db/roles.sql` → run against the `mortris` database (see its own header
  comment for the two-pass ordering requirement around migrations).
- `pgbackrest/pgbackrest.conf` → `/etc/pgbackrest.conf` (note: **not**
  `/etc/pgbackrest/pgbackrest.conf` — the Debian package's default is the
  flat file, not the directory, despite pgBackRest's own docs suggesting
  otherwise).
- `backup/` — off-host backup setup, restore drill record.
- `runbooks-executed/` — drills actually run against production, with
  real commands and real output, not just theoretical procedures.
- `loadtest/` — a standalone Go module (own `go.mod`, deliberately not
  part of the main build) for section 15's capacity acceptance criteria.
  Not yet run against production at real scale — see
  `runbooks-executed/` (or its absence) for status.

## Rebuilding Mortris's portion of the VPS from scratch

Assumes ForkHorizon is already running here (or being rebuilt alongside —
see that repo's own `deploy/README.md` first if starting from nothing).

1. Point a DNS A record for `mortris.forkhorizon.com` at the VPS's IP.
2. Install PostgreSQL 18 (`apt install postgresql-18` — already in
   Ubuntu 26.04's repos, no PGDG needed) and Go 1.26.4 (official tarball
   to `/usr/local/go`, matching `go.mod`'s pin — apt's Go package is
   usually stale).
3. `CREATE DATABASE mortris;` as the postgres superuser, then run
   `db/roles.sql` **twice**: once before migrations (creates the roles),
   once after (grants table privileges) — see that file's header.
4. rsync this repo to `/opt/mortris` (not `git clone` — the VPS has no
   GitHub auth configured, same reason ForkHorizon deploys via rsync).
5. `make build` in `/opt/mortris` (needs Node 22+ for the dashboard
   build, already present for ForkHorizon).
6. `analytics-server migrate` (as `MORTRIS_MIGRATOR_DSN`) — **use the
   real subcommand, not a raw `psql -f migrations/*.sql` pipe**, or
   `schema_migrations` silently stays empty (this happened once during
   the actual 2026-07-17 deploy — see `runbooks-executed/`).
7. Create `/etc/mortris/env` (root-owned, `chmod 600`) with
   `LISTEN_ADDR=127.0.0.1:8090`, `MORTRIS_MIGRATOR_DSN`,
   `MORTRIS_WRITER_DSN`, `MORTRIS_READER_DSN` (see `internal/config` for
   the full list), `MORTRIS_DISK_PATH=/`.
8. Copy `systemd/mortris.service` and
   `systemd/journald-mortris-cap.conf` into place,
   `systemctl daemon-reload`, `systemctl restart systemd-journald`,
   `systemctl enable --now mortris.service`.
9. Copy `nginx/mortris.conf` into `/etc/nginx/sites-available/mortris`,
   symlink into `sites-enabled/`, `nginx -t`, `systemctl reload nginx`.
10. Run certbot for **all hostnames on this box in one invocation** with
    `--expand` (never per-domain — see the gotcha in ForkHorizon's root
    `CLAUDE.md`, which applies here too since the cert is shared):
    `sudo certbot --nginx --expand -d forkhorizon.com,www.forkhorizon.com,ci.forkhorizon.com,housepuzzle.forkhorizon.com,finance.forkhorizon.com,mortris.forkhorizon.com`.
11. `sudo -u postgres pgbackrest --stanza=mortris stanza-create`, enable
    `archive_mode`/`archive_command` in postgresql.conf (see
    `backup/README.md`), take an initial full backup, enable
    `mortris-backup-full.timer` and `mortris-backup-diff.timer`.
12. Set up `rclone` + Google Drive per `backup/README.md`'s interactive
    steps, then enable `mortris-backup-sync.timer`.
13. `analytics-server create-admin` for the first real admin user.
14. Add `mortris.forkhorizon.com` to ForkHorizon's
    `scripts/healthcheck.mjs` `CHECKS` array if it's not already there
    (it should be, in the ForkHorizon repo, not this one).
