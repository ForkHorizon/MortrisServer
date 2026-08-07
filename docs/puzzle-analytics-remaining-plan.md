# Puzzle analytics: remaining implementation plan after telemetry v2

Status date: 2026-08-06  
Primary project: `puzzle_gravity_test`  
Server repository: `/Users/daliys/Daliys/MortrisServer`  
Unity repository: `/Users/daliys/Daliys/UnityProjects/Puzzle`

This document is the implementation handoff for the work that remains after
the telemetry-v2 foundation was proven on a real Android emulator against
production. It is deliberately explicit. The receiving agent should not infer
missing product rules, silently reinterpret identifiers, or start coding from
an older plan without reconciling it with this baseline first.

The intended outcome is still simple: a designer should be able to open the
Puzzle analytics site and understand which houses are difficult, which wave or
detail causes the difficulty, what the player did, whether the evidence is
trustworthy, and whether tester cheat actions affected the result.

## 1. Executive status

### Stage 1 is complete

The event foundation is implemented and deployed:

- The Unity client emits schema-v2 semantic gameplay events.
- A player interaction is traceable through take, release, placement result,
  return or abandonment using `interaction_id`.
- A wave attempt is traceable through `attempt_id` / `wave_attempt_id` and
  monotonic `attempt_event_index`.
- A full house run is traceable through `house_run_id` and
  `house_event_index`.
- State is periodically anchored by `state_checkpoint`,
  `placed_block_ids`, and `placed_state_hash`.
- Release positions are in `house_local` coordinates.
- Events carry the imported content revision rather than the per-house
  development fallback hash.
- Developer-menu commands have start, result and mutation events connected by
  `developer_action_id`.
- Natural, developer-modified and developer-reset progress are distinguished
  by `progress_origin`.
- Mortris strict-catalog migration `0013_puzzle_analytics_schema_v2.sql` is
  deployed and allows all 24 client event names and all 56 schema-v2 property
  keys.
- Production was reset before the new smoke test. Old tester telemetry no
  longer participates in current analytics.

Production proof from the first clean Android run:

- App version: `1.1.11`.
- Build number: `9dccf3d0738247059cafde1c16106d70`.
- One fresh installation sent 110 events in three batches: 50, 41 and 19.
- Accepted: 110. Rejected: 0. Duplicates: 0.
- SDK sequence was exactly 1 through 110 with no gaps or duplicates.
- Seven attempts each had a contiguous `attempt_event_index` beginning at 0.
- Seventeen detail interactions were complete; no orphan interaction was
  found.
- Four deliberate falls were recorded and returned to inventory:
  `fell_missing_support` and `fell_no_snap_target` were both observed.
- Two developer actions were fully paired:
  `CompleteHouse99` and `ResetHouse`, each with start, completion, progress
  mutation, before/after hashes, and its own action ID.
- Aggregate gameplay queries already exclude events whose origin/progress is
  not natural. Attempt replay keeps developer events visible and labels them.

The pre-reset data is recoverable only if it is ever needed for debugging. It
is stored on production at:

`/opt/mortris/backups/puzzle-reset-20260806T162904Z`

Do not restore that data into the live project for ordinary analysis. It uses
the earlier schema and would contaminate the clean cutover.

### What is not complete

The system can now collect trustworthy event chains, and the main visual house
views already exist. What remains is to turn the new event fidelity into a
complete, safe decision workflow:

1. a visible data-quality control plane;
2. exact natural-versus-cheat segmentation at the correct aggregation level;
3. house-run and wave funnels/coverage;
4. the remaining supporting visual diagnostics;
5. device and memory views;
6. repeatable QA, monitoring and release gates;
7. scale work only after production measurements justify it.

## 2. Documents and code that define the current system

Read these before changing anything:

- `docs/puzzle-visual-analytics-plan.md` explains the visual design and the
  city → house → wave → detail → attempt drill path.
- `docs/puzzle-visual-analytics-handoff.md` describes the geometry/art
  deployment and the existing dashboard surfaces. Parts of its client-state
  section are historical and are superseded by this document.
- `docs/puzzle-gravity-playtest-handoff.md` explains the original anonymous
  playtest contract. Its old event list is incomplete for schema v2.
