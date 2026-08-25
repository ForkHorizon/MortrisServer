package analytics

import (
	"context"
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
	BlockID   int    `json:"block_id"`
	TargetID  int    `json:"target_id"`
	Outcome   string `json:"outcome"`
	ReleaseX  int    `json:"release_x_milli"`
	ReleaseY  int    `json:"release_y_milli"`
	TargetX   int    `json:"target_x_milli"`
	TargetY   int    `json:"target_y_milli"`
	AttemptID string `json:"attempt_id"`
	Legacy    bool   `json:"legacy_coordinates"`
}

type PuzzleDropMap struct {
	Drops []PuzzleDrop `json:"drops"`
	// UnresolvedDrops counts drops whose target could not be matched in
	// the layout revision, so the caller can say how much is missing
	// instead of quietly plotting less than it measured.
	UnresolvedDrops int `json:"unresolved_drops"`
	// OffsetX/Y is the world-to-house correction subtracted from every
	// release point — see alignDrops. Aligned is false when it could not
	// be established, in which case Drops is empty rather than wrong.
	OffsetX int  `json:"offset_x_milli"`
	OffsetY int  `json:"offset_y_milli"`
	Aligned bool `json:"aligned"`
	// AlignmentIssue says WHY alignment failed, empty when it succeeded.
	// "too_few_placed" and "inconsistent" need different words in the UI:
	// the first is "come back when more people have played", the second is
	// "these positions disagree with each other".
	AlignmentIssue string `json:"alignment_issue,omitempty"`
	// Spread is the median absolute deviation of the offset estimate, in
	// milli-units. Surfaced so the caller can show how tight the fit was
	// rather than presenting a fitted map as ground truth.
	Spread             int  `json:"offset_spread_milli"`
	TrustedCoordinates bool `json:"trusted_coordinates"`
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
func GetPuzzleDrops(ctx context.Context, pool *pgxpool.Pool, projectID string, cityID, houseID int, blockID *int, from, to time.Time, build ...*string) (*PuzzleDropMap, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	revision, err := latestPuzzleRevision(ctx, pool, projectID)
	if err != nil {
		return nil, err
	}
	result := &PuzzleDropMap{Drops: []PuzzleDrop{}}
	rows, err := pool.Query(ctx, puzzleDropsQuery, projectID, revision, cityID, houseID, from, to, blockID, optionalBuild(build))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var drop PuzzleDrop
		var targetX, targetY *int
		if err := rows.Scan(&drop.BlockID, &drop.TargetID, &drop.Outcome,
			&drop.ReleaseX, &drop.ReleaseY, &targetX, &targetY, &drop.AttemptID, &drop.Legacy); err != nil {
			return nil, err
		}
		if targetX == nil || targetY == nil {
			result.UnresolvedDrops++
			continue
		}
		drop.TargetX, drop.TargetY = *targetX, *targetY
		result.Drops = append(result.Drops, drop)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result.TrustedCoordinates = allDropRevisionsKnown(result.Drops)
	alignDrops(result)
	return result, nil
}

// alignDrops converts legacy release points from world space into the house
// space the layout is drawn in. Corrected coordinates need no fitting.
func alignDrops(result *PuzzleDropMap) {
	result.TrustedCoordinates = allDropRevisionsKnown(result.Drops)
	if result.TrustedCoordinates {
		result.Aligned = true
		return
	}
	legacy, corrected := 0, 0
	for _, drop := range result.Drops {
		if drop.Legacy {
			legacy++
		} else {
			corrected++
		}
	}
	if legacy > 0 && corrected > 0 {
		result.AlignmentIssue = "mixed_coordinate_spaces"
		result.Drops = []PuzzleDrop{}
		return
	}
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
	// A correct offset leaves placed drops scattered within the snap
	// radius. Much wider than that means the house moved between builds,
	// or these events span layouts that no longer agree, and no single
	// offset describes them.
	if spread > snapDistanceMilli {
		result.Aligned, result.Spread, result.AlignmentIssue = false, spread, "inconsistent"
		result.Drops = []PuzzleDrop{}
		return
	}
	result.Aligned, result.OffsetX, result.OffsetY, result.Spread = true, offsetX, offsetY, spread
	for i := range result.Drops {
		result.Drops[i].ReleaseX -= offsetX
		result.Drops[i].ReleaseY -= offsetY
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
const puzzleDropsQuery = `
WITH d AS (
    SELECT (properties->>'block_id')::int block_id,
           NULLIF(COALESCE(
               NULLIF((properties->>'candidate_target_id')::int, -1),
               NULLIF((properties->>'nearest_compatible_target_id')::int, -1)
           ), -1) target_id,
           COALESCE(properties->>'outcome','') outcome,
           (properties->>'release_x_milli')::int release_x,
           (properties->>'release_y_milli')::int release_y,
           COALESCE(properties->>'attempt_id','') attempt_id,
           NULLIF(properties->>'content_revision','') content_revision
    FROM events
    WHERE project_id=$1 AND name='placement_resolved'
      AND effective_at>=$5 AND effective_at<$6
	  AND ($8::text IS NULL OR build_number=$8)
      AND (properties->>'city_id')::int=$3 AND (properties->>'house_id')::int=$4
      AND properties ? 'release_x_milli' AND properties ? 'block_id'
      AND COALESCE(properties->>'origin','player')='player'
      AND COALESCE(properties->>'progress_origin','natural')='natural'
      AND ($7::int IS NULL OR (properties->>'block_id')::int=$7)
)
SELECT d.block_id, COALESCE(d.target_id,-1), d.outcome, d.release_x, d.release_y,
       t.local_x_milli, t.local_y_milli, d.attempt_id, r.content_revision IS NULL
FROM d
LEFT JOIN puzzle_content_revisions r
       ON r.project_id=$1 AND r.content_revision=d.content_revision
LEFT JOIN puzzle_content_targets t
       ON t.project_id=$1 AND t.content_revision=COALESCE(r.content_revision,$2)
      AND t.city_id=$3 AND t.house_id=$4 AND t.target_id=d.target_id
ORDER BY d.block_id
LIMIT 2000`
