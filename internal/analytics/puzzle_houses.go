package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// minPlacementsForRate is the point below which a per-block or per-house
// fall rate is noise rather than a finding.
//
// Measured 2026-08-03 against the live playtest: three testers had touched
// 153 of 7065 blocks, and two thirds of those had two placements or fewer.
// A block painted red off two events looks exactly like a block painted red
// off two hundred, which is the failure this whole view exists to prevent.
// Below this count the API still returns the raw counts and the dashboard
// renders the block neutral, labelled as not yet played enough.
const minPlacementsForRate = 5

type PuzzleHouseSummary struct {
	CityID       int    `json:"city_id"`
	HouseID      int    `json:"house_id"`
	DisplayLabel string `json:"display_label"`
	// Attempts remains the historical field name. OpenedAttempts makes its
	// meaning explicit for a first-time analytics reader.
	Attempts            int64   `json:"attempts"`
	OpenedAttempts      int64   `json:"opened_attempts"`
	CompletedAttempts   int64   `json:"completed_attempts"`
	UniqueInstallations int64   `json:"unique_installations"`
	PlayedDetailCount   int64   `json:"played_detail_count"`
	ReliableDetailCount int64   `json:"reliable_detail_count"`
	DominantFailure     string  `json:"dominant_failure"`
	Placements          int64   `json:"placements"`
	Falls               int64   `json:"falls"`
	Hints               int64   `json:"hints"`
	FallRate            float64 `json:"fall_rate"`
	// RateIsReliable is false when Placements < minPlacementsForRate. The
	// dashboard must not colour a house by an unreliable rate.
	RateIsReliable bool `json:"rate_is_reliable"`
	HasGeometry    bool `json:"has_geometry"`
	HasArt         bool `json:"has_art"`
}

type PuzzleHouseList struct {
	ContentRevision string               `json:"content_revision"`
	Houses          []PuzzleHouseSummary `json:"houses"`
}

// latestPuzzleRevision returns the newest imported catalogue revision.
// Layout is drawn from it; event semantics still resolve per event.
func latestPuzzleRevision(ctx context.Context, pool *pgxpool.Pool, projectID string) (string, error) {
	var revision string
	err := pool.QueryRow(ctx, `SELECT content_revision FROM puzzle_content_revisions WHERE project_id=$1 ORDER BY imported_at DESC LIMIT 1`, projectID).Scan(&revision)
	return revision, err
}

// GetPuzzleHouses lists every house in the catalogue with its play
// outcomes, including houses nobody has opened — "nobody has played this"
// is itself a finding, and hiding untouched houses would make the wall
// silently describe only the corner of the game that happens to have data.
//
// scope splits at two granularities (Stage 3, docs/puzzle-analytics-remaining-plan.md
// section 6): placements/falls/hints/played-detail-count are placement-
// quality metrics and stay event-level; attempts/completed_attempts/
// unique_installations answer "did this house get played/finished," so
// they're gated on the whole wave attempt being in scope, via
// puzzle_wave_attempt_classification, not just the surviving event rows.
func GetPuzzleHouses(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, scope TrafficScope) (*PuzzleHouseList, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	revision, err := latestPuzzleRevision(ctx, pool, projectID)
	if err != nil {
		return nil, err
	}
	result := &PuzzleHouseList{ContentRevision: revision, Houses: []PuzzleHouseSummary{}}
	rows, err := pool.Query(ctx, puzzleHousesQuery(scope), projectID, revision, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var row PuzzleHouseSummary
		if err := rows.Scan(&row.CityID, &row.HouseID, &row.DisplayLabel, &row.HasGeometry, &row.HasArt,
			&row.Attempts, &row.CompletedAttempts, &row.UniqueInstallations, &row.PlayedDetailCount, &row.ReliableDetailCount, &row.DominantFailure,
			&row.Placements, &row.Falls, &row.Hints); err != nil {
			return nil, err
		}
		row.OpenedAttempts = row.Attempts
		if row.Placements > 0 {
			row.FallRate = float64(row.Falls) / float64(row.Placements)
		}
		row.RateIsReliable = row.Placements >= minPlacementsForRate
		result.Houses = append(result.Houses, row)
	}
	return result, rows.Err()
}