- `migrations/0013_puzzle_analytics_schema_v2.sql` is the authoritative server
  catalogue for the current client property set.
- `internal/analytics/puzzle_*.go` contains the current derivations.
- `dashboard/src/pages/HousesPage.tsx` is the visual entry point.
- `dashboard/src/pages/HouseDetailPage.tsx` and
  `dashboard/src/pages/houseDetailParts.tsx` implement the house drill-down.
- `dashboard/src/components/HouseCanvas.tsx` draws geometry, art, drops,
  support dependencies and replay state.
- `dashboard/src/components/AttemptPicker.tsx` and `ReplayPlayer.tsx` expose
  individual attempts.
- `dashboard/src/pages/GameplayDiagnosticsPage.tsx` is the older table-heavy
  diagnostics surface. Do not independently extend it without deciding how it
  fits the visual flow; otherwise two conflicting analytics products will
  emerge.

Unity sources relevant to telemetry:

- `Assets/Scripts/DI/Services/Analytical/PuzzleAnalyticsService.cs`
- `Assets/Scripts/DI/Services/Analytical/PuzzleAnalyticsLifecycle.cs`
- `Assets/Scripts/DI/Services/Analytical/Editor/PuzzleAnalyticsBuildGuard.cs`
- `Assets/Scripts/DI/Services/Developer-Panel/DeveloperMenuService.cs`
- `Assets/Scripts/Gameplay/Services/CheckTruePositionService.cs`
- `Assets/Scripts/Gameplay/Services/InventoryReturnService.cs`
- `Assets/Scripts/Gameplay/Services/SetBlockService.cs`
- `Assets/Scripts/Gameplay/Services/CheckHouseCompleteService.cs`
- `Assets/Resources/PuzzleAnalyticsContentRevisions.asset`
- `Docs/PuzzleAnalyticsContentCatalog.md`

### Critical repository safety note

The Puzzle working tree is heavily dirty and contains the user's unrelated
house, scene, Addressables, art, SDK and content work beside the analytics
changes. The receiving agent must not reset, clean, stash, switch branches,
commit the whole tree, or “tidy” unrelated files. Work read-only until the
exact analytics diff is understood. A request to implement analytics does not
automatically grant permission to commit the user's other Puzzle changes.

The Mortris server is currently on clean `main` at merge commit `2248d34`.

## 3. Non-negotiable analytical definitions

These definitions must be used consistently by every server query and every
dashboard label.

### Identifiers

- `install_id`: anonymous SDK installation, not a person or account.
- `session_id` + SDK `sequence`: transport/session ordering.
- `house_run_id`: one continuous run through a house. Use this for house
  completion, house abandonment and wave-funnel denominators.
- `attempt_id` / `wave_attempt_id`: one wave attempt. In the current client
  these identify the same logical scope. Use this for per-wave replay and
  attempt integrity.
- `interaction_id`: one physical detail move beginning at `detail_taken` and
  ending in placement, return or abandonment.
- `developer_action_id`: one developer-menu command across scene reloads.
- `content_revision`: the immutable semantic catalogue used by the build.
- `block_id`: the detail selected from inventory.
- `candidate_target_id`: the slot considered for this release. `-1` means no
  snap candidate, not “unknown block.”

Do not substitute one scope for another. Counting `attempt_id` as house starts
over-counts houses with many waves. Counting `house_run_id` as placement tries
under-counts detail interactions.

### Natural and developer traffic

The default designer analytics must describe natural play.

Event-level natural gameplay means:

- `origin = player`; and
- `progress_origin = natural`.

Developer-menu events are diagnostic records and never count as player
placements, hints, completions or failures.

Events with `progress_origin = developer_modified` or `developer_reset` are not
natural progression and must not affect difficulty or retention metrics.

There is one important distinction:

- For placement-quality metrics, a genuine manual placement made before the
  tester changes progress may remain valid. Filter at event time.
- For completion, abandonment, duration, house funnel and retention, exclude
  the entire house run if any developer command changes that run or if its
  final progress origin is non-natural. A cheated completion cannot supply the
  numerator while its natural prefix supplies the denominator.

