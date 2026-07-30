package analytics

import (
	"context"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DimensionsResult struct {
	Platforms   []string `json:"platforms"`
	AppVersions []string `json:"app_versions"`
}

// GetDimensions returns the distinct platform and app_version values
// observed in the last 90 days, for filter dropdowns: both fields are a
// tiny, known value set per project rather than free text worth
// typo-driven empty results. The 90-day floor reuses idx_events_project_effective
// instead of a new index just for this.
func GetDimensions(ctx context.Context, pool *pgxpool.Pool, projectID string) (*DimensionsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := pool.Query(ctx, `
		SELECT DISTINCT platform, app_version
		FROM events
		WHERE project_id = $1
		  AND effective_at > now() - interval '90 days'
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	platforms := map[string]struct{}{}
	appVersions := map[string]struct{}{}
	for rows.Next() {
		var platform, appVersion string
		if err := rows.Scan(&platform, &appVersion); err != nil {
			return nil, err
		}
		platforms[platform] = struct{}{}
		appVersions[appVersion] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := &DimensionsResult{Platforms: sortedKeys(platforms), AppVersions: sortedKeys(appVersions)}
	return result, nil
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
