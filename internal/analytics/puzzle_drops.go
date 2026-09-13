package analytics

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A drop is one release of a detail: where the player let go, and which
// target that attempt was aimed at. Plotted over the house it separates
// failures that look identical in a table — a tight cluster offset from
// the slot (the art reads as sitting somewhere it does not), a wide
// scatter (snap radius too small), or a cluster on a different slot
// (the player cannot tell two compatible targets apart).
type PuzzleDrop struct {
	BlockID                   int    `json:"block_id"`
	TargetID                  int    `json:"target_id"`
	CandidateTargetID         int    `json:"candidate_target_id"`
	CandidateTargetX          *int   `json:"candidate_target_x_milli,omitempty"`
	CandidateTargetY          *int   `json:"candidate_target_y_milli,omitempty"`
	NearestCompatibleTargetID int    `json:"nearest_compatible_target_id"`
	NearestTargetX            *int   `json:"nearest_target_x_milli,omitempty"`
	NearestTargetY            *int   `json:"nearest_target_y_milli,omitempty"`
	NearestDistance           *int   `json:"nearest_distance_milli,omitempty"`
	Outcome                   string `json:"outcome"`
	RuleState                 string `json:"rule_state,omitempty"`
	ReleaseX                  int    `json:"release_x_milli"`
	ReleaseY                  int    `json:"release_y_milli"`
	TargetX                   int    `json:"target_x_milli"`
	TargetY                   int    `json:"target_y_milli"`
	AttemptID                 string `json:"attempt_id"`
	Legacy                    bool   `json:"legacy_coordinates"`
}

type PuzzleDropMap struct {
	Drops              []PuzzleDrop `json:"drops"`
	UnresolvedDrops    int          `json:"unresolved_drops"` // drops whose target could not be matched
	OffsetX            int          `json:"offset_x_milli"`   // world-to-house correction subtracted from release
	OffsetY            int          `json:"offset_y_milli"`   // world-to-house correction subtracted from release
	Aligned            bool         `json:"aligned"`
	AlignmentIssue     string       `json:"alignment_issue,omitempty"` // "too_few_placed" or "inconsistent"
	Spread             int          `json:"offset_spread_milli"`       // median absolute deviation of offset estimate
	TrustedCoordinates bool         `json:"trusted_coordinates"`
}

// snapDistanceMilli is CheckTruePositionService's maxSnapDistance (1.5
// world units). A placed drop is by definition within it of its target,
// which is what bounds the offset estimate below.
const snapDistanceMilli = 1500

// minPlacedForAlignment is how many placed drops the offset estimate
// needs before it is trusted. Below this a couple of unlucky releases
// would drag the median and silently shift the whole map.
const minPlacedForAlignment = 5

// GetPuzzleDrops returns release points for one house, optionally one
// block, joined to target positions.
//
// Events with an imported revision join that exact layout. Events from
// before the corrected client build have an unimported local hash, so they
// are labelled legacy and use the latest layout only as a best-effort
// fallback. A mixed range is deliberately withheld rather than plotting
// two coordinate spaces as though they agree.
func GetPuzzleDrops(ctx context.Context, pool *pgxpool.Pool, projectID string, cityID, houseID int, blockID *int, from, to time.Time, scope TrafficScope, build ...*string) (*PuzzleDropMap, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	revision, err := latestPuzzleRevision(ctx, pool, projectID)
	if err != nil {
		return nil, err
	}
	result := &PuzzleDropMap{Drops: []PuzzleDrop{}}
	rows, err := pool.Query(ctx, puzzleDropsQuery(scope), projectID, revision, cityID, houseID, from, to, blockID, optionalBuild(build))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var drop PuzzleDrop
		var candX, candY, nearX, nearY, clientNearDist *int
		if err := rows.Scan(
			&drop.BlockID, &drop.CandidateTargetID, &candX, &candY,
			&drop.NearestCompatibleTargetID, &nearX, &nearY, &clientNearDist,
			&drop.Outcome, &drop.RuleState, &drop.ReleaseX, &drop.ReleaseY,
			&drop.AttemptID, &drop.Legacy,
		); err != nil {
			return nil, err
		}
		computeDropDistance(&drop, nearX, nearY, clientNearDist)
		if !resolveDropTarget(&drop, candX, candY, nearX, nearY) {
			result.UnresolvedDrops++
			continue
		}
		result.Drops = append(result.Drops, drop)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	alignDrops(result)
	return result, nil
}