The UI may offer `Natural only` (default), `Developer affected`, and `All`
views, but it must never silently mix them.

### Time

- Use `effective_at` for analytics windows, not `received_at`.
- Use `received_at` only for delivery delay/freshness diagnostics.
- Use `active_elapsed_ms` for active gameplay duration. It already excludes
  time spent backgrounded.
- Wall-clock gaps must be capped where the current diagnostics code already
  caps long background gaps. Do not turn a night in the background into an
  eight-hour puzzle attempt.
- `app_backgrounded` does not itself mean abandonment. Abandonment requires a
  documented inactivity threshold and no later recovery/completion.

### Evidence and sample size

- Preserve the existing minimum of five placement samples before painting a
  block or house as good/bad.
- Below five, show raw count and “not enough plays”; never show a confident
  green/red verdict.
- Never hide untouched houses. “Nobody played this house” is a coverage
  finding.
- Always show raw sample count beside a percentage.
- Do not interpret one installation reloading a scene as multiple players.
- Do not describe correlation by device/memory as causation.

### Content and coordinates

- Events must resolve against their own `content_revision`.
- The current clean build sends imported revision
  `09f3b91e4e45313f973b3264536f62277bb22d414453a6e6f5a25c6e3211f23e`.
- `coordinate_space` must be `house_local` for new spatial analysis.
- Negative house-local X or Y values are valid coordinates. Never use
  “coordinate < 0” as a missing-value check.
- Geometry and art attach to a catalogue revision but do not redefine its
  semantic hash.

## 4. Remaining stage overview

| Stage | Outcome | Dependency |
|---|---|---|
| 2 | Trust and data-quality control plane | Stage 1 production events |
| 3 | Exact natural/developer segmentation | Stage 2 definitions and checks |
| 4 | House coverage and wave funnel | Stage 3 clean run classification |
| 5 | Detail friction, pacing and replay completion | Stages 2–4 |
| 6 | Device and memory diagnostics | Enough long sessions to emit samples |
| 7 | Automated QA, monitoring and release procedure | Stable endpoint contracts |
| 8 | Measured performance/rollups, only if needed | Real production volume |
| 9 | Ongoing tester analysis and product decisions | Stages 2–7 operational |

Implement in this order. A later stage must not work around an unresolved
quality or segmentation error from an earlier stage.

## 5. Stage 2 — trust and data-quality control plane

### Goal

Make it immediately visible whether the selected date range is safe to use.
Today integrity warnings exist mainly inside one opened replay. The dashboard
needs an aggregate quality summary before anyone can trust house rankings.

### Required server work

Add a project/date/build-scoped quality response, either as a dedicated
gameplay quality endpoint or as a clearly separated section of the Puzzle
overview response. Keep it Puzzle-specific; do not weaken the generic ingestion
contract.

It must report at least:

1. total received and accepted event counts;
2. server rejection counts grouped by stable rejection code and event name;
3. duplicate count from ingestion stats;
4. schema-v2 share and count of events missing `schema_version`;
5. events whose `content_revision` does not join to
   `puzzle_content_revisions`;
6. placement events whose `coordinate_space` is absent or not `house_local`;
7. distinct installs, sessions, house runs, wave attempts and interactions;
8. SDK-sequence gaps/duplicates within a session;
9. `house_event_index` gaps/duplicates within a house run;
10. `attempt_event_index` gaps/duplicates within a wave attempt;
11. interaction chains missing take, release, resolution or terminal return;
12. state checkpoint hash mismatches;
13. attempts recovered after restart;
14. attempts/runs that never closed, separated into recent/in-progress and
    stale/lost;
15. developer commands without a matching terminal result;
16. developer mutations without a matching start/action ID;
17. event delivery delay distribution (`received_at - sent_at_client`), not
    just an average;
18. the app/build versions contributing to the range.

Do not compute integrity by ordering only on `effective_at`. Prefer the
explicit event indexes; timestamps are tie-breakers and human display values.

Do not count a fall interaction as orphaned merely because
`placement_resolved` says it fell. Its terminal inventory return may be a later
`detail_returned` event with the same interaction ID.

### Revision fallback decision

