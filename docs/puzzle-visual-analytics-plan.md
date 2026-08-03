# Puzzle visual analytics: plan

Turn `/gameplay` from ID tables into a picture of the house. The goal is
that somebody who has never read an analytics dashboard opens the page and
can say, without help, which house is failing, which detail inside it is
failing, and what the player actually did wrong.

Scope is the existing `puzzle_gravity_test` project. Nothing here changes
the ingestion contract, the strict catalogue rule, or the anonymity
guarantees in `docs/puzzle-gravity-playtest-handoff.md`.

## Where we are

The data side is done. `puzzle_content_revisions/blocks/targets/rules` hold
an immutable per-revision catalogue, `internal/analytics/puzzle_*.go`
derives summary, scope, friction, daily rows, attempt-rule reconstruction
and an anonymous device list, and every event carries `attempt_id`,
`content_revision`, `city_id`, `house_id`, `wave_index`,
`active_elapsed_ms`, `placed_block_count` and a monotonic
`attempt_event_index`.

The presentation side is not. `dashboard/src/pages/GameplayDiagnosticsPage.tsx`
is four collapsed `<details>` blocks of numeric IDs. A designer reading
`block_id 36 / target_id 36 / fall_rate 62%` learns nothing about which
piece of the house that is.

## The blocker: no block geometry

`puzzle_content_blocks` stores one centre point per block
(`local_x_milli`, `local_y_milli`) plus `visual_key`, `is_ground` and
`order_in_layer`. A house of 56 centre points cannot be drawn. Every visual
below depends on fixing this first.

It is cheap to fix because the shapes are already computed elsewhere.
`Assets/Scripts/Tools/EditorToolDesigner/Editor/EditorToolAiExport.cs`
already calls `PixelTracingHelper.TraceSprite` and emits `TracedWorldPaths`,
`WorldBounds`, pivot, pixels-per-unit and a ground plane per block. The
analytics exporter reuses that builder.

### Geometry attaches to a revision, it does not live inside it

The obvious design — add shapes to the catalogue document — is wrong, and
the reason matters.

`content_revision` is the SHA-256 of that document. Adding fields mints a
new revision, and events always resolve against their own. Every event
already collected would keep pointing at the old shapeless revision and
could never be drawn. The entire existing playtest would be invisible to
the feature built to look at it.

Shapes are not semantics. Where a block sits, which wave it belongs to,
and what must be placed before it are meaning, and stay immutable. How
that same block was drawn is a description of it, and can be attached
afterwards without changing anything the revision asserts.

So geometry is a separate additive upload against an existing revision:

```json
{
  "schema_version": 1,
  "content_revision": "09f3b91e…",
  "blocks": [{
    "city_id": 0, "house_id": 1, "block_id": 36,
    "bounds_milli": { "min_x": -2100, "min_y": 14300, "max_x": -900, "max_y": 15600 },
    "outline_milli": [[-2100,14300],[-900,14300],[-900,15600],[-2100,15600]]
  }]
}
```

- `outline_milli` is the silhouette in the same quantized milli-unit
  space as `local_x_milli`/`local_y_milli`, capped at 64 points. Optional
  — bounds alone draw a rectangle.
- `bounds_milli` is required, and is what the renderer aggregates into a
  house viewBox.
- It may only describe blocks the revision already declares. A block it
  does not recognise fails the whole upload rather than landing partial
  shapes that would draw a broken house.
- Re-uploading is safe, and re-importing the catalogue does not erase
  geometry.

Coordinates must be the **assembled-house** space, the same one
`local_x_milli` already uses — not the exploded scheme space.
`ScalingFactor` and `Spacing` are presentation-time values and must not
leak into the upload.

A house-level `ground_top_milli` was considered and dropped: the ground
line is the lowest bound among blocks already flagged `is_ground`, so
storing it would duplicate derivable state.

Still forbidden, unchanged: no Unity objects, no sprite pixel data, no
rule arrays in events, no drag frames, no per-frame coordinates.

### Event additions (Puzzle side): none needed

`hint_used` carries no `block_id`, which at first looks like a gap. It is
not one: `HintService.UseHint()` reveals the whole-house blueprint, so
there is no single detail to attribute it to. Adding a block would be
inventing precision the game does not have.

