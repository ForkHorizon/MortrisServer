# Monitoring (section 13.4)

## What's watching Mortris right now

- **In-box uptime check**: `mortris.forkhorizon.com/health/ready` (checks
  Postgres connectivity, not just process liveness) was added to
  ForkHorizon's existing `scripts/healthcheck.mjs` /
  `forkhorizon-healthcheck.timer` (every 5 minutes), which already alerts
  to the same Telegram bot ForkHorizon's other products use. Reused
  rather than duplicated — it's the same VPS, same established pattern,
  one less thing to maintain.
- **Structured logs**: `internal/httpapi/logging.go` emits one JSON line
  per request (request ID, route, status, latency) via `log/slog`, never
  including credentials, session/install IDs, request bodies, or property
  values (section 13.4). Captured by journald, capped at 500MB /
  90 days box-wide (`deploy/systemd/journald-mortris-cap.conf`) — there
  was no explicit cap before this, just systemd's implicit default.
- **`GET /api/v1/system`** (System Health screen): DB latency, pool
  stats, disk state, ingestion/rejection rate, last maintenance runs,
  enabled policy rule count — for a human looking, not automated
  alerting.

## The gap — same one ForkHorizon already has and hasn't closed

The in-box healthcheck runs *on the same VPS* it's checking. If the whole
box goes down — not just the Mortris process, the entire machine — nothing
fires, because the thing that would fire is also down. This is exactly
the "watchman watching itself" gap ForkHorizon's own CLAUDE.md documents
for its own four products, and it applies identically here since Mortris
shares the box.

**Not implemented**: a genuinely external dead-man's-switch (e.g.
healthchecks.io, UptimeRobot, or similar) pinging `mortris.forkhorizon.com`
from outside this VPS. This needs a third-party account, which is a real
decision (cost, provider) for whoever owns this deployment to make — set
one up pointed at `https://mortris.forkhorizon.com/health/ready` when
ready, and note it here once it exists.