`loadReplayCatalog` currently falls back to the newest imported revision when
an event revision cannot be found. That workaround was necessary for legacy
data but can hide future content-pipeline failures.

Perform these steps in order:

1. Prove the clean production range has zero unmatched revisions.
2. Surface unmatched revisions as a visible quality failure.
3. Add tests for an unknown revision.
4. Only then remove or hard-disable the silent fallback for schema-v2 events.
5. If historical data must remain viewable elsewhere, make fallback an
   explicit legacy mode with a warning; never use it silently for current
   builds.

### Required dashboard work

Add a quality banner/card at the top of `/houses` and the house detail page:

- Green: no known integrity problems in the chosen scope.
- Amber: too little data, recent open runs, or expected short-test omissions.
- Red: rejected events, revision mismatch, index gaps, orphan interactions,
  checkpoint mismatch, unpaired cheat command, or invalid coordinate space.

Every status needs plain language and a drill-down. “3 sequence gaps” must link
to the affected attempt/run rather than leave the designer with an abstract
number.

Do not use green for “zero events.” Zero events is grey/unknown.

Add build filtering early in this stage. A date range can contain multiple
build contracts; the page must show which build is selected and warn when the
range mixes builds.

### Documentation work

Update the older Puzzle handoff documents so they no longer state that:

- `returned` is never emitted;
- release coordinates are still world-space;
- all events use unimported fallback revisions;
- schema-v2 client changes are unbuilt/unverified.

Keep historical explanations where useful, but label them as resolved and
point to this cutover.

### Tests

- Unit tests for every integrity bucket.
- Database-backed test using a compact full chain:
  start → take → release → fall → return → checkpoint → retry → place → close.
- Gap, duplicate, orphan and bad-hash fixtures must each trip exactly one
  expected quality flag.
- A developer action crossing a scene reload must still pair by
  `developer_action_id`.
- A batch delivered late but internally ordered must not be called a sequence
  failure.
- Empty range and one-event range must render without division by zero.

### Exit criteria

- The first production card for build `9dccf3d…` is green.
- 110-event smoke run is reported as 110 accepted, zero rejected/duplicate,
  zero sequence gaps, zero orphan interactions and two paired developer
  actions.
- A deliberately malformed local fixture produces visible red failures.
- No current replay silently resolves against the wrong revision.

## 6. Stage 3 — exact natural/developer segmentation

### Goal

Prevent tester shortcuts from changing difficulty, completion, abandonment or
retention conclusions while preserving them for debugging.

### Required server classification

Create one shared classification rule used by all Puzzle queries. Do not copy
slightly different `COALESCE(...)=natural` fragments into every new query.
Whether implemented as reusable SQL, a view, helper query builder or another
small mechanism is an implementation choice, but its semantics must be tested
once and reused.

Classify at three levels:

1. Event:
   - natural player event;
   - developer-menu event;
   - player event on developer-modified/reset progress;
   - malformed/unknown origin.
2. Wave attempt:
   - fully natural;
   - developer affected;
   - recovered after restart;
   - incomplete/recent;
   - stale/lost.
3. House run:
   - fully natural;
   - developer affected at any point;
   - naturally completed;
   - naturally abandoned;
   - developer-completed/reset.

Use `house_run_id` for house outcomes and `developer_action_id` for cheat
pairing. Do not infer a cheat solely from a fast completion time.

### Metric-specific inclusion rules

- Detail fall rate, no-snap rate, missing-support rate and release map:
  include only event-level natural interactions.
- Retry/time-to-place samples: include only interactions whose whole chain is
  natural and belongs to one attempt.
- Wave entry/completion, house completion and abandonment: include only fully
  natural house runs.
- Attempt replay: allow all classifications, but label them visibly.
- Player/device timeline: allow all raw events for authorized debugging; add
  classification labels instead of deleting events.

### Dashboard behavior

Add a scope control with:

- `Natural only` — default and used for every designer verdict.
- `Developer affected` — tester/debug investigation.
- `All traffic` — diagnostic only, visibly marked as mixed.

When `Developer affected` or `All traffic` is selected, remove or soften any
language claiming “players struggle” because the selected population is no
longer ordinary play.

