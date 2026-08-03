package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func loadPuzzleDataQuality(ctx context.Context, pool *pgxpool.Pool, detail *PuzzleHouseDetail, projectID string, from, to time.Time) error {
	var legacy, corrected int64
	err := pool.QueryRow(ctx, `
SELECT COUNT(*) FILTER (WHERE r.content_revision IS NULL),
       COUNT(*) FILTER (WHERE r.content_revision IS NOT NULL)
FROM events e
LEFT JOIN puzzle_content_revisions r
  ON r.project_id=e.project_id AND r.content_revision=e.properties->>'content_revision'
WHERE e.project_id=$1 AND e.effective_at>=$2 AND e.effective_at<$3
  AND (e.properties->>'city_id')::int=$4 AND (e.properties->>'house_id')::int=$5
  AND e.name='placement_resolved'`, projectID, from, to, detail.CityID, detail.HouseID).Scan(&legacy, &corrected)
	if err != nil {
		return err
	}
	detail.DataQuality.LegacyRevisionEvents = legacy
	detail.DataQuality.CoordinateStatus = coordinateStatus(legacy, corrected)
	return nil
}

func coordinateStatus(legacy, corrected int64) string {
	if legacy == 0 {
		return "trusted"
	}
	if corrected == 0 {
		return "legacy_only"
	}
	return "mixed_coordinate_spaces"
}