Hint pressure per detail is instead derived server-side, from the
`placement_resolved` failures immediately preceding a `hint_used` in the
same `attempt_id`. That is what "the player was stuck on this piece and
gave up and asked" actually means, and it needs no client change.

No other event field is missing for anything in this plan.

### Coordinate system (resolved)

Verified against all 125 house folders in the Puzzle repo:

```
block rect = position ± (spriteWidthPx, spriteHeightPx) / (2 × PPU)
```

with `PPU` read from the sprite's `spritePixelsToUnits` (120 everywhere
checked) and the pivot centred at `0.5, 0.5`. Reconstructing a house this
way reproduces its packed canvas
(`assets.json`: `canvas_w / assets_dpi / ScalingFactor`) exactly for 104
of 125 houses; the remaining 21 overshoot by about 1%, which is canvas
cropping around transparent margins, not a different scale.

`ScalingFactor` relates block space to packed-art pixel space, and
`Spacing` is exploded-scheme presentation only. Neither belongs in the
export.

### Art tier: both layers

The house view renders two layers over one coordinate space:

1. **Art layer** — the house's real `composite.png` as the background, so
   the page looks like the game. Note it is *not* the catalogue's
   `preview_asset_key`: that points at `static_packed.png`, which turned
   out to be the window/lights layer, not the assembled house.
2. **Diagram layer** — block outlines on top, tinted by metric,
   clickable, labelled. This is what makes "which detail sits in which
   position" answerable at a glance.

A three-way toggle switches between art + diagram, art only, and diagram
only, so the schematic stays fully available to anyone who wants it.

Over the art, only blocks with a trustworthy rate are filled. A flat tint
on every block washes the picture out and buries the one red block among
fifty grey ones — the opposite of what an overlay is for. Blocks without
enough plays keep just their outline and let the art through.

Placement needs no stored data. The art is cropped to its opaque pixels,
and that crop's extent is exactly the union of the revision's block
bounds — verified by overlaying outlines on the art, which land on the
pediment, cornices, window frames and door. So the renderer places it
with the rect it already computes for its viewBox, and there is no second
copy of that rect to drift.

Images live in Postgres, not on disk. Measured, the 103 houses come to
4.2 MB as WebP cropped to 900px on the long edge — small enough that a
filesystem store would buy nothing and cost a config path, a backup
concern and orphan cleanup. In the database it is transactional with its
revision and already covered by pgbackrest. The earlier ~100 MB estimate
assumed uncompressed PNGs.

## Design principle

One drill path, never a table as the entry point:

```
city  ->  house  ->  wave  ->  detail  ->  one attempt
```

Every level is a picture, every picture is clickable, every screen states
its conclusion as a sentence before it shows a number. The existing
`verdict` paragraph pattern in `GameplayDiagnosticsPage.tsx` is the model —
extend it everywhere.

Colour never carries meaning alone: every tint is paired with a number or
a label, per the accessibility rule already in `index.css`.

## The screens

### Level 0 — house wall

Grid of house cards, one per `(city_id, house_id)`, tinted green to red by
fall rate, sorted worst first, attempt count as a badge, `display_label` as
the name. Answers "which house is failing" without reading. Replaces the
"City, house, and wave outcomes" table.

### Level 1 — house X-ray

The house drawn from block outlines, painted by a metric the user picks:
fall rate, first-try failure rate, hint rate, tries-to-place, or
time-to-place. Wave tabs ghost the blocks belonging to other waves, so the
intended build order is visible.

Clicking a block opens a panel with its picture, its numbers, and its rule
in plain words, generated from `puzzle_content_rules`:

> Detail 36 can be placed after 5, or after both 33 and 34.

An optional overlay draws the support graph — arrows from required blocks
to their target, one colour per OR alternative. This deliberately mirrors
the editor's `Only Scheme` inspection view that designers already read.

Replaces the "Target and detail friction" table.

### Level 2 — drop-miss map

For one block, scatter every `release_x_milli`/`release_y_milli` over the
house silhouette, with the correct target outlined and dots coloured by
`outcome`. Needs no new event fields.

It separates three failures that look identical in a table:

- a tight cluster offset from the slot — the art reads as sitting
  somewhere it does not;