Show a compact tester-impact summary:

- number of developer actions;
- affected runs;
- commands by name/result;
- resets versus progress completions;
- failures/noops;
- before/after progress mutation availability.

### Edge cases that must be tested

- A tester makes natural manual placements, then uses `CompleteHouse99`.
  Placement-quality events before the cheat may remain natural; the house run
  must not count as a natural completion.
- `ResetHouse` begins on one visible house and reloads into another context.
  Pair by action ID and preserve both before/after context; do not require the
  start and completion house IDs to be identical.
- Menu opened but scene/app closes before a command: menu lifecycle only, no
  fake failed command.
- Command starts but never produces completed/failed/noop: quality failure.
- A no-progress command must be `developer_command_noop`, not completion with
  invented mutation.
- Recovered attempt after app restart remains the same attempt only when the
  persisted identifiers/checkpoint say so.

### Exit criteria

- Natural house metrics do not change when a fixture adds a developer
  completion to the same run.
- Tester view shows the two production smoke commands as two paired actions.
- No dashboard metric silently combines natural and modified progress.
- Every Puzzle aggregate endpoint accepts or consistently inherits the same
  traffic-scope parameter.

## 7. Stage 4 — house coverage and wave funnel

### Goal

Answer two questions before drilling into individual details:

1. Which houses have enough play to evaluate?
2. Where do natural players stop progressing through a house?

### House coverage view

Extend the house wall without turning it back into a table. Each house card
should expose:

- unique anonymous installations;
- natural house runs started;
- natural house runs completed;
- natural completion rate;
- waves reached versus total waves;
- played details versus total details;
- details with enough samples;
- last natural play time/build;
- dominant failure reason;
- evidence state: unplayed, thin, usable or mixed-quality.

Keep “worst first” only among houses with sufficient evidence. Untouched and
thin houses should be grouped as coverage work, not ranked as easy/hard.

### Wave staircase/funnel

Build the wave staircase described in the original visual plan. The unit is a
natural `house_run_id`:

- entered wave N;
- completed wave N;
- reached wave N+1;
- abandoned at/after wave N;
- still recent/in progress.

Do not use placement count as wave entry. A user may enter a wave and make no
placement, which is exactly a potential drop-off.

Show both counts and transition percentages. A shrinking ribbon/step view is
preferred to a table. Clicking a step should filter the attempt/run examples
below it.

Define the stale/lost threshold in one server constant/configuration and state
it in the UI. The existing diagnostics use a lost threshold; reuse the same
definition rather than inventing another.

### Build and cohort filters

At minimum support:

- date range;
- build number;
- city/house;
- natural/developer scope;
- device class/OS only when sample counts are visible.

Do not add arbitrary account demographics; the project is anonymous and does
not collect them.

### Tests

- One house run traversing four waves counts as one house start, not four.
- Restart/recovery does not double-count wave entry.
- A developer completion is absent from the natural completion numerator.
- An entered wave with zero placement remains in the funnel.
- Recent open run is not immediately called abandoned.
- A range boundary does not split a house run into a false completion or false
  loss without an explicit documented rule.

### Exit criteria

- A designer can identify coverage gaps and worst trustworthy house from the
  first screen.
- Clicking a wave shows representative natural runs from that wave.
- House completion and wave transition numbers are invariant when developer
  events are added to a fixture.

## 8. Stage 5 — detail friction, pacing and replay completion

Several ingredients already exist. Do not rebuild them:

- house art and geometry;
- metric-painted block diagram;
- wave tabs;
- support graph;
- drop map;
- per-block median retries and median time-to-place;
- attempt picker and scrubber;
- state checkpoints and integrity counters.

The remaining work is to make the supporting evidence visible and remove
unsafe replay assumptions.

### 5A. Retry ladder

For every detail show the distribution of natural interaction attempts before
success:

- success on first try;
- success on second/third/fourth+ try;
- never succeeded in the observed attempt;
- median and a high percentile when sample size supports it.

Group retries by attempt + block/inventory group and reset the chain when the
player changes to a different block. Do not join retries merely because two
events share a target ID.

Clicking a ladder row should select the block on the house and offer example
replays.