func computeDropDistance(drop *PuzzleDrop, nearX, nearY, clientNearDist *int) {
	if nearX != nil && nearY != nil {
		dx := float64(drop.ReleaseX - *nearX)
		dy := float64(drop.ReleaseY - *nearY)
		dist := int(math.Round(math.Hypot(dx, dy)))
		drop.NearestDistance = &dist
		return
	}
	if clientNearDist != nil {
		drop.NearestDistance = clientNearDist
	}
}

func resolveDropTarget(drop *PuzzleDrop, candX, candY, nearX, nearY *int) bool {
	drop.CandidateTargetX, drop.CandidateTargetY = candX, candY
	drop.NearestTargetX, drop.NearestTargetY = nearX, nearY
	if drop.CandidateTargetID >= 0 {
		if candX != nil && candY != nil {
			drop.TargetID = drop.CandidateTargetID
			drop.TargetX, drop.TargetY = *candX, *candY
			return true
		}
		return false
	}
	if drop.NearestCompatibleTargetID >= 0 && nearX != nil && nearY != nil {
		drop.TargetID = drop.NearestCompatibleTargetID
		drop.TargetX, drop.TargetY = *nearX, *nearY
		return true
	}
	drop.TargetID = -1
	return true
}

// alignDrops converts legacy release points from world space into the house
// space the layout is drawn in. Corrected coordinates need no fitting.
func alignDrops(result *PuzzleDropMap) {
	result.TrustedCoordinates = allDropRevisionsKnown(result.Drops)
	if result.TrustedCoordinates || len(result.Drops) == 0 {
		result.Aligned = true
		return
	}
	if hasMixedCoordinates(result.Drops) {
		result.AlignmentIssue = "mixed_coordinate_spaces"
		result.Drops = []PuzzleDrop{}
		return
	}
	fitLegacyDrops(result)
}

func hasMixedCoordinates(drops []PuzzleDrop) bool {
	legacy, corrected := false, false
	for _, drop := range drops {
		if drop.Legacy {
			legacy = true
		} else {
			corrected = true
		}
		if legacy && corrected {
			return true
		}
	}
	return false
}

func fitLegacyDrops(result *PuzzleDropMap) {
	dx, dy := make([]int, 0, len(result.Drops)), make([]int, 0, len(result.Drops))
	for _, drop := range result.Drops {
		if drop.Outcome == "placed" && drop.TargetID >= 0 {
			dx = append(dx, drop.ReleaseX-drop.TargetX)
			dy = append(dy, drop.ReleaseY-drop.TargetY)
		}
	}
	if len(dx) < minPlacedForAlignment {
		result.AlignmentIssue = "too_few_placed"
		result.Drops = []PuzzleDrop{}
		return
	}
	offsetX, offsetY := intMedian(dx), intMedian(dy)
	spread := max(dropSpread(dx, offsetX), dropSpread(dy, offsetY))
	if spread > snapDistanceMilli {
		result.Aligned, result.Spread, result.AlignmentIssue = false, spread, "inconsistent"
		result.Drops = []PuzzleDrop{}
		return
	}
	result.Aligned, result.OffsetX, result.OffsetY, result.Spread = true, offsetX, offsetY, spread
	for i := range result.Drops {
		result.Drops[i].ReleaseX -= offsetX
		result.Drops[i].ReleaseY -= offsetY
		if result.Drops[i].NearestTargetX != nil && result.Drops[i].NearestTargetY != nil {
			computeDropDistance(&result.Drops[i], result.Drops[i].NearestTargetX, result.Drops[i].NearestTargetY, nil)
		}
	}
}

func allDropRevisionsKnown(drops []PuzzleDrop) bool {
	return len(drops) > 0 && !slices.ContainsFunc(drops, func(drop PuzzleDrop) bool { return drop.Legacy })
}

func intMedian(values []int) int {
	sorted := append([]int(nil), values...)
	slices.Sort(sorted)
	return sorted[len(sorted)/2]
}

// dropSpread is the median absolute deviation around centre. Named apart
// from anomalies.go's float64 medianAbsDeviation, which these integer
// milli-unit coordinates would only lose precision passing through.
func dropSpread(values []int, centre int) int {
	deviations := make([]int, len(values))
	for i, v := range values {
		deviations[i] = intAbs(v - centre)
	}
	return intMedian(deviations)
}

