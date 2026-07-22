# Puzzle gravity playtest: Mortris handoff

These are uncommitted local Mortris changes for Puzzle's anonymous internal gravity-playtest analytics. The receiving agent should review, commit, and deploy them.

## Intent

The test uses a dedicated strict project called `puzzle_gravity_test`. Puzzle sends semantic gameplay outcomes through the existing durable SDK. Mortris preserves raw events, stores the exact content revision used by the player, and derives reporting and gravity diagnostics server-side. No PII, Unity objects, sprite data, drag frames, rule arrays, or duplicated client counters are collected.

## Changed files

| Path | Change and reason |
|---|---|
| `migrations/0004_puzzle_gravity_diagnostics.sql` | Creates the strict test project and immutable revision, block, target, and placement-rule tables. |
| `migrations/0005_puzzle_gravity_event_properties.sql` | Declares all allowed flat property keys. |
| `migrations/0007_puzzle_gravity_device_profile.sql` | Adds the strict `device_profile` and `memory_sample` event schemas. |
| `internal/ingest/batch_processing.go` | Rejects keys outside a declared strict catalogue. Empty legacy property declarations remain name-only strict. |
| `internal/analytics/puzzle_gravity.go` | Catalogue validation/import, summary/scope/friction/daily reporting, and attempt-rule reconstruction. |
| `internal/analytics/puzzle_players.go` | Builds the anonymous player/device list from raw installation events. |
| `internal/analytics/puzzle_gravity_test.go` | Catalogue-reference and alternative-support tests. |
| `internal/httpapi/server.go` | Registers import and diagnostics routes. |
| `internal/httpapi/puzzle_gravity_handlers.go` | Admin/CSRF protected import and protected raw timeline routes. |
| `dashboard/src/pages/GameplayDiagnosticsPage.tsx` | Designer diagnostics page. |
| `dashboard/src/App.tsx`, `dashboard/src/components/Layout.tsx`, `dashboard/src/api/types.ts` | Route, navigation, and response types. |

## Database setup

Migration `0004` creates `puzzle_gravity_test` with environment `test`, strict catalogue enabled, and 90-day retention. It creates:

- `puzzle_content_revisions`: immutable original import document.
- `puzzle_content_blocks`: stable IDs, visual key, ground flag, layer order, position, wave.
- `puzzle_content_targets`: target positions, wave, compatible block IDs.
- `puzzle_content_rules`: support alternatives.

Events must be interpreted against their own `content_revision`, never the latest house content.

## Events and properties

Seeded events: `house_opened`, `wave_presented`, `detail_taken`, `placement_resolved`, `hint_used`, `app_backgrounded`, `app_foregrounded`, `attempt_closed`, `wave_completed`, `house_completed`, `device_profile`, and `memory_sample`.

Every event has `attempt_id`, `content_revision`, `city_id`, `house_id`, `wave_index`, `active_elapsed_ms`, `placed_block_count`, and monotonic `attempt_event_index`.

`placement_resolved` additionally has selected block/group, candidate and nearest compatible target, quantized distance/release coordinates, `outcome`, `rule_state`, compact `placed_block_ids` when it fits, and `placed_state_hash` always.

Outcomes are `placed`, `fell_no_snap_target`, `fell_missing_support`, `fell_missing_rule`, and `returned`. Rule states are `ground`, `support_satisfied`, `support_unsatisfied`, `rule_missing`, and `no_target`.

### Anonymous player and device telemetry

The player ID is the SDK's persistent random installation UUID (`install_id`), not an account ID, advertising ID, IMEI, or other personal identifier. It lets a project administrator select one device and see that device's raw event timeline.

The SDK already attaches platform, OS version, device model (`device_class`), app/build version, locale, and timezone offset to every event. Puzzle adds one `device_profile` event at the first playable wave with total device RAM, graphics RAM, and screen dimensions. It then emits `memory_sample` only after each cumulative ten minutes of foreground active play, with Unity allocated/reserved memory and Mono used memory. It does not sample per frame or while backgrounded.

## New HTTP API

`POST /api/v1/projects/{projectID}/puzzle-content`

Requires project-admin access and the existing CSRF header. It validates all IDs/references and idempotently imports an immutable catalogue revision. It returns its `content_revision`.

`GET /api/v1/analytics/gameplay/diagnostics?project=puzzle_gravity_test`

Optional filters: `from`, `to`, `timezone`, `city_id`, `house_id`, `wave_index`. Default timezone is `Europe/Madrid`. Returns active/wall time, pause count/duration, city-house-wave scope, friction, and daily rows grouped by revision/build.

`GET /api/v1/analytics/gameplay/attempts/{attemptID}?project=puzzle_gravity_test`

Requires project-admin access. Returns the event timeline and `missing_support_groups` for each placement. Inner groups are AND requirements; outer groups are OR alternatives. If the payload omitted the placed-ID string for size, reconstruction continues from earlier successful placements in that attempt.

`GET /api/v1/analytics/gameplay/players?project=puzzle_gravity_test&from=...&to=...`

Requires project-admin access. Returns at most 500 anonymous installation IDs, last-seen device/app context, device RAM, most-recent sampled app RAM, attempt count, and falls. Selecting an ID uses the existing protected `GET /api/v1/analytics/installations/{installID}` raw timeline route.

## Dashboard

The manager-only `/gameplay` page provides summary, city/house/wave, detail/target friction, daily reporting, an attempt-timeline/reconstruction view, and an anonymous-player/device list that opens raw device events.

## Deployment order

1. Review, commit, and deploy these Mortris changes; run migrations `0004`, `0005`, `0006`, and `0007`.
2. Rebuild/embed the dashboard using the normal deployment process.
3. Grant internal dashboard users access to `puzzle_gravity_test` (project-admin required for import and raw timeline).
4. Import the content catalogue before publishing a test build.
5. Build Puzzle with matching content-revision and opt-in consent assets.
6. Run a scripted unsupported-placement path and check selected block, target, installed state, missing support, fall, and pause/resume timing in `/gameplay`.

## Puzzle-side prerequisites

The catalogue contract is in `Puzzle/Docs/PuzzleAnalyticsContentCatalog.md`. The exporter must supply two test-build assets:

- `Assets/Resources/PuzzleAnalyticsContentRevisions.asset`: city/house to imported revision mapping.
- `Assets/Resources/PuzzleAnalyticsTestConsent.asset`: `CollectionAllowed = true` only for the internally opted-in test cohort.

Without the consent asset, Puzzle fails closed and sends no analytics. Do not use the client’s local JSON-hash fallback in a real test build.

## Local verification

- `go test ./...` passed.
- `npm run build` from `MortrisServer/dashboard` passed.

Review before deployment: confirm migration execution/dashboard embedding in the deployment path, whether 90-day retention is appropriate, and project-admin access for the seeded project. Database-backed integration tests for lifecycle pairing and duplicate SDK uploads are still recommended when a PostgreSQL test fixture is available.