### 5B. Time-to-place distribution

Measure from `detail_taken` to the terminal placement result using
`active_elapsed_ms`, not raw wall-clock time. Report:

- sample count;
- median;
- p75/p90 only when the sample is sufficient;
- incomplete/abandoned interactions separately.

Do not treat a missing terminal event as a very long placement; it is an
integrity/abandonment category.

### 5C. Attempt pacing band

For one attempt/run, render active progression over time with:

- take/release/resolve chains;
- falls and retries;
- hint pins;
- app background/foreground spans;
- checkpoints/recovery;
- wave boundaries;
- developer actions when viewing tester traffic.

Keep active time and wall time visually distinct. A background span is not
gameplay work.

### 5D. Drop-map improvements

For schema-v2 events, require `coordinate_space=house_local` and draw directly.
Legacy alignment code may remain only for explicitly historical data.

Show:

- release point;
- candidate target;
- nearest compatible target and distance;
- outcome/rule state;
- density/cluster only after enough points;
- example attempts linked from suspicious clusters.

Do not fabricate a heatmap from one or two dots.

### 5E. Replay correctness fixes

Audit the replay arrow before considering replay complete:

- Current rendering suppresses coordinates when X or Y is negative. Negative
  house-local positions are valid and already occurred in the production smoke
  test. Missing coordinates need an explicit nullable/presence representation.
- Current replay arrow locates a target by searching for a block whose
  `block_id` equals `target_id`. Verify this contract against every imported
  house. If IDs can differ, return target coordinates or target geometry from
  the server; do not depend on accidental equality.
- A replay step should distinguish “no coordinates on this event” from a real
  coordinate of `-1`.
- Developer command events should receive plain-language replay sentences,
  not raw underscored event names.
- `interaction_abandoned`, command failed/noop, recovery and lifecycle steps
  need explicit wording and visual treatment.

### 5F. Detail diagnosis sentence

The selected block panel should conclude in plain language, for example:

- “Players release close to the right slot but the required support is
  missing.”
- “Drops are widely scattered and usually find no snap target.”
- “The detail is placed successfully but takes several retries.”
- “There is not enough evidence yet.”

The sentence must be produced from measured buckets and sample thresholds,
not from an unconstrained language model.

### Tests and visual verification

- Server fixtures for retry grouping, time calculation and incomplete chains.
- Negative coordinate replay fixture.
- Target ID different from block ID fixture.
- Background interval does not inflate active time.
- Screenshot/visual test or manual browser verification for every outcome.
- Keyboard and screen-reader labels remain meaningful; colour cannot be the
  only signal.
- Test art-only, diagram-only and combined modes after overlays change.

### Exit criteria

- Every existing schema-v2 interaction can be replayed without silently
  dropping valid coordinates.
- A designer can move from a suspicious block to its distribution and then to
  an example attempt without copying IDs.
- Retry/time/pacing visuals remain neutral below their evidence threshold.

## 9. Stage 6 — device and memory diagnostics

### Goal

Determine whether technical device pressure correlates with gameplay problems
without confusing device records with users or inventing performance data the
client does not collect.

### Existing data

Every event already has platform, OS version, device class, app version, build,
locale and timezone offset. `device_profile` carries device/graphics memory and
screen dimensions. `memory_sample` is expected only after each cumulative ten
minutes of active foreground play.

The short production smoke test correctly had no `memory_sample`. Absence in a
short session is not a defect.

Scene reloads may emit `device_profile` more than once for one installation.
Server views must deduplicate by `install_id` and use the latest applicable
profile. Do not count profile events as devices.

### Views to build

1. Anonymous device summary:
   - installs and natural runs;
   - build/OS/device class;
   - latest profile;
   - memory-sample availability;
   - falls/completions with visible sample counts.
2. Memory timeline for one installation:
   - allocated, reserved and Mono-used memory over active play time;
   - scene/house/wave markers;
   - background/recovery markers;
   - build changes.
3. Cohort comparison only when evidence exists:
   - low/medium/high device-memory bands;
   - fall/completion rate with confidence/evidence counts;
   - no causal wording.

### Non-goals