- a wide scatter — snap radius too small;
- a cluster on a different slot — ambiguous `compatible_block_ids`, the
  player cannot tell the twins apart.

### Level 3 — attempt replay

`placed_block_ids` plus `attempt_event_index` make an attempt fully
reconstructable, and `internal/analytics/puzzle_attempt.go` already
rebuilds the placed set and `missing_support_groups`. Render it as a
scrubber: each step draws the house's placed state, the piece in play, an
arrow from the release point to the intended target, and a verdict badge.
On `fell_missing_support`, flash the blocks that were missing.

This is the "click an event, see the house, see that exact detail"
requirement. Replaces the attempt timeline table.

### Level 4 — supporting charts

The four `outcome` values each imply a different fix, so label them by
what they mean rather than by their enum value:

| `outcome` | Shown as | What it points at |
|---|---|---|
| `fell_no_snap_target` | dropped nowhere near a slot | snap radius, affordance |
| `fell_missing_support` | tried to build in the air | rule strictness or unclear build order |
| `fell_missing_rule` | content bug | broken catalogue — should be ~0, worth an alert |
| `returned` | changed their mind | hesitation, not failure |

Alongside:

- **Wave staircase** — attempts entering each wave as a shrinking ribbon.
  Shows where a house loses people, replacing the daily table's role.
- **Retry ladder** — distribution of placement attempts per block, sorted
  by median. "Detail 36 takes 3.4 tries."
- **Time-to-place** — per-block distribution from `detail_taken` to
  `placement_resolved`, derived server-side from timestamps. The long tail
  is the piece players stare at.
- **Attempt pacing band** — active versus backgrounded intervals with
  hints as pins, per attempt. Makes the rage-quit shape visible.
- **Device and memory** — RAM against falls, and `memory_sample` over
  active play time. Already collected, currently only shown as columns.

## Server work

- Schema (migration `0010`, additive, all columns nullable): four integer
  `bounds_*_milli` columns plus a jsonb `outline_milli` on
  `puzzle_content_blocks`. Bounds are plain columns rather than jsonb
  because the renderer's viewBox is a MIN/MAX aggregate over a house.
- Upload: `POST /api/v1/projects/{id}/puzzle-content/{revision}/geometry`,
  project-admin plus CSRF, the same guard as the catalogue import. The
  path revision is authoritative; a body claiming a different one is
  refused as a mismatched export.
- Read endpoints, all under the existing project-scoped auth:
  - `GET /api/v1/analytics/gameplay/houses` — house wall leaderboard.
  - `GET /api/v1/analytics/gameplay/houses/{city}/{house}/layout` —
    geometry, waves, targets and rules for one revision, for rendering.
  - `GET /api/v1/analytics/gameplay/houses/{city}/{house}/blocks` —
    per-block metrics for the chosen paint metric.
  - `GET /api/v1/analytics/gameplay/blocks/{blockID}/drops` — release
    points for the drop-miss map.
  - Attempt replay reuses the existing
    `GET /api/v1/analytics/gameplay/attempts/{attemptID}`.

- Asset serving: `GET /gameplay/houses/{city}/{house}/art?revision=`,
  with the image stored in Postgres and loaded by the
  `import-puzzle-art` subcommand. Content-Type comes from an allowlist
  checked at import, never from the stored row alone, and responses carry
  `nosniff` plus an immutable cache header since a revision cannot change.
- **Every migration that creates a table must also grant on it.** The
  runtime pools use least-privilege roles, so `CREATE TABLE` alone leaves
  the writer unable to touch it — `0011` shipped without grants and the
  art import failed in production with "permission denied" until `0012`
  added them. `0006` is the same fix for the `0004` tables.

Current queries scan `events` with jsonb extraction. That is fine at
playtest volume. Revisit with rollup tables only against measured
pressure, per the plan's section 16.

## Hosting: houseart.mortris.forkhorizon.com

This view gets its own hostname, dedicated to `puzzle_gravity_test`.

It stays **one binary, one port, one SPA bundle**. There is no second
service to deploy, monitor or back up. The hostname is a routing and
framing concern only:

- nginx server block: `deploy/nginx/houseart.conf`, written and committed,
  proxying to `127.0.0.1:8090` like `mortris.conf`.
  `deploy/staging/nginx-sdk-test.conf` is the existing precedent for a
  second Mortris hostname on the same box.
