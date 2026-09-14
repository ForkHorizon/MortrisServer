# Puzzle Release Smoke Gate & Quality Alerts Runbook

## Overview
This runbook defines the repeatable acceptance gate for every candidate build of the Puzzle game before it is approved for full tester distribution (Stage 7 of `docs/puzzle-analytics-remaining-plan.md`). It also documents production monitoring alerts and incident remediation.

---

## 1. Candidate Build Smoke Verification Procedure

For every candidate release build, execute the following 13-point verification workflow on a physical Android test device or emulator:

### Device Execution Checklist
1. **Record App Version and Build Number**:
   - Note the exact Git commit, app version (e.g. `2.0.0`), and build number (e.g. `204`).
2. **Clear App Data or Reinstall**:
   - Uninstall the previous build or perform "Clear Storage" to ensure no stale durable queue events pollute the test.
3. **Fresh Registration**:
   - Launch the app and confirm installation registration completes successfully.
4. **Natural House & Wave Progression**:
   - Complete at least one normal house and wave attempt without developer cheats.
5. **Produce `fell_no_snap_target`**:
   - Drag a piece and release it away from any valid snap slot. Verify fall.
6. **Produce `fell_missing_support`**:
   - Place a piece over an overhang without support blocks. Verify fall.
7. **Return and Retry**:
   - Wait for the fallen piece to return to inventory, then pick it up and successfully place it.
8. **App Background and Foreground**:
   - Press the home button, wait 5 seconds, and resume the app.
9. **Attempt Recovery**:
   - During an active wave attempt, kill the app process and restart. Verify the attempt recovers cleanly from state checkpoint.
10. **Developer Cheats Execution**:
    - Open the developer menu. Run one progress-completing cheat (e.g. `CompleteHouse99`) and one reset cheat (e.g. `ResetHouse`).
11. **Force Telemetry Flush**:
    - Send the app to background to trigger immediate flush of the durable telemetry queue to the analytics server.
12. **Verify Schema-v2 & Catalog Integrity**:
    - Ensure zero rejections, zero unknown revisions, and zero coordinate or checkpoint mismatches.
13. **Confirm Natural Metrics Isolation**:
    - Confirm natural designer metrics ignore cheated completions and resets.

---

## 2. Automated Smoke Verification Tool Execution

Run the automated verification CLI against production or staging:

```bash
# Set database connection string:
export MORTRIS_WRITER_DSN="postgres://user:pass@host:5432/mortris"

# Using the shell wrapper:
./tools/puzzle_smoke_check.sh -p puzzle -b <build_number>

# Or with JSON output:
./tools/puzzle_smoke_check.sh -p puzzle -b <build_number> --json

# Direct binary execution:
bin/analytics-server smoke-check -project puzzle -build <build_number>
```

### Self-Verification Against Master Fixture:
To verify the testing tool and server validation logic independently of a live client run:
```bash
./tools/puzzle_smoke_check.sh --fixture
```

### Exit Codes:
- `0`: **PASS** — Build candidate satisfies all 13 criteria and is APPROVED.
- `1`: **FAIL** — One or more criteria failed. Build candidate is REJECTED.

---

## 3. Release Gate Criteria

A build candidate must be rejected if any of the following occur:
- **Index Gaps or Sequence Duplicates > 0**: Durable queue sequence corrupted.
- **Orphan Interactions > 0**: Detail interaction chain never completed.
- **Catalog Rejections > 0**: Client emitted properties rejected by strict catalog.
- **Unknown Revision > 0**: Client running content revision unimported on server.
- **Coordinate Space Mismatch > 0**: Schema-v2 placement emitted outside `house_local`.
- **Checkpoint Hash Mismatch > 0**: State hash does not match block IDs.
- **Unpaired Developer Actions > 0**: Cheat start without terminal result.
- **Natural Metrics Pollution**: Cheated completions leaked into natural completion rate.

---

## 4. Production Quality Monitoring & Alerts

The server evaluates and surfaces categorized alerts in `GET /api/v1/analytics/gameplay/quality`:

| Alert Category | Severity | Condition | Root Cause & Action |
|---|---|---|---|
| `rejection_spike` | Critical | `rejected_events > 0` | Client sent undeclared property or unknown event name. Check `/events` and inspect client catalog version. |
| `unknown_revision` | Critical | `unknown_revision_events > 0` | Client deployed with content revision not imported into server database. Run catalog import at `/catalog` for that revision immediately. |
| `schema_share_drop` | Warning | `schema_v2_share < 0.95` | Older app versions active or schema_version missing from client events. Inspect `/gameplay`. |
| `missing_schema_version` | Warning | `missing_schema_version_events > 0` | Gameplay events emitted without schema_version integer field. Inspect `/events`. |
| `coordinate_mismatch` | Critical | `invalid_coordinate_space_events > 0` | Events emitted in world space instead of `house_local`. Reject client build. Inspect `/events?name=placement_resolved`. |
| `sequence_integrity` | Critical | `sequence_gaps > 0` | Durable queue dropped events during upload. Inspect network retry logs at `/gameplay`. |
| `index_gap` | Critical | `event_index_gaps > 0` | Event index discontinuity within house or attempt lifecycle. Inspect `/gameplay`. |
| `orphan_interaction` | Critical | `orphan_interactions > 0` | Interaction lifecycle severed (no release/resolution/return). Inspect `/houses`. |
| `checkpoint_mismatch` | Critical | `checkpoint_mismatches > 0` | Placed block SHA256 hash does not match placed blocks. Inspect `/events?name=state_checkpoint`. |
| `unpaired_developer_action` | Critical | `unpaired_developer_actions > 0` | Developer command started without completed/failed/noop terminal event. Inspect `/gameplay`. |
| `delivery_delay_anomaly` | Warning | `p90_delay_ms > 10000` | Ingestion latency spike. Check ingestion service queue and server load at `/gameplay`. |
