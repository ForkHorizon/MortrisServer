# Threat model

Scope: the analytics server (Go service, PostgreSQL, Caddy) and its two
client surfaces — the untrusted Unity SDK and the internal admin
dashboard. Section references are to server_implementation_plan.md.

## Assets

- Availability of ingestion for legitimate installations.
- Integrity of stored events (no duplication, no cross-installation
  relabeling, no unauthenticated writes).
- Confidentiality of dashboard access and of installation credentials.
- Disk capacity on the single production host.

## Trust boundaries

1. **Unity SDK → server.** Untrusted. The device can be rooted, the app
   modified, the clock wrong, the network hostile. The server never
   assumes an event is truthful, only that it's structurally valid
   (section 2, "Guaranteed truthfulness... is explicitly out of scope").
2. **Dashboard operator → server.** Trusted after authentication, scoped
   by role and project (section 10.3). No public account creation.
3. **Server → PostgreSQL.** Trusted, but scoped by role: `analytics_writer`
   cannot run arbitrary queries the dashboard would need, and
   `analytics_reader` is read-only with a statement timeout (section 8.1).

## Threats and mitigations

| Threat | Mitigation |
|---|---|
| Registration flood exhausts the database or masks real growth metrics. | Per-IP (loose), per-project, and per-day registration caps; unactivated rows are deleted after 7 days so registration alone never inflates "new installations" (section 5.2, 6). |
| Credential/identity hijack — attacker with just an `install_id` tries to take over an installation. | The server never rotates or reveals a credential based on knowledge of `install_id` alone (section 5.3). Credential comparison is constant-time. |
| Compromised or malicious installation floods ingestion. | Per-installation token bucket is the primary limiter; per-IP ingestion limit is a loose emergency breaker only, so CGNAT doesn't collateral-damage legitimate installations (section 6). |
| Malformed/oversized/compressed-bomb request crashes or stalls the service. | Body size limits at Caddy and Go, bounded decompression, strict JSON decoding with unknown-field rejection, read/write/idle timeouts (section 13.2). |
| SQL injection via any client-controlled string (project_id, event name, property values, dashboard filters). | Parameterized SQL only, everywhere — no endpoint accepts free-form SQL, dashboard query dimensions/filters are allowlisted (section 10.1, 13.2). |
| Dashboard session theft or fixation. | `Secure`, `HttpOnly`, `SameSite=Strict` cookies; session rotates on login and privilege change; CSRF token on state-changing requests; login throttling; audit log (section 10.3). |
| Dashboard credential stuffing. | Argon2id password hashing with unique salt, login throttling (section 10.3). |
| Operator with read access exfiltrates the whole dataset. | No endpoint accepts arbitrary SQL; CSV export is admin-only, local-CLI-only (no public export endpoint), and every export is recorded in the audit log with operator/parameters/row-count — though not full DLP (section 11). |
| Disk exhaustion from event or log growth takes the server down uncontrolled. | Staged disk-pressure behavior (Warning/High/Critical/Rejecting) that degrades reads and then ingestion before the disk actually fills, while retention/backups/health checks keep running throughout (section 12). |
| Silent data loss from an unacknowledged write. | PostgreSQL commit precedes acknowledgement, always; `ON CONFLICT DO NOTHING` on `event_id` makes retries idempotent (section 5, decision 5; section 8.2). |
| Total server/network outage goes unnoticed because monitoring runs on the same host. | Off-host uptime probe and off-host backup destination are required before production; the in-host healthcheck/backup timers are not sufficient alone (section 12.4 / 13.3 / 13.4 "watchman watching itself" gap). |
| A property value or log line leaks PII despite the "no PII" product decision. | Structured logs never include credentials, install IDs at info level, raw event bodies, or property values (section 13.4). Property *values* are still product-team-controlled free text — there is no server-side content scrubber in v1; this is a documented residual risk, not a solved one (see docs/data-inventory.md). |

## Explicitly accepted residual risk (not mitigated in v1)

- A sufficiently sophisticated modified client can still submit plausible
  but false event data — the server can validate shape, not truth
  (section 2).
- Property *values* are not scanned for accidental PII; this depends on
  product teams following the "anonymous installation" data policy when
  defining catalog events.
