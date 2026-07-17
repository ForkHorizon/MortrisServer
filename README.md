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
cmd/analytics-server/   Single binary entrypoint
internal/contracts/     Wire types + structural validation shared by the
                         service and the fixture tests below
contracts/openapi.yaml  Authoritative OpenAPI 3.1 spec for /v1 (SDK) and
                         /api/v1 (dashboard, stubbed until Phase S2/S3)
contracts/fixtures/     Valid/invalid request fixtures + manifest.json
                         driving contracts/fixtures_test.go
events/catalog.yaml     Event catalog format + the v1 reserved system events
docs/errors.md          Stable error codes, HTTP status, retry class
docs/metrics.md         Metric definitions (SQL implementation: Phase S2/S3)
docs/threat-model.md    Threats and mitigations
docs/data-inventory.md  What's stored, per table, and what's never collected
```

Phases S1+ (registration, ingestion, PostgreSQL schema, dashboard, CLI
tools, production hardening) are tracked and built incrementally — see the
plan's section 14 for exit gates per phase.

## Verify

```sh
make lint   # go vet + gofmt check
make test   # go test ./...  (loads every fixture in contracts/fixtures)
make build  # bin/analytics-server
```

## Toolchain

Go 1.26.x (pinned in `go.mod`). No other runtime dependency yet — Phase S1
adds `pgx/v5` and PostgreSQL.
