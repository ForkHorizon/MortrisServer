# Stable error codes

Every non-2xx response uses `ErrorResponse` (contracts/openapi.yaml) with a
code from this table. Codes and their retry classification are part of the
wire contract — do not change a code's meaning or retry class without
updating this doc, `internal/contracts/errors.go`, and the Unity SDK
together.

**Retry class** follows section 5.4/12: `retryable` means the SDK keeps the
queued event(s) and retries later; `permanent` means the SDK deletes the
event(s) (after counting, for per-event codes) because retrying cannot
change the outcome.

| Code | HTTP status | Retry class | Meaning |
|---|---|---|---|
| `invalid_request` | 400 | permanent | Envelope fails a structural or business rule not covered by a more specific code below. |
| `unknown_field` | 400 | permanent | An unrecognized field was present (envelope-level: whole request; event-level: that event only). |
| `invalid_credential` | 400 | permanent | `installation_credential` is not 32 bytes of unpadded base64url. |
| `invalid_uuid` | 400 | permanent | An ID field is not a canonical UUIDv4 string. |
| `invalid_timestamp` | 400 | permanent | A timestamp field is not RFC 3339 UTC with millisecond precision. |
| `invalid_batch_size` | 400 | permanent | `events` has fewer than 1 or more than 100 items. |
| `duplicate_event_id_in_batch` | 400 | permanent | Two events in the same request share an `event_id` (section 5.4) — the whole request is rejected before any transaction opens; the client should split the retry, not resend as-is. |
| `invalid_event_name` | — (per-event, inside 200) | permanent | Event `name` isn't lowercase snake_case within 64 characters. |
| `reserved_event_name` | — (per-event, inside 200) | permanent | Event `name` starts with `sys_` but isn't one of the three reserved system events (section 7). |
| `too_many_properties` | — (per-event, inside 200) | permanent | More than 32 property keys. |
| `invalid_property_key` | — (per-event, inside 200) | permanent | A property key exceeds 64 characters. |
| `invalid_property_type` | — (per-event, inside 200) | permanent | A property value is an array, object, or otherwise not string/number/boolean/null. |
| `property_too_large` | — (per-event, inside 200) | permanent | A string property value exceeds 1024 UTF-8 bytes. |
| `properties_too_large` | — (per-event, inside 200) | permanent | The event's encoded properties exceed 8 KiB total. |
| `unknown_event` | — (per-event, inside 200) | permanent | A product event name isn't in the project's event catalog and the project has `strict_catalog` enabled (section 7). In non-strict projects, unknown product events are stored and auto-added to the catalog instead of rejected. |
| `install_conflict` | 409 | permanent | `install_id` is already registered with a different credential — the client must generate a new ID/credential pair (section 5.2/5.3). |
| `unauthorized` | 401 | permanent | Bearer credential missing, malformed, or doesn't match the installation. |
| `rate_limited` | 429 | retryable | A rate limit was hit (section 6). Response includes `Retry-After`. |
| `server_storage_pressure` | 503 | retryable | Server is in the Rejecting disk-pressure state (section 12). Response includes `Retry-After`. |
| `internal_error` | 500 | retryable | Unclassified server-side failure. Never includes SQL or stack trace details (section 5.1). |

Per-event codes (`invalid_event_name`, `reserved_event_name`,
`too_many_properties`, `invalid_property_key`, `invalid_property_type`,
`property_too_large`, `properties_too_large`, and event-scoped
`unknown_field`) never appear as the top-level HTTP error — they appear in
`BatchIngestResponse.rejected[].code` inside an otherwise-200 response,
because valid sibling events in the same batch still commit (section 5.4).

Also retryable per section 5.4, independent of any code above: transport
failures, `408`, any `5xx`, a malformed or truncated success response, and
database unavailability during the request.

## Dashboard API errors

These use the same `ErrorResponse` shape but describe operator-auth
failures (section 10.3), not SDK wire-contract violations —
`internal/adminauth/errors.go` is their source of truth. "Retryable" here
means "the operator can immediately retry," not "an SDK should queue and
resend."

| Code | HTTP status | Meaning |
|---|---|---|
| `invalid_credentials` | 401 | Login email/password didn't match. Deliberately identical whether the account doesn't exist, is disabled, or the password is wrong — never reveals which. |
| `session_invalid` | 401 | Missing, unknown, or revoked session cookie. |
| `session_expired` | 401 | Session hit its idle timeout (30 min) or absolute timeout (12h) (section 10.3). |
| `csrf_mismatch` | 403 | The `X-CSRF-Token` header didn't match the `csrf_token` cookie, or either was missing, on a state-changing request. |
| `forbidden_project` | 403 | The session isn't scoped to the requested `project` (section 10.3: "each scoped to explicit projects"). |
| `forbidden_role` | 403 | The action requires the `admin` role; the session is `viewer` (installation timeline, policy administration — section 10.2 #5, Phase S3 kill-switch). |
| `too_many_attempts` | 429 | Login throttle hit (10/minute per email, 30/minute per source IP). |
| `not_found` | 404 | The requested installation ID or policy rule ID doesn't exist (or doesn't belong to the given project). |
