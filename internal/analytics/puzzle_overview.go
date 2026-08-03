package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PuzzleOverview struct {
	TotalHouses        int64               `json:"total_houses"`
	PlayedHouses       int64               `json:"played_houses"`
	ReliableHouses     int64               `json:"reliable_houses"`
	InsufficientHouses int64               `json:"insufficient_houses"`
	WorstHouse         *PuzzleHouseSummary `json:"worst_house,omitempty"`
	WorstWave          *PuzzleWaveSummary  `json:"worst_wave,omitempty"`
	FailureReasons     map[string]int64    `json:"failure_reasons"`
	DataQuality        PuzzleDataQuality   `json:"data_quality"`
}

func GetPuzzleOverview(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time) (*PuzzleOverview, error) {
	houses, err := GetPuzzleHouses(ctx, pool, projectID, from, to)
	if err != nil {
		return nil, err
	}
	result := summarizePuzzleHouses(houses.Houses)
	if err := loadPuzzleOverviewEvents(ctx, pool, result, projectID, from, to); err != nil {
		return nil, err
	}
	return result, nil
}

func summarizePuzzleHouses(houses []PuzzleHouseSummary) *PuzzleOverview {
	result := &PuzzleOverview{TotalHouses: int64(len(houses)), FailureReasons: map[string]int64{}}
	for i := range houses {
		house := &houses[i]
		if house.Placements == 0 {
			continue
		}
		result.PlayedHouses++
		if !house.RateIsReliable {
			result.InsufficientHouses++
			continue
		}
		result.ReliableHouses++
		if result.WorstHouse == nil || house.FallRate > result.WorstHouse.FallRate {
			result.WorstHouse = house
		}
	}
	return result
}

func loadPuzzleOverviewEvents(ctx context.Context, pool *pgxpool.Pool, result *PuzzleOverview, projectID string, from, to time.Time) error {
	rows, err := pool.Query(ctx, `
SELECT COALESCE(properties->>'outcome',''), (properties->>'wave_index')::int,
       COUNT(*), COUNT(*) FILTER (WHERE properties->>'outcome' LIKE 'fell_%')
FROM events WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3
  AND name='placement_resolved' GROUP BY 1,2`, projectID, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()
	waves := map[int]*PuzzleWaveSummary{}
	for rows.Next() {
		var outcome string
		var wave int
		var placements, falls int64
		if err := rows.Scan(&outcome, &wave, &placements, &falls); err != nil {
			return err
		}
		entry := waveFor(waves, wave)
		entry.Placements += placements
		entry.Falls += falls
		if len(outcome) > 5 && outcome[:5] == "fell_" {
			result.FailureReasons[outcome] += placements
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, wave := range waves {
		if wave.Placements > 0 {
			wave.FallRate = float64(wave.Falls) / float64(wave.Placements)
		}
		if result.WorstWave == nil || wave.FallRate > result.WorstWave.FallRate {
			result.WorstWave = wave
		}
	}
	return loadPuzzleOverviewQuality(ctx, pool, result, projectID, from, to)
}

func loadPuzzleOverviewQuality(ctx context.Context, pool *pgxpool.Pool, result *PuzzleOverview, projectID string, from, to time.Time) error {
	var legacy, corrected int64
	err := pool.QueryRow(ctx, `
SELECT COUNT(*) FILTER (WHERE r.content_revision IS NULL), COUNT(*) FILTER (WHERE r.content_revision IS NOT NULL)
FROM events e LEFT JOIN puzzle_content_revisions r ON r.project_id=e.project_id AND r.content_revision=e.properties->>'content_revision'
WHERE e.project_id=$1 AND e.effective_at>=$2 AND e.effective_at<$3 AND e.name='placement_resolved'`, projectID, from, to).Scan(&legacy, &corrected)
	result.DataQuality = PuzzleDataQuality{LegacyRevisionEvents: legacy, CoordinateStatus: coordinateStatus(legacy, corrected)}
	return err
}
