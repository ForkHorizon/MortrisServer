package analytics

// Stage 4 (docs/puzzle-analytics-remaining-plan.md section 7): "which
// houses have enough play to evaluate" needs a house-run-scoped count,
// not the wave-attempt-scoped one puzzle_houses.go already has — a house
// with ten waves would otherwise look ten times more played than an
// otherwise-identical one-wave house. These fields are always computed
// against natural traffic specifically (their names say so), independent
// of the page's selected TrafficScope: coverage is a baseline fact about
// the evidence itself, not a population the designer is choosing to
// debug. Split into their own file/functions to keep puzzle_houses.go
// under the repo's line/function-length gates.

// puzzleHousesCoverageCTEs is concatenated into puzzleHousesCTEs. It never
// takes a scope argument — see the file doc comment.
func puzzleHousesCoverageCTEs() string {
	return `, natural_runs AS (
    SELECT city_id, house_id, house_run_id, completed, last_event_at, last_build_number
    FROM puzzle_house_run_classification
    WHERE project_id=$1 AND last_event_at>=$3 AND last_event_at<$4 AND fully_natural
), all_runs AS (
    SELECT city_id, house_id, COUNT(*) total_runs
    FROM puzzle_house_run_classification
    WHERE project_id=$1 AND last_event_at>=$3 AND last_event_at<$4
    GROUP BY 1,2
), run_agg AS (
    SELECT city_id, house_id, COUNT(*) started, COUNT(*) FILTER (WHERE completed) completed,
           MAX(last_event_at) last_play_at,
           (array_agg(last_build_number ORDER BY last_event_at DESC))[1] last_build
    FROM natural_runs GROUP BY 1,2
), wave_reach AS (
    SELECT (properties->>'city_id')::int city_id, (properties->>'house_id')::int house_id,
           COUNT(DISTINCT (properties->>'wave_index')::int) waves_reached
    FROM events
    WHERE project_id=$1 AND effective_at>=$3 AND effective_at<$4
      AND properties ? 'house_run_id' AND properties ? 'wave_index'
      AND properties->>'house_run_id' IN (SELECT house_run_id FROM natural_runs)
    GROUP BY 1,2
)`
}

// evidenceState is Stage 4's four-way coverage bucket. "Worst first"
// ranking (HousesPage) must only ever sort within usable/mixed_quality —
// unplayed and thin houses are coverage work, not a difficulty finding.
func evidenceState(house *PuzzleHouseSummary) string {
	if house.NaturalHouseRunsStarted == 0 {
		return "unplayed"
	}
	if !house.RateIsReliable {
		return "thin"
	}
	if house.DeveloperRunsObserved {
		return "mixed_quality"
	}
	return "usable"
}