- **Live.** DNS A record added in Cloudflare (DNS-only, matching
  `mortris` and `sdk-test`, which have no wildcard), and the shared
  certificate expanded to eight hostnames in one `certbot --expand` call.
  All eight verified serving valid TLS afterwards — passing only the new
  name would have dropped the other five.
- The host split is client-side (`dashboard/src/houseArtHost.ts`): on a
  `houseart.` hostname the app lands on the house wall, pins the project,
  and trims the navigation. It is presentation only — sessions, CSRF and
  per-project checks are server-side and identical on both hostnames, so
  editing the value in a browser grants nothing. The project is pinned
  only if the account actually has access, otherwise it would pin to
  something the server refuses with no way to change it.
- TLS: `certbot --nginx --expand` must be run with **every** ForkHorizon
  hostname in one invocation, per the shared-cert gotcha already
  documented in `deploy/README.md` step 13. Adding this domain alone
  would drop the others from the cert.
- The Go server checks the `Host` header: on `houseart.`, the SPA boots
  straight into the house view with the project pinned to
  `puzzle_gravity_test` and the project selector hidden. Everything else
  — auth, sessions, CSRF, project access checks — is unchanged and
  unconditional. The hostname is presentation, never authorization: a
  user without access to `puzzle_gravity_test` gets the same refusal here
  as anywhere else.
- DNS: an A record for `houseart.mortris.forkhorizon.com` at the VPS IP,
  and the hostname added to ForkHorizon's uptime checks alongside the
  others.

## Phasing

1. **P0 — geometry. Done on `feat/puzzle-house-geometry`.** Migration
   `0010`, the `PuzzleGeometry` type with validation,
   `ApplyPuzzleGeometry`, the upload route, and
   `tools/puzzle_geometry_export.py` which builds the payload from the
   Unity assets. Verified against a real Postgres: all 103 houses and
   7065 blocks import and attach, re-apply is idempotent, and an unknown
   revision is refused. Not yet applied to production.
2. **P1 — house wall and house X-ray. Done and deployed.** Metric
   painting, wave tabs, block panel with plain-language rules, and the
   art layer with its art / diagram / both toggle. Migrations `0011`
   (art storage) and `0012` (its role grants). Replaces every table
   previously on the page.
3. **P2 — drop-miss map. Done.** `GET /gameplay/houses/{city}/{house}/drops`,
   optionally scoped to one detail, plotted over the house as dots with a
   line to the slot the player was aiming at. Fall reasons already read in
   plain language on the block panel.
4. **P3 — attempt replay. Done.** An attempt picker per house, ordered
   worst-first because the reason to open one is almost always "show me
   one that went badly", and a scrubber that steps through it: grey is
   already standing, amber is the detail in the player's hand, red is
   what it was waiting on. Each step carries the full placed set, so any
   step renders without folding the ones before it.

   Fixed while building it: `loadAttemptCatalog` looked up the attempt's
   own `content_revision`, which is never imported, so every placement
   silently reported no missing support — the feature looked like it
   worked and always said "nothing was missing". `loadReplayCatalog`
   falls back to the newest imported revision. On a real attempt that is
   the difference between 0 and 5 of 7 falls naming what they were
   waiting on.

   The release arrow is deliberately not drawn here: the drop map already
   shows release points, and it needs the world-to-house correction that
   two of five houses cannot currently establish. Adding it would put an
   arrow in the wrong place on exactly the houses that most need looking
   at.
5. **P4 — support-graph overlay, retry ladder, time-to-place, pacing band,
   device and memory.**

## Data: measured, not assumed

Read via `MORTRIS_READER_DSN` on the production host (`/etc/mortris/env`,
root-only; queries run server-side over SSH so the credential never
leaves the box). Snapshot taken 2026-08-03, covering 2026-07-23 to
2026-08-02.

| | |
|---|---|
| events | 936 |
| installations | 3 |
| attempts | 45 |
| distinct blocks ever placed | 153, of 7065 in the catalogue |
| content revisions | 1 (`09f3b91e…`, 2026-07-22) |

Placement outcomes:

