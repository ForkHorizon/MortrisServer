package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GameplayPlayer is an anonymous install, not an account or person. Its ID is
// the SDK-generated install UUID and can be opened through the existing raw
// installation timeline endpoint by project administrators.
type GameplayPlayer struct {
	InstallID             string    `json:"install_id"`
	LastSeenAt            time.Time `json:"last_seen_at"`
	Platform              string    `json:"platform"`
	OSVersion             string    `json:"os_version"`
	DeviceClass           string    `json:"device_class"`
	AppVersion            string    `json:"app_version"`
	BuildNumber           string    `json:"build_number"`
	Locale                string    `json:"locale"`
	TimezoneOffsetMinutes int       `json:"timezone_offset_minutes"`
	DeviceTotalMemoryMB   int64     `json:"device_total_memory_mb"`
	GraphicsMemoryMB      int64     `json:"graphics_memory_mb"`
	LastAllocatedMemoryMB int64     `json:"last_allocated_memory_mb"`
	LastReservedMemoryMB  int64     `json:"last_reserved_memory_mb"`
	LastMonoUsedMemoryMB  int64     `json:"last_mono_used_memory_mb"`
	Attempts              int64     `json:"attempts"`
	Falls                 int64     `json:"falls"`
}

type GameplayPlayersResult struct {
	Players []GameplayPlayer `json:"players"`
}

func GetGameplayPlayers(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time) (*GameplayPlayersResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	rows, err := pool.Query(ctx, `
		SELECT install_id::text,
			MAX(effective_at),
			(array_agg(platform ORDER BY effective_at DESC))[1],
			(array_agg(os_version ORDER BY effective_at DESC))[1],
			(array_agg(device_class ORDER BY effective_at DESC))[1],
			(array_agg(app_version ORDER BY effective_at DESC))[1],
			(array_agg(build_number ORDER BY effective_at DESC))[1],
			(array_agg(locale ORDER BY effective_at DESC))[1],
			(array_agg(timezone_offset_minutes ORDER BY effective_at DESC))[1],
			COALESCE((array_agg((properties->>'device_total_memory_mb')::bigint ORDER BY effective_at DESC) FILTER (WHERE name='device_profile'))[1],0),
			COALESCE((array_agg((properties->>'graphics_memory_mb')::bigint ORDER BY effective_at DESC) FILTER (WHERE name='device_profile'))[1],0),
			COALESCE((array_agg((properties->>'app_allocated_memory_mb')::bigint ORDER BY effective_at DESC) FILTER (WHERE name='memory_sample'))[1],0),
			COALESCE((array_agg((properties->>'app_reserved_memory_mb')::bigint ORDER BY effective_at DESC) FILTER (WHERE name='memory_sample'))[1],0),
			COALESCE((array_agg((properties->>'mono_used_memory_mb')::bigint ORDER BY effective_at DESC) FILTER (WHERE name='memory_sample'))[1],0),
			COUNT(DISTINCT properties->>'attempt_id') FILTER (WHERE properties ? 'attempt_id'),
			COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%')
		FROM events
		WHERE project_id=$1 AND event_kind='product' AND effective_at >= $2 AND effective_at < $3
		GROUP BY install_id
		ORDER BY MAX(effective_at) DESC
		LIMIT 500`, projectID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := &GameplayPlayersResult{Players: []GameplayPlayer{}}
	for rows.Next() {
		var row GameplayPlayer
		if err := rows.Scan(&row.InstallID, &row.LastSeenAt, &row.Platform, &row.OSVersion, &row.DeviceClass, &row.AppVersion, &row.BuildNumber, &row.Locale, &row.TimezoneOffsetMinutes, &row.DeviceTotalMemoryMB, &row.GraphicsMemoryMB, &row.LastAllocatedMemoryMB, &row.LastReservedMemoryMB, &row.LastMonoUsedMemoryMB, &row.Attempts, &row.Falls); err != nil {
			return nil, err
		}
		result.Players = append(result.Players, row)
	}
	return result, rows.Err()
}