// The catalogue is the spine of this query, not the events, so untouched
// houses survive the join. display_label lives only inside the stored
// catalogue document, hence the jsonb path lookup per house.
//
// scope is spliced in as one of three fixed literals (TrafficScope.
// eventPredicate/runPredicate), never raw input — see puzzle_traffic.go.
// Split into two functions/a const purely to stay under the repo's
// 50-line function gate; puzzleHousesCTEs and puzzleHousesSelect are not
// meant to be read as independent queries.
func puzzleHousesQuery(scope TrafficScope) string {
	return puzzleHousesCTEs(scope) + puzzleHousesSelect
}

func puzzleHousesCTEs(scope TrafficScope) string {
	return `
WITH house AS (
    SELECT city_id, house_id,
           bool_or(bounds_min_x_milli IS NOT NULL) has_geometry
    FROM puzzle_content_blocks
    WHERE project_id=$1 AND content_revision=$2
    GROUP BY 1,2
), eligible_attempts AS (
    SELECT attempt_id FROM puzzle_wave_attempt_classification
    WHERE project_id=$1 AND last_event_at>=$3 AND last_event_at<$4 AND ` + scope.runPredicate("fully_natural") + `
), ev AS (
    SELECT (properties->>'city_id')::int city_id,
           (properties->>'house_id')::int house_id,
           name, properties, install_id
    FROM events
    WHERE project_id=$1 AND effective_at>=$3 AND effective_at<$4
      AND properties ? 'house_id' AND properties ? 'attempt_id'
      AND ` + scope.eventPredicate() + `
), attempt_agg AS (
    -- Attempts/completions need the whole attempt in scope (plan section 6).
    SELECT city_id, house_id,
           COUNT(DISTINCT properties->>'attempt_id') attempts,
           COUNT(DISTINCT properties->>'attempt_id') FILTER (WHERE name='house_completed') completed_attempts,
           COUNT(DISTINCT install_id) unique_installations
    FROM ev WHERE properties->>'attempt_id' IN (SELECT attempt_id FROM eligible_attempts)
    GROUP BY 1,2
), metric_agg AS (
    -- Placement-quality metrics stay event-level (plan section 6, rule 1).
    SELECT city_id, house_id,
           COUNT(*) FILTER (WHERE name='placement_resolved') placements,
           COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%') falls,
           COUNT(*) FILTER (WHERE name='hint_used') hints,
           COALESCE(MODE() WITHIN GROUP (ORDER BY properties->>'outcome') FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%'), '') dominant_failure
    FROM ev GROUP BY 1,2
), detail_counts AS (
	SELECT city_id, house_id, COUNT(*) played_details,
		COUNT(*) FILTER (WHERE placements >= 5) reliable_details
	FROM (
		SELECT city_id, house_id, properties->>'block_id' block_id, COUNT(*) placements
		FROM ev WHERE name='placement_resolved' AND properties ? 'block_id'
		GROUP BY 1,2,3
	) blocks
	GROUP BY 1,2
)
`
}

const puzzleHousesSelect = `
SELECT h.city_id, h.house_id,
       COALESCE(jsonb_path_query_first(r.catalog,
           ('$.cities[*] ? (@.city_id == ' || h.city_id || ').houses[*] ? (@.house_id == ' || h.house_id || ').display_label')::jsonpath
       ) #>> '{}', '') display_label,
       h.has_geometry,
	       EXISTS(SELECT 1 FROM puzzle_house_art art WHERE art.project_id=$1 AND art.content_revision=$2 AND art.city_id=h.city_id AND art.house_id=h.house_id) has_art,
       COALESCE(a.attempts,0), COALESCE(a.completed_attempts,0), COALESCE(a.unique_installations,0),
	   COALESCE(d.played_details,0), COALESCE(d.reliable_details,0), COALESCE(m.dominant_failure,''),
	   COALESCE(m.placements,0), COALESCE(m.falls,0), COALESCE(m.hints,0)
FROM house h
JOIN puzzle_content_revisions r ON r.project_id=$1 AND r.content_revision=$2
LEFT JOIN attempt_agg a ON a.city_id=h.city_id AND a.house_id=h.house_id
LEFT JOIN metric_agg m ON m.city_id=h.city_id AND m.house_id=h.house_id
LEFT JOIN detail_counts d ON d.city_id=h.city_id AND d.house_id=h.house_id
ORDER BY COALESCE(m.placements,0) DESC, h.city_id, h.house_id`
