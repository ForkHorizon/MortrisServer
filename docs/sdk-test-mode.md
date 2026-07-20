# SDK fault mode

The supported mode is a protected `test` project inside the central Mortris
dashboard. Only the Owner can enable a scenario, and the ingestion path accepts
it only when the request presents that project's one-time test token. Production
projects cannot enable or use these controls.

Create the project from **Projects** with “Enable protected SDK test controls”
checked and environment set to `test`. Save the displayed token immediately;
the server stores only its SHA-256 hash. Select a scenario from the same Owner
screen, then add this header to the Unity test request:

```text
X-Mortris-Test-Token: <central test-project token>
```

The server reads the selected scenario from the protected central project. The
old isolated `sdk-test` site has been retired; its checked-in templates remain
only as historical deployment reference.

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