| outcome | count | share |
|---|---|---|
| `placed` | 298 | 81.6% |
| `fell_no_snap_target` | 41 | 11.2% |
| `fell_missing_support` | 26 | 7.1% |
| `fell_missing_rule` | 0 | — |
| `returned` | 0 | — |

Two things fall out of this.

**`fell_missing_rule` is zero.** The catalogue is clean — no house is
asking for a rule that does not exist. This is the number to alert on if
it ever leaves zero, and it belongs on the page as a standing green
check rather than a row in a table.

**`returned` is never emitted.** No call site in
`SetBlockService`/`CheckTruePositionService` produces it, so the value is
declared in the catalogue and documented but is dead. Either wire it —
a detail picked up and deliberately put back is a real hesitation signal
and distinct from a fall — or drop it from the docs. Until then the page
must not show a "changed their mind" category that is structurally
always zero.

### The sample is thin, and the design has to say so

Per `(house, block)` placement counts:

| placements | blocks |
|---|---|
| 1 | 41 |
| 2 | 56 |
| 3–4 | 45 |
| 5–9 | 11 |
| 10+ | 0 |

Three testers over ten days have touched 2% of the catalogue, and
two-thirds of the blocks they did touch have two placements or fewer. A
block painted bright red off two events is not a finding, it is noise
wearing a confident colour — precisely the failure this whole page exists
to avoid.

So the paint layer is sample-size aware, not optional:

- fewer than 5 placements — neutral grey, labelled "not enough plays
  yet", never tinted;
- 5 or more — tinted, with the raw count always shown beside the
  percentage;
- house-level and wave-level rollups, which do have usable volume today,
  carry the colour until per-block counts grow.

Observed fall rates, for the ramp: 18.4% overall, 9–22% across the six
houses with real volume, and two houses at 67% on three and nine
placements respectively — i.e. exactly the noise the grey rule
suppresses. Provisional ramp: green under 10%, amber 10–25%, orange
25–45%, red above 45%, re-fit once volume grows.

This reorders the work. Aggregate views — the house wall, the
outcome breakdown, the wave staircase — are meaningful **today**. The
per-block X-ray paint is built now but will read mostly grey until more
testers play, which is the honest state and should look like it.

## Two data problems found while building P2

Both are client-side and neither is fixed here. They limit what the drop
map can say, and the second one quietly weakens the revision guarantee.

### Release positions are in world space

`CheckTruePositionService` sends `blockTransform.position`, a world
coordinate, while blocks and targets are house-local. Each house sits at
its own world offset, so the two frames differ by a constant per house.
House 0/1 happens to sit near the origin, which is why this is invisible
if you only check that one — houses 0/2 and 1/2 are offset by about 11
and 12 units.

`alignDrops` recovers the offset from the data: a placed drop is within
the 1.5-unit snap radius of its target, so the median of
`release - target` over placed drops is the offset. When the estimate is
not tight enough to trust — spread wider than the snap radius, or fewer
than five placed drops — the map refuses to draw rather than putting dots
in the wrong place. Two of the five played houses currently refuse.

**The real fix is one line in the client**: send the release position
relative to the house root. After that `alignDrops` becomes a no-op for
new data and every house plots.

### No event references an imported content revision

All 936 events carry per-house revisions produced by the client's local
JSON-hash fallback (`Sha256(content.DataJson.text)`), 18 distinct values,
each covering exactly one house. None was ever imported. That is exactly
what `PuzzleAnalyticsContentCatalog.md` says must not happen in a test
build — the exporter-generated
`Resources/PuzzleAnalyticsContentRevisions.asset` either was not in the
build or did not resolve at runtime.

The consequence is that no query can join events to the catalogue by
revision; the house view and drop map match on `(city, house, block)`
against the latest imported revision instead. Block and target IDs are
array indices and have been stable across these builds, so this is
correct in practice today. It stops being correct the moment a house is
re-authored with different indices — and the revision contract, which
exists precisely to catch that, is not currently able to.

## Open questions

- All 103 exported houses share one `content_revision` today. If houses
  are later re-exported individually, the house wall must group by
  `(city_id, house_id)` across revisions rather than by revision.
- Asset upload has no route yet, so the deployment order in
  `docs/puzzle-gravity-playtest-handoff.md` gains a step between
  catalogue import and build publication.