func intAbs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// candidate_target_id is the slot the drop actually snapped to and is -1
// when nothing was in range; nearest_compatible_target_id is the closest
// slot that would have accepted the piece. Preferring the candidate and
// falling back to the nearest is what makes a "fell nowhere near a slot"
// drop still point at the slot the player was probably aiming for.
const puzzleDropsQueryTemplate = `
WITH d AS (
    SELECT COALESCE(CASE WHEN properties->>'block_id' ~ '^[0-9]{1,9}$' THEN (properties->>'block_id')::int END, -1) block_id,
           COALESCE(
               CASE WHEN properties->>'candidate_target_id' ~ '^-?[0-9]{1,9}$' THEN (properties->>'candidate_target_id')::int END,
               CASE WHEN properties->>'target_id' ~ '^-?[0-9]{1,9}$' THEN (properties->>'target_id')::int END,
               -1
           ) candidate_target_id,
           COALESCE(CASE WHEN properties->>'nearest_compatible_target_id' ~ '^-?[0-9]{1,9}$' THEN (properties->>'nearest_compatible_target_id')::int END, -1) nearest_compatible_target_id,
           CASE WHEN properties->>'nearest_target_distance_milli' ~ '^[0-9]{1,9}$' THEN (properties->>'nearest_target_distance_milli')::int END client_near_dist,
           COALESCE(properties->>'outcome','') outcome,
           COALESCE(properties->>'rule_state','') rule_state,
           (CASE WHEN properties->>'release_x_milli' ~ '^-?[0-9]{1,9}$' THEN (properties->>'release_x_milli')::int END) release_x,
           (CASE WHEN properties->>'release_y_milli' ~ '^-?[0-9]{1,9}$' THEN (properties->>'release_y_milli')::int END) release_y,
           COALESCE(properties->>'attempt_id','') attempt_id,
           effective_at,
           (COALESCE(CASE WHEN properties->>'schema_version' ~ '^[0-9]{1,9}$' THEN (properties->>'schema_version')::int END, 1) < 2
            OR COALESCE(properties->>'coordinate_space','') <> 'house_local') AS is_legacy,
           NULLIF(properties->>'content_revision','') content_revision
    FROM events
    WHERE project_id=$1 AND name='placement_resolved'
      AND effective_at>=$5 AND effective_at<$6
      AND ($8::text IS NULL OR build_number=$8)
      AND CASE WHEN properties->>'city_id' ~ '^[0-9]{1,9}$' THEN (properties->>'city_id')::int END = $3
      AND CASE WHEN properties->>'house_id' ~ '^[0-9]{1,9}$' THEN (properties->>'house_id')::int END = $4
      AND properties->>'release_x_milli' ~ '^-?[0-9]{1,9}$'
      AND properties->>'release_y_milli' ~ '^-?[0-9]{1,9}$'
      AND properties->>'block_id' ~ '^[0-9]{1,9}$'
      AND (
          COALESCE(CASE WHEN properties->>'schema_version' ~ '^[0-9]{1,9}$' THEN (properties->>'schema_version')::int END, 1) < 2
          OR COALESCE(properties->>'coordinate_space','') = 'house_local'
      )
      AND %s
      AND ($7::int IS NULL OR CASE WHEN properties->>'block_id' ~ '^[0-9]{1,9}$' THEN (properties->>'block_id')::int END = $7)
)
SELECT d.block_id, d.candidate_target_id, tc.local_x_milli, tc.local_y_milli,
       d.nearest_compatible_target_id, tn.local_x_milli, tn.local_y_milli,
       d.client_near_dist, d.outcome, d.rule_state, d.release_x, d.release_y,
       d.attempt_id, d.is_legacy
FROM d
LEFT JOIN puzzle_content_revisions r
       ON r.project_id=$1 AND r.content_revision=d.content_revision
LEFT JOIN puzzle_content_targets tc
       ON tc.project_id=$1 AND tc.content_revision=COALESCE(r.content_revision,$2)
      AND tc.city_id=$3 AND tc.house_id=$4 AND tc.target_id=d.candidate_target_id
LEFT JOIN puzzle_content_targets tn
       ON tn.project_id=$1 AND tn.content_revision=COALESCE(r.content_revision,$2)
      AND tn.city_id=$3 AND tn.house_id=$4 AND tn.target_id=d.nearest_compatible_target_id
ORDER BY d.effective_at DESC, d.block_id
LIMIT 2000`

func puzzleDropsQuery(scope TrafficScope) string {
	return fmt.Sprintf(puzzleDropsQueryTemplate, scope.eventPredicate())
}
