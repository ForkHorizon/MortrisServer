# Staging SDK fault mode

This mode exists only for exercising the Unity SDK's retry and policy paths.
It must run as a separate staging deployment with its own database, hostname,
and project. Do not point it at the production database or enable it for
`mortris-prod`.

The checked-in deployment templates use `sdk-test.mortris.forkhorizon.com`,
`/opt/mortris-sdk-test`, and the `mortris_sdk_test` database. Create the DNS
record before running Certbot, then issue its certificate after nginx has the
HTTP-only template enabled.

The process refuses to start with test mode enabled unless all of these are
true:

- `MORTRIS_DEPLOYMENT=staging`
- `MORTRIS_SDK_TEST_MODE=1`
- `MORTRIS_SDK_TEST_PROJECT=mortris-sdk-test` (or another dedicated project)
- `MORTRIS_SDK_TEST_TOKEN` contains at least 16 bytes

Create the configured test project with `environment='staging'`,
`strict_catalog=false`, and `enabled=true`. Register the Unity installation
normally, then add both headers to the test request:

```text
X-Mortris-Test-Token: <staging token>
X-Mortris-Test-Scenario: <scenario>
```

The token and project must both match; otherwise the headers do nothing.

| Scenario | Endpoint | Expected behavior |
|---|---|---|
| `lost_acknowledgement` | batch | Stores the events, then aborts before sending an acknowledgement. Retrying the same event IDs returns them in `duplicates`. |
| `unauthorized_once` | batch | Returns `401 unauthorized` once per installation. Re-registration is normal; the next retry is accepted. |
| `payload_too_large` | batch | Returns `413 payload_too_large` without storing the batch. |
| `rate_limited` | batch | Returns `429 rate_limited` with `Retry-After: 2`. |
| `policy_active` | batch, policy | Returns `client_policy.mode=active`. |
| `policy_pause_upload` | batch, policy | Returns `client_policy.mode=pause_upload`. |
| `policy_disable_collection` | batch, policy | Returns `client_policy.mode=disable_collection` and `discard_pending=true`. |

`lost_acknowledgement` closes the upstream response after persistence. A proxy
may surface that to the SDK as a connection error or a gateway error; either
is deliberately treated as an unacknowledged delivery by the SDK contract.
