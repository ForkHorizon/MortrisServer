# Unity SDK integration

The SDK talks only to these three HTTPS endpoints. All JSON is UTF-8, all
timestamps are UTC RFC 3339 with millisecond precision, and every success or
error body includes `server_time`.

| Endpoint | Purpose | Authentication |
|---|---|---|
| `POST /v1/installs/register` | Idempotently register a persisted `install_id` and 32-byte base64url credential. | Request body credential |
| `POST /v1/events/batch` | Deliver 1–100 queued events with `Content-Encoding: gzip`. | `Authorization: Bearer <installation_credential>` |
| `POST /v1/client/policy` | Fetch the effective collection/upload policy without sending events. | `Authorization: Bearer <installation_credential>` |

## Delivery rules

- Delete an event only after its ID appears in `accepted`, `duplicates`, or
  `rejected`; a sent ID omitted from all three stays queued.
- Process acknowledgements before applying the returned `client_policy`.
- On `401`, retry idempotent registration once with the same installation
  identity. `409 install_conflict` is a fatal pause: retain the queue and do
  not overwrite that identity.
- Retry network failures, malformed/truncated `200`s, `408`, `429`, and all
  `5xx` responses. Honour `Retry-After` on `429` and storage-pressure `503`.
- On `413 payload_too_large`, halve the batch; if one event cannot fit, reject
  that local event permanently.

## Stable SDK codes

| Code | Location | Client action |
|---|---|---|
| `invalid_request` | HTTP error or per-event rejection | Permanent for that envelope/event; do not spin. |
| `unknown_field` | HTTP error or per-event rejection | Permanent for that envelope/event. |
| `invalid_credential` | Registration error | Permanent; generate a new installation identity. |
| `invalid_uuid` | HTTP error or `rejected[]` | Permanently delete the rejected event, or rebuild the invalid envelope. |
| `invalid_timestamp` | HTTP error or `rejected[]` | Permanently delete the rejected event, or rebuild the invalid envelope. |
| `invalid_batch_size` | HTTP error | Split or rebuild the batch. |
| `duplicate_event_id_in_batch` | HTTP error | Split/rebuild; do not resend the identical batch. |
| `install_conflict` | HTTP 409 | Fatal pause; retain queue and identity. |
| `unauthorized` | HTTP 401 | Re-register once with the same identity, then retry later if still failing. |
| `rate_limited` | HTTP 429 | Retry after `Retry-After`. |
| `payload_too_large` | HTTP 413 | Halve batch; permanently reject an impossible one-event body. |
| `server_storage_pressure` | HTTP 503 | Retry after `Retry-After`. |
| `internal_error` | HTTP 500 | Retry. |
| `invalid_event_name` | `rejected[]` | Permanently delete that event. |
| `reserved_event_name` | `rejected[]` | Permanently delete that event. |
| `too_many_properties` | `rejected[]` | Permanently delete that event. |
| `invalid_property_key` | `rejected[]` | Permanently delete that event. |
| `invalid_property_type` | `rejected[]` | Permanently delete that event. |
| `property_too_large` | `rejected[]` | Permanently delete that event. |
| `properties_too_large` | `rejected[]` | Permanently delete that event. |
| `unknown_event` | `rejected[]` | Permanently delete that event. |

`client_policy.mode` is `active`, `pause_upload`, or `disable_collection`.
Clamp `next_check_seconds` locally to five minutes through 24 hours. Local
game consent always wins: a server policy can reduce collection but never
override a local opt-out.
