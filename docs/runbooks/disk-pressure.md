# Disk-pressure runbook (section 12)

This runbook is for `mortris.forkhorizon.com`'s host, `143.14.22.61` — a
48GB disk **shared** with ForkHorizon, CI Scope, House Puzzle, and
Finance. Baseline as of 2026-07-17: 16GB used / 32GB free (33%),
Postgres data 96MB, pgBackRest repo 14MB — Mortris itself is a tiny
fraction of current usage; the other four products' data (plus the OS,
Go/Node toolchains, and this repo's own checkout) account for the rest.

`internal/diskstate` evaluates `MORTRIS_DISK_PATH` (`/` in production)
against both percent-used and absolute-free-bytes thresholds, taking
whichever is more conservative — see `internal/diskstate/diskstate.go`
for the exact table. The states, in escalating order:

| State | Trigger | What Mortris does automatically |
|---|---|---|
| Normal | <70% used, ≥15GB free | Nothing — normal operation. |
| Warning | ≥70% used or <15GB free | Nothing yet — `GET /api/v1/system`'s `disk_state` field turns non-`normal`; the System Health screen surfaces an alert banner. No automated action. |
| High | ≥80% used or <10GB free | Still nothing automated in the current implementation — see "What's NOT automated" below. |
| Critical | ≥85% used or <7.5GB free | Same — surfaced via `/api/v1/system`, no automated throttling yet. |
| Rejecting | ≥90% used or <5GB free | **`POST /v1/events/batch` returns `503 server_storage_pressure`** with `Retry-After: 300` (`internal/ingest/batch.go`) — the one state that *is* automated. Registration is not currently gated the same way (see gap below). |

## What's NOT automated (gap vs. the plan's full table)

Section 12 describes High/Critical states also reducing retention job
frequency, export/query concurrency, and disabling nonessential
maintenance — none of that exists yet; only the Rejecting-state ingestion
gate is implemented. Retention deletion and backups are unconditional
(always run) regardless of disk state, which matches the plan's explicit
requirement ("Retention deletion is essential and must never be disabled
by disk pressure") — that part is correct by construction, not by
oversight. The gap is specifically the *graduated* response (High/Critical
throttling reads/exports) — add it if disk pressure on this box ever
becomes a recurring reality rather than a theoretical table.

## Manual response, by state

**Warning/High**: Check what's growing. On this shared box, first
determine whether it's Mortris (`sudo du -sh /var/lib/postgresql/18/main
/var/lib/pgbackrest`) or one of the other four products
(`/opt/forkhorizon`, `/var/lib/forkhorizon`) before assuming it's Mortris's
problem. Check `journalctl --disk-usage` too (capped at 500MB per
`deploy/systemd/journald-mortris-cap.conf`, but confirm the cap is
actually being enforced).

**Critical**: Same investigation, more urgency. Consider triggering an
off-host backup sync manually (`sudo systemctl start
mortris-backup-sync.service`) before doing anything else, so a recovery
path exists no matter what happens next. Do not manually shorten
`retention_days` on any project as a quick fix — the plan is explicit
this must be a deliberate decision, not a panic response to disk pressure.

**Rejecting**: Mortris is already refusing ingestion by itself at this
point (client SDKs queue and retry per section 5.4 — no data loss as long
as this doesn't last longer than a device's local queue can hold). This
is when to actually act:
1. Confirm backups and WAL archiving are still succeeding —
   `sudo -u postgres pgbackrest --stanza=mortris check` and `df -h` again
   after any cleanup, since retention/backup work is exempted from the
   pressure gate and needs headroom to keep running.
2. Find what's growing (`sudo du -sh /* 2>/dev/null | sort -rh | head`).
3. If it's genuinely Mortris outgrowing its retention window: this is
   the deliberate-decision point the plan calls for — expand the disk
   (this box is a HostUp VPS; resizing is a control-panel action, see
   the panel screenshot context from this deployment) or explicitly
   approve a `retention_days` reduction for the affected project(s), not
   both reflexively.
4. If it's a runaway log or a different product's data: fix that
   directly; Mortris's gate lifts automatically once free space recovers
   past the Rejecting threshold (no restart needed — `internal/maintenance`
   re-evaluates disk state every 30 seconds).
