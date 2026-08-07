# Puzzle visual analytics — handoff

Written 2026-08-03 for whoever picks this up next. The design reasoning
lives in [`puzzle-visual-analytics-plan.md`](puzzle-visual-analytics-plan.md);
this file is state, gotchas, and what to do next. Read the plan for *why*,
this for *where things are*.

## The goal, in one line

Turn Mortris's `/gameplay` page from tables of numeric block IDs into a
picture of the house, so a designer who has never used an analytics tool
can see which house is failing, which detail inside it, and why.

## Three repos are involved

| Repo | Path | Role |
|---|---|---|
| Mortris | `~/Daliys/MortrisServer` | Go analytics server + React dashboard. All server work happens here. |
| Puzzle | `~/Daliys/UnityProjects/Puzzle` | The Unity game. Emits the events, exports the content catalogue. |
| — | production `143.14.22.61` | One VPS shared with five other ForkHorizon products. |

## Where it runs

- **https://houseart.mortris.forkhorizon.com** — dedicated hostname, opens
  on the house wall with the project pinned and the nav trimmed.
- **https://mortris.forkhorizon.com/houses** — same thing inside the full
  dashboard.

Both are the **same binary on the same port**. The split is client-side
(`dashboard/src/houseArtHost.ts`) and is presentation only — auth, CSRF
and per-project checks are server-side and identical on both.

Production access: passwordless SSH as `daliys`, `sudo -n` works. DSNs are
in `/etc/mortris/env` (root-only). Run read queries server-side so the
credential never leaves the box:

```bash
ssh 143.14.22.61 'sudo -n bash -s' <<'EOF'
set -a; . /etc/mortris/env; set +a
psql "$MORTRIS_READER_DSN" -At -c "SELECT ..."
EOF
```

Deploy is rsync → `make build` → `migrate` → `systemctl restart`, per
`deploy/README.md`. Always `cp bin/analytics-server bin/analytics-server.pre-<tag>-$(date …)`
first; several such rollback copies are already there.

## State

