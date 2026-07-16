# Data inventory

What the server stores, per table (section 8.2), and what it explicitly
never collects (section 1). This is the reference for privacy review and
for anyone asking "do we have X" — if a field isn't listed here, we don't
store it.

## Never collected (section 1)

Advertising ID, Android ID, device fingerprint, email, name, precise
location, or the event-row source IP. `installation_credential` is never
stored in plaintext — only its SHA-256 hash (`installations.credential_hash`).

## `installations`

| Field | Source | Notes |
|---|---|---|
| `project_id`, `install_id` | client-generated at first registration | Primary key. Never reissued for the same anonymous identity — see credential-loss rule (section 5.3). |
| `credential_hash` | SHA-256 of client-generated credential | Plaintext credential never touches disk or logs. |
| `registered_at`, `activated_at`, `first_product_event_at`, `last_seen_at` | server clock | `activated_at` is null until the first product event commits; unactivated rows are deleted after 7 days (section 5.2). |
| `last_app_version`, `last_build_number`, `last_sdk_version` | client-reported, self-attested | Not verified against a store listing — device is untrusted (section 2). |
| `status` | server-derived | Not user/PII data. |

## `events`

| Field | Source | Notes |
|---|---|---|
| `event_id`, `session_id`, `sequence`, `session_elapsed_ms` | client-generated | `event_id` is the idempotency key; no uniqueness constraint on `sequence` (section 8.2) — gaps/repeats are surfaced, not hidden. |
| `name`, `event_kind` | client name, server-assigned kind | `event_kind` is never trusted from the client (section 7). |
| `occurred_at_client`, `sent_at_client`, `received_at`, `effective_at`, `clock_skew_ms`, `time_quality` | client + server clock | Raw client time is kept even when implausible — see section 8.4 for the plausibility/adjustment rule. |
| `app_version`, `build_number`, `platform`, `os_version`, `device_class`, `locale`, `timezone_offset_minutes` | client-reported | Coarse app/device context, not a fingerprint surface — deliberately excludes anything combinable into a unique device signature. |
| `properties` (jsonb) | client-defined, per event catalog | Flat, typed, size-bounded (section 5.4). Product teams must not put PII in a property value — there is no server-side PII scrubbing for property contents in v1. |

## Retention

Raw event rows: 90 days by `received_at` default, per-project
`retention_days` up to that ceiling (section 8.5). Deletion runs
regardless of disk pressure state (section 12) — retention is never
disabled to "solve" a capacity problem.

## Backups

Whole-database encrypted backups leave the server (section 13.3). Because
backups replicate whatever is in the primary tables, nothing in a backup
exceeds what's listed above — there is no separate backup-only data
collection.
