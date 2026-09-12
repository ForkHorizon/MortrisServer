package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PuzzleAttemptSummary is one row of the attempt picker.
type PuzzleAttemptSummary struct {
	AttemptID         string    `json:"attempt_id"`
	InstallID         string    `json:"install_id"`
	StartedAt         time.Time `json:"started_at"`
	WaveIndex         int       `json:"wave_index"`
	Placements        int64     `json:"placements"`
	Falls             int64     `json:"falls"`
	Hints             int64     `json:"hints"`
	Completed         bool      `json:"completed"`
	AppVersion        string    `json:"app_version"`
	BuildNumber       string    `json:"build_number"`
	ActiveDurationMS  int64     `json:"active_duration_ms"`
	LastWaveIndex     int       `json:"last_wave_index"`
	DominantFailure   string    `json:"dominant_failure"`
	ProgressOrigin    string    `json:"progress_origin"`
	DeveloperAffected bool      `json:"developer_affected"`
}

func GetPuzzleAttempts(ctx context.Context, pool *pgxpool.Pool, projectID string, cityID, houseID int, from, to time.Time, build ...*string) ([]PuzzleAttemptSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	rows, err := pool.Query(ctx, `
SELECT properties->>'attempt_id', MIN(install_id::text),
       MIN(effective_at),
       COALESCE(MIN(CASE WHEN properties->>'wave_index' ~ '^[0-9]{1,9}$' THEN (properties->>'wave_index')::int ELSE NULL END), 0),
       COUNT(*) FILTER (WHERE name='placement_resolved'),
       COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%'),
       COUNT(*) FILTER (WHERE name='hint_used'),
       BOOL_OR(name IN ('wave_completed','house_completed')),
       COALESCE((array_agg(app_version ORDER BY effective_at DESC))[1], ''),
       COALESCE((array_agg(build_number ORDER BY effective_at DESC))[1], ''),
       COALESCE(MAX(CASE WHEN properties->>'active_elapsed_ms' ~ '^[0-9]{1,18}$' THEN (properties->>'active_elapsed_ms')::bigint ELSE NULL END) - MIN(CASE WHEN properties->>'active_elapsed_ms' ~ '^[0-9]{1,18}$' THEN (properties->>'active_elapsed_ms')::bigint ELSE NULL END), 0),
       COALESCE(MAX(CASE WHEN properties->>'wave_index' ~ '^[0-9]{1,9}$' THEN (properties->>'wave_index')::int ELSE NULL END), 0),
       COALESCE(MODE() WITHIN GROUP (ORDER BY properties->>'outcome') FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%'),''),
       COALESCE((array_agg(properties->>'progress_origin' ORDER BY effective_at DESC) FILTER (WHERE properties ? 'progress_origin'))[1],'natural'),
       COALESCE(BOOL_OR(properties->>'origin'='developer_menu' OR properties->>'close_reason'='developer_command' OR name LIKE 'developer_%'),false)
FROM events
WHERE project_id=$1 AND effective_at>=$4 AND effective_at<$5
	  AND ($6::text IS NULL OR build_number=$6)
  AND properties ? 'attempt_id' AND properties->>'attempt_id' IS NOT NULL AND properties->>'attempt_id' != ''
  AND properties->>'city_id'=$2::text
  AND properties->>'house_id'=$3::text
GROUP BY 1
ORDER BY MIN(effective_at) DESC
LIMIT 200`, projectID, cityID, houseID, from, to, optionalBuild(build))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PuzzleAttemptSummary{}
	for rows.Next() {
		var row PuzzleAttemptSummary
		if err := rows.Scan(&row.AttemptID, &row.InstallID, &row.StartedAt, &row.WaveIndex,
			&row.Placements, &row.Falls, &row.Hints, &row.Completed, &row.AppVersion, &row.BuildNumber,
			&row.ActiveDurationMS, &row.LastWaveIndex, &row.DominantFailure, &row.ProgressOrigin, &row.DeveloperAffected); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
