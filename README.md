# Mortris (daliys-analytics-server)

Standalone analytics server: receives, validates, stores, queries,
exports, and displays anonymous game analytics, without any hosted
analytics or third-party dashboard product. Full spec:
`server_implementation_plan.md` (implementation reference kept alongside
this repo; ping the project owner if you need a copy).

Client in v1: a Unity game on Android. Identity is an anonymous
**installation**, never a person or account — no advertising ID, device
fingerprint, email, name, precise location, or event-row IP is collected.

## Non-negotiables

- No Grafana or any hosted analytics/dashboard product. The dashboard is
  our own React + Apache ECharts app, self-hosted, no third-party runtime
  requests.
- PostgreSQL is the v1 source of truth; a request is acknowledged only
  after its transaction commits.
- Delivery is at least once; stable event IDs + a DB uniqueness
  constraint make retries idempotent.
- No Kafka/NATS/Redis/ClickHouse/Kubernetes/microservices in v1 — one Go
  binary, one PostgreSQL instance, evaluated for change only against
  measured pressure (see the plan's section 16).

See the plan's section 17 ("Decisions an implementation agent must not
silently change") before altering any of the above.

## Layout

```text
cmd/analytics-server/   Single binary: migrate, serve, export-events,
                         parity-report, create-admin subcommands
internal/contracts/     SDK wire types + structural validation, shared by
                         the service and contracts/fixtures_test.go
internal/store/         pgx pool + migration runner
internal/ingest/        Registration, batch ingestion, client policy
                         (section 5) — HTTP-agnostic
internal/adminauth/     Dashboard operator auth: Argon2id, sessions,
                         CSRF, login throttling (section 10.3)
internal/analytics/     Read-only metrics queries behind the dashboard
                         (section 9, 10.1) — all 7 screens' queries
internal/policyadmin/   Kill-switch administration: create/list/delete
                         client_policy_rules, audit-logged
internal/httpapi/       net/http routing, body limits, JSON envelopes,
                         error rendering for both the SDK and dashboard
                         APIs, and serving the embedded dashboard SPA
internal/ratelimit/     In-process token buckets (section 6)
internal/diskstate/     Disk-pressure classification (section 12)
internal/maintenance/   Retention + unactivated-installation cleanup,
                         disk-state monitor
migrations/             Flat, numbered, idempotent SQL migrations
deploy/                 Dockerfile (two-stage: frontend then Go), Compose
                         stack, dev Caddyfile, DB roles
dashboard/               React 19 + TypeScript + Vite dashboard SPA (all 7
                         screens + login). `dashboard/embed.go` go:embeds
                         its `dist/` build into the Go binary (section
                         13.1) — `dist/` is gitignored except a tracked
                         `.gitkeep`, so `go build` still works without
                         Node; `make build`/CI run `npm run build` first.
contracts/openapi.yaml  Authoritative OpenAPI 3.1 spec for /v1 (SDK) and
                         /api/v1 (dashboard) — fully specified
contracts/fixtures/     Valid/invalid request fixtures + manifest.json
                         driving contracts/fixtures_test.go
events/catalog.yaml     Event catalog format + the v1 reserved system events
docs/errors.md          Stable error codes, HTTP status, retry class —
                         SDK and dashboard API
docs/metrics.md         Metric definitions, verified by
                         internal/analytics/metrics_test.go
docs/threat-model.md    Threats and mitigations
docs/data-inventory.md  What's stored, per table, and what's never collected
```

All dashboard screens from the plan's section 10.2 are implemented: Overview,
Event Explorer, Funnel, Installation Retention, Installation Timeline
(admin-only), Event Catalog, System Health, plus kill-switch Policy
administration. Phase S4 (production hardening: TLS/firewall/secrets,
off-host backup, restore drill, load/soak tests) is what's left — see the
plan's section 14 for exit gates per phase.

## Verify

```sh
make lint   # go vet + gofmt check
make test   # go test ./...  (contracts fixtures always run; internal/analytics's
            #   fixture-dataset tests need MORTRIS_TEST_DSN set to a real Postgres,
            #   otherwise they skip)
make build  # bin/analytics-server
```

Local end-to-end verification (no Docker required):

```sh
export MORTRIS_MIGRATOR_DSN=postgres://.../mortris_dev
export MORTRIS_WRITER_DSN=postgres://.../mortris_dev
bin/analytics-server migrate
bin/analytics-server create-admin --email you@example.com --role admin --projects your-project
bin/analytics-server serve
```

## Toolchain

Go 1.26.x (pinned in `go.mod`), PostgreSQL 18.x, `pgx/v5`. Node 22+/npm is
a build-time-only dependency for `dashboard/` (React 19, Vite 8,
TypeScript, react-router-dom, Apache ECharts) — production never runs
Node, only the compiled Go binary with the frontend embedded. Docker/Compose
is optional — `deploy/compose.yaml` runs the full stack, but every
subcommand also runs directly against any reachable PostgreSQL.