**Mortris:** branch `feat/puzzle-house-geometry`, 11 commits, pushed,
[PR #31](https://github.com/ForkHorizon/MortrisServer/pull/31), working
tree clean. **Production runs this branch, ahead of the merge** — the user
wanted to test before merging. Migrations `0010`–`0012` applied.

**Puzzle: two fixes are edits in the working tree, NOT committed.** They
sit on branch `new-sprites-processing` alongside the user's own unrelated
uncommitted work (deleted MishMashBurg assets, modified `Game.unity`, SDK
settings). Deliberately left for the user to land — do not commit or
branch over their work without asking.

- `Assets/Scripts/Gameplay/Services/CheckTruePositionService.cs`
- `Assets/Resources/PuzzleAnalyticsContentRevisions.asset`
- `Docs/PuzzleAnalyticsContentCatalog.md`

Neither takes effect until a Unity build ships to testers.

## What was built

Phases P0–P3 plus the support graph, all deployed.

- **Block geometry.** `migrations/0010`, `internal/analytics/puzzle_geometry.go`,
  `tools/puzzle_geometry_export.py`, `import-puzzle-geometry` subcommand.
  7065 blocks across 103 houses.
- **House art.** `migrations/0011`+`0012`, `internal/analytics/puzzle_art.go`,
  `tools/puzzle_art_export.py`, `import-puzzle-art` subcommand. 4.2 MB of
  cropped WebP stored in Postgres.
- **House wall + house X-ray.** `puzzle_houses.go`, `puzzle_house_detail.go`,
  `HousesPage.tsx`, `HouseDetailPage.tsx` (+ `houseDetailParts.tsx`).
- **Drop-miss map.** `puzzle_drops.go`, `houseOverlays.tsx`.
- **Attempt replay.** `puzzle_replay.go`, `ReplayPlayer.tsx`, `AttemptPicker.tsx`.
- **Support graph.** `houseOverlays.tsx` + `houseColors.ts`. Catalogue-driven,
  no new endpoint.

Endpoints, all under `/api/v1/analytics/gameplay/`:
`houses`, `houses/{city}/{house}`, `.../art`, `.../drops`, `.../attempts`,
`attempts/{id}/replay`. Plus the admin upload
`POST /api/v1/projects/{id}/puzzle-content/{revision}/geometry`.

## Things that will bite you

**1. No event references an imported content revision.** All 936 events
carry per-house JSON-hash revisions from the client's dev fallback. Every
query therefore joins on `(city, house, block)` against the *latest*
imported revision, not on revision. Correct only while block indices stay
stable. Root cause was `PuzzleAnalyticsContentRevisions.asset` having
`m_Script: {fileID: 0}` — no script binding, so `Resources.Load` returned
null silently. Fixed in the working tree, needs a build.

> **Resolved.** Built and proven on a real Android smoke test (see
> `docs/puzzle-analytics-remaining-plan.md` section 1): production events
> now carry the imported revision `09f3b91e…` and join correctly. The
> replay resolver now rejects an unknown schema-v2 revision with a visible
> `unknown_content_revision` failure. Legacy replay fallback remains
> available only through an explicit legacy path and carries a warning.

**2. Release positions were world coordinates.** Blocks and targets are
house-local; each house sits at its own world offset. `alignDrops` in
`puzzle_drops.go` recovers the offset from placed drops and *refuses to
draw* when it cannot — two of five played houses currently refuse. Fixed
client-side in the working tree. **When that build ships, a date range
spanning old and new data will be bimodal and refuse.** That is correct,
not a regression; pick a range starting after the build.

> **Resolved.** That build shipped; production events now carry
> `coordinate_space=house_local` release positions directly, no
> `alignDrops` offset recovery needed for current data. Negative
> house-local X/Y are valid coordinates and occurred in the production
> smoke test — never treat a negative coordinate as missing.

**3. The sample is tiny.** 3 installations, 45 attempts, 153 of 7065
blocks ever touched, two-thirds of those with ≤2 placements. Everything
per-block is gated at 5 placements (`minPlacementsForRate`) and renders
neutral grey below it. **Do not remove that gate to make the page look
livelier** — it is the whole reason the page is trustworthy.

**4. Every migration that creates a table must also grant on it.** Runtime
pools use least-privilege roles. `0011` shipped without grants and the art
import failed in production; `0012` fixed it. `0006` is the same fix for
the `0004` tables.

**5. Certbot.** The TLS cert is shared across eight hostnames. Always pass
**all** of them in one `certbot --nginx --expand` call — passing only a new
one silently drops the rest. See `deploy/nginx/houseart.conf`'s header.

**6. Line gates.** CI enforces 300 lines/file and 50 lines/function on
`.go`, `.ts`, `.tsx`, `.py`. Several files here were split for exactly
this reason. Four pre-existing violations elsewhere in the repo are not
yours (`deploy/loadtest`, `funnel.go`, `policyprobe.go`). Run:
`python3 <path>/linter-checker-300-lines.py --config .linter-checker-300-lines.json`

**7. `static_packed.png` is not the house.** It is the window/lights
layer. The assembled art is `composite.png`, despite what the catalogue's
`preview_asset_key` points at.

**8. `returned` is a dead outcome.** Declared in the catalogue, no call
site in the client emits it. Either wire it or drop it; do not build UI
that implies it exists.

> **Resolved.** Schema v2 wired a distinct `detail_returned` event into
> the interaction chain (take/release/resolve/return), proven on the
> production smoke test. It is no longer structurally zero — see
> `docs/puzzle-analytics-remaining-plan.md` section 1.

## Verifying locally

There is no docker. Use the local Homebrew Postgres 18 — it needs
`LC_ALL=C` or it refuses to start:

```bash
LC_ALL=C pg_ctl -D /opt/homebrew/var/postgresql@18 -l /tmp/pg18.log start
createdb mortris_dev
export MORTRIS_MIGRATOR_DSN=... MORTRIS_WRITER_DSN=... MORTRIS_READER_DSN=... MORTRIS_DISK_PATH=/ LISTEN_ADDR=127.0.0.1:8099
go run ./cmd/analytics-server migrate
```

Then import the real catalogue + geometry + art from the Puzzle repo with
the two `tools/*.py` exporters and the two import subcommands, and copy
production events in with `\copy` if you need real data. Delete the
database and any CSV copy afterwards.

`create-admin` needs role `owner|project_admin|viewer` and a 12+ char
password. The dashboard is embedded at build time, so re-run
`npm run build` in `dashboard/` before restarting the Go server or you
will be testing stale JS.

## What to do next

In rough value order:

1. **Ship the Puzzle build** with the two fixes. Everything else is
   waiting on the data it unlocks. Verify afterwards that events carry
   `09f3b91e…` and that all five played houses draw a drop map.
2. **Merge PR #31**, since production already runs it.
3. **More testers.** The dashboard is ready; the data is what is thin.
4. **P4 remainder** — retry ladder, time-to-place, pacing band, device and
   memory. Deliberately not built: with 3 testers they would all render as
   the same "not enough plays" grey. Build when the sample grows.
5. Decide on `returned`, and on whether `preview_asset_key` should point
   at `composite.png`.