- Do not show FPS, frame time, thermal state or crash rate unless separately
  instrumented later. They do not exist in the current contract.
- Do not collect device advertising identifiers or account identity.
- Do not add per-frame memory telemetry.

### Exit criteria

- Multiple profiles from one install render as one device.
- Short sessions say “memory sample not expected yet,” not “missing data.”
- Device comparisons always show installs/runs and never claim causation.

## 10. Stage 7 — automated QA, monitoring and release procedure

### Goal

Turn the successful manual smoke test into a repeatable gate for every future
analytics build.

### Server test fixture

Create a reusable schema-v2 fixture representing at least:

- registration/device profile;
- house and wave start;
- successful placement;
- no-target fall and return;
- missing-support fall and return;
- checkpoint;
- background/foreground;
- recovery after restart;
- natural completion;
- developer completion;
- developer reset;
- failed and noop developer commands;
- upload retry producing duplicates;
- late delivery with valid sequence;
- malformed property rejected by strict catalogue.

Use it for database-backed endpoint tests. Do not only unit-test helper
functions with hand-built partial maps.

### Unity verification

Keep and expand focused tests around:

- build guard and consent/revision assets;
- house-local coordinate conversion, including translated/scaled/rotated
  roots and negative coordinates;
- interaction lifecycle pairing;
- state hash/checkpoint determinism;
- attempt recovery;
- developer action pairing across scene reload;
- terminal flush/background behavior.

A C# compile is not runtime proof. For analytics release acceptance, run a real
Android/emulator or device smoke test.

### Android smoke script/checklist

For every candidate build:

1. Record app version and build number.
2. Clear app data or uninstall the previous build so an old durable queue does
   not pollute the test.
3. Register one new installation.
4. Complete at least one normal house/wave.
5. Produce one `fell_no_snap_target`.
6. Produce one `fell_missing_support`.
7. Return and retry a fallen detail.
8. Background and foreground the app.
9. Kill/restart during an open attempt and verify recovery.
10. Run one progress-completing cheat and one reset cheat.
11. Background the app to force upload.
12. On production/staging, verify accepted/rejected/duplicate counts, sequence
    continuity, interaction pairing, revision, coordinates and developer
    pairing.
13. Confirm natural metrics ignore cheated completion.

Do not hand a build to the full tester group until this smoke gate is green.

### Production monitoring

Monitor and alert on:

- readiness and service restart;
- any Puzzle strict-catalog rejection;
- schema-v2 share dropping below expected for the active build;
- unknown content revision;
- coordinate-space mismatch;
- sequence/orphan/checkpoint integrity failure;
- unpaired developer action;
- ingestion delay growth;
- disk/retention pressure using existing Mortris runbooks.

Alerts need project/build context and a link to the affected data. Avoid an
un-actionable generic “analytics error.”

### Deployment discipline

Normal sequence:

1. run focused tests;
2. run `make lint`, `go test ./...`, dashboard build and changed-file
   readability gate from the Mortris repository root;
3. obtain all required green CI checks;
4. merge;
5. build dashboard and Linux binary from the exact merged SHA;
6. verify binary hash;
7. copy a rollback binary on production;
8. run the migration subcommand, not raw SQL;
9. restart service;
10. verify internal and external readiness, migration and binary hash;
11. run read-only production smoke queries.

The self-hosted readability runner experienced a GitHub Actions service outage
during the schema-v2 emergency deployment. Do not treat that incident as a
general permission to bypass CI. If the user explicitly orders an emergency
override, document the pending check and still run the equivalent gate locally.

Every migration creating a table must include the least-privilege grants needed
by the runtime roles.

### Exit criteria

- One command/checklist can validate a known schema-v2 fixture.
- A candidate Android build can be approved without manual ad-hoc SQL
  reasoning.
- Production alerts distinguish ingestion, content, coordinate, sequence and
  developer-action failures.

## 11. Stage 8 — measured performance and rollups only when needed

### Goal

Keep the current simple raw-event model until evidence proves it is too slow,
then optimize without changing metric definitions.

### Do not optimize yet merely because the event schema is verbose

Traffic volume was explicitly accepted for this testing phase. Correctness and
observability are more important than client batching or compression changes.

Before adding rollup tables or generated columns, capture:

- event count and growth per day;
- database size and index size;
- p50/p95/p99 latency for each Puzzle endpoint over common ranges;
- query plans for the slow endpoints;
- dashboard payload sizes;
- production CPU/memory during concurrent dashboard use;
- retention and backup duration.

### Optimization order if thresholds are exceeded

1. Narrow queries and reuse exact filters/classification.
2. Add measured indexes or generated columns for repeatedly extracted keys
   such as build, house run, attempt, city/house/wave and progress origin.
3. Cache immutable layout/art/catalogue responses.
4. Add daily/hourly rollups only for stable aggregate definitions.
5. Keep raw events as the replay/audit source of truth.
6. Make rollups rebuildable and test them against raw-query fixtures.

Do not roll up away `interaction_id`, checkpoints or developer actions needed
for replay/integrity.

### Exit criteria

- Optimization is tied to a recorded production measurement.
- Raw and rollup answers match on the same fixture/date/build/scope.
- No dashboard behavior or metric definition changes silently during the
  optimization.

## 12. Stage 9 — ongoing tester analysis and product decisions

This is an operating cadence, not a one-time coding phase.

### Before drawing conclusions

For each reporting window verify:

- quality card is green or its limitation is stated;
- one build/scope is selected;
- natural traffic is the default;
- sample counts are shown;
- house coverage is sufficient;
- tester cheat traffic is reported separately;
- recent/in-progress runs are not called lost.

### Weekly designer output

Produce a short, visual report answering:

1. Which houses have enough evidence?
2. Which trustworthy house is worst and why?
3. Which wave loses the most natural runs?
4. Which details show no-target versus missing-support problems?
5. Are release points clustered, scattered or aimed at another slot?
6. Which example replay best demonstrates each problem?
7. Is the problem stable across builds/devices?
8. What cannot be concluded because evidence is thin?

Every recommendation should link back to a house/detail/replay and name the
sample size.

### Product decisions the dashboard should support

- Increase snap radius or improve visual affordance for repeated no-target
  drops.
- Simplify/reorder support rules for repeated missing-support falls.
- Clarify ambiguous twin details/targets.
- Re-author a detail whose art consistently suggests the wrong slot.
- Investigate long retry/time distributions.
- Separate technical/device pressure from house-design friction.

The analytics system should reveal these patterns; it should not automatically
change game physics or authored house content.

## 13. Work-package discipline for the receiving agent

Use one bounded work package at a time. Recommended sequence inside each stage:

1. Read the relevant existing code and tests.
2. Write down the exact metric/event contract in the PR description or stage
   notes before implementation.
3. Implement server derivation and tests first.
4. Add typed dashboard API contract.
5. Build the smallest visual that answers the stated question.
6. Test empty, thin, natural, developer and malformed data.
7. Run local quality gates.
8. Verify the visual in a real browser; build success alone is insufficient.
9. Deploy only through the documented production sequence.
10. Verify with read-only production evidence and record the exact build/SHA.

Do not combine unrelated Puzzle client work, server rollups, dashboard redesign
and deployment into one large change. Later agents need to be able to isolate a
metric error to one stage.

## 14. Global definition of done

The remaining roadmap is complete only when all of the following are true:

- A designer enters through the house wall, not an ID table.
- The page states whether the selected data is trustworthy.
- Natural and developer-affected play cannot be silently mixed.
- House starts/completions use house runs; wave analysis uses wave attempts;
  physical moves use interactions.
- Coverage and sample size are visible everywhere a rate is shown.
- House, wave, detail and replay are connected by clicks rather than copied
  identifiers.
- Drop maps use only trusted house-local coordinates for current builds.
- Replay handles negative coordinates and non-equal target/block IDs safely.
- Retry, time and pacing visuals are derived from complete natural chains.
- Device/memory views deduplicate installs and avoid causal claims.
- Integrity failures and strict-catalog rejections are monitored.
- Android smoke verification is repeatable for every analytics build.
- Any later optimization is backed by production measurements and preserves
  raw-event truth.

Until these conditions hold, the system may collect excellent telemetry but it
is not yet a complete decision tool for house and physics design.
