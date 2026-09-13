package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TenMinutesActivePlayMS is the 10-minute active foreground play threshold
// required before the client emits memory_sample telemetry.
const TenMinutesActivePlayMS int64 = 10 * 60 * 1000

// MemorySampleStatus describes sample availability for an anonymous install.
type MemorySampleStatus string

const (
	MemoryStatusNotExpected MemorySampleStatus = "not_expected"
	MemoryStatusAvailable   MemorySampleStatus = "available"
	MemoryStatusAbsent      MemorySampleStatus = "absent"
)

// PuzzleDeviceSummary represents exactly one anonymous install. Multiple
// scene reloads emitting device_profile must resolve to a single entry.
type PuzzleDeviceSummary struct {
	InstallID               string             `json:"install_id"`
	LastSeenAt              time.Time          `json:"last_seen_at"`
	Platform                string             `json:"platform"`
	OSVersion               string             `json:"os_version"`
	DeviceClass             string             `json:"device_class"`
	AppVersion              string             `json:"app_version"`
	BuildNumber             string             `json:"build_number"`
	Locale                  string             `json:"locale"`
	TimezoneOffsetMinutes   int                `json:"timezone_offset_minutes"`
	DeviceTotalMemoryMB     int64              `json:"device_total_memory_mb"`
	GraphicsMemoryMB        int64              `json:"graphics_memory_mb"`
	NaturalRunsCount        int64              `json:"natural_runs_count"`
	CompletedRunsCount      int64              `json:"completed_runs_count"`
	FallsCount              int64              `json:"falls_count"`
	PlacementsCount         int64              `json:"placements_count"`
	ActivePlayMS            int64              `json:"active_play_ms"`
	MemorySampleCount       int64              `json:"memory_sample_count"`
	MemorySampleExpected    bool               `json:"memory_sample_expected"`
	MemorySampleStatus      MemorySampleStatus `json:"memory_sample_status"`
	MemorySampleStatusLabel string             `json:"memory_sample_status_label"`
	LastAllocatedMemoryMB   int64              `json:"last_allocated_memory_mb"`
	LastReservedMemoryMB    int64              `json:"last_reserved_memory_mb"`
	LastMonoUsedMemoryMB    int64              `json:"last_mono_used_memory_mb"`
}

// PuzzleDeviceListResult wraps the device summaries and cohort comparison.
type PuzzleDeviceListResult struct {
	Devices []PuzzleDeviceSummary `json:"devices"`
	Cohorts []MemoryCohort        `json:"cohorts"`
}

// evaluateMemorySampleStatus checks whether active time satisfies the 10-minute
// threshold contract before classifying samples as expected or absent.
func evaluateMemorySampleStatus(sampleCount int64, activePlayMS int64) (bool, MemorySampleStatus, string) {
	if sampleCount > 0 {
		return true, MemoryStatusAvailable, "Recorded"
	}
	if activePlayMS < TenMinutesActivePlayMS {
		return false, MemoryStatusNotExpected, "Memory sample not expected yet (<10m active play)"
	}
	return true, MemoryStatusAbsent, "Expected (>=10m active play) but not recorded"
}

// deviceDiagnosticsSQL is the SQL query for device diagnostics.
const deviceDiagnosticsSQL = `
WITH active_installs AS (
    SELECT install_id,
           MAX(effective_at) AS last_seen_at,
           (array_agg(platform ORDER BY effective_at DESC))[1] AS platform,
           (array_agg(os_version ORDER BY effective_at DESC))[1] AS os_version,
           (array_agg(device_class ORDER BY effective_at DESC))[1] AS device_class,
           (array_agg(app_version ORDER BY effective_at DESC))[1] AS app_version,
           (array_agg(build_number ORDER BY effective_at DESC))[1] AS build_number,
           (array_agg(locale ORDER BY effective_at DESC))[1] AS locale,
           (array_agg(timezone_offset_minutes ORDER BY effective_at DESC))[1] AS timezone_offset_minutes,
           COUNT(*) FILTER (WHERE name='placement_resolved' AND COALESCE(properties->>'origin','player')='player' AND COALESCE(properties->>'progress_origin','natural')='natural') AS placements_count,
           COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%' AND COALESCE(properties->>'origin','player')='player' AND COALESCE(properties->>'progress_origin','natural')='natural') AS falls_count,
           COUNT(*) FILTER (WHERE name='memory_sample') AS memory_sample_count,
           COALESCE((array_agg(CASE WHEN properties->>'app_allocated_memory_mb' ~ '^[0-9]{1,9}$' THEN (properties->>'app_allocated_memory_mb')::bigint ELSE NULL END ORDER BY effective_at DESC) FILTER (WHERE name='memory_sample'))[1], 0) AS last_allocated_memory_mb,
           COALESCE((array_agg(CASE WHEN properties->>'app_reserved_memory_mb' ~ '^[0-9]{1,9}$' THEN (properties->>'app_reserved_memory_mb')::bigint ELSE NULL END ORDER BY effective_at DESC) FILTER (WHERE name='memory_sample'))[1], 0) AS last_reserved_memory_mb,
           COALESCE((array_agg(CASE WHEN properties->>'mono_used_memory_mb' ~ '^[0-9]{1,9}$' THEN (properties->>'mono_used_memory_mb')::bigint ELSE NULL END ORDER BY effective_at DESC) FILTER (WHERE name='memory_sample'))[1], 0) AS last_mono_used_memory_mb
    FROM events
    WHERE project_id=$1 AND event_kind='product' AND effective_at >= $2 AND effective_at < $3
      AND ($4::text IS NULL OR build_number=$4)
    GROUP BY install_id
),
latest_profiles AS (
    SELECT DISTINCT ON (e.install_id)
        e.install_id,
        CASE WHEN e.properties->>'device_total_memory_mb' ~ '^[0-9]{1,9}$' THEN (e.properties->>'device_total_memory_mb')::bigint ELSE 0 END AS device_total_memory_mb,
        CASE WHEN e.properties->>'graphics_memory_mb' ~ '^[0-9]{1,9}$' THEN (e.properties->>'graphics_memory_mb')::bigint ELSE 0 END AS graphics_memory_mb
    FROM events e
    INNER JOIN active_installs ai ON ai.install_id = e.install_id
    WHERE e.project_id=$1 AND e.name='device_profile'
    ORDER BY e.install_id, e.effective_at DESC
),
run_stats AS (
    SELECT r.install_id,
           COUNT(*) AS natural_runs_count,
           COUNT(*) FILTER (WHERE r.completed) AS completed_runs_count
    FROM puzzle_house_run_classification r
    INNER JOIN active_installs ai ON ai.install_id = r.install_id
    WHERE r.project_id=$1 AND r.first_event_at >= $2 AND r.last_event_at < $3 AND r.fully_natural
    GROUP BY r.install_id
),
attempt_active AS (
    SELECT install_id, properties->>'attempt_id' AS attempt_id,
           COALESCE(MAX(CASE WHEN properties->>'active_elapsed_ms' ~ '^[0-9]{1,9}$' THEN (properties->>'active_elapsed_ms')::bigint ELSE 0 END), 0) AS attempt_active_ms
    FROM events
    WHERE project_id=$1 AND effective_at >= $2 AND effective_at < $3 AND properties ? 'attempt_id'
      AND ($4::text IS NULL OR build_number=$4)
    GROUP BY install_id, properties->>'attempt_id'
),
install_active AS (
    SELECT install_id, SUM(attempt_active_ms) AS total_active_ms
    FROM attempt_active
    GROUP BY install_id
)
SELECT ai.install_id::text, ai.last_seen_at, ai.platform, ai.os_version, ai.device_class,
       ai.app_version, ai.build_number, ai.locale, ai.timezone_offset_minutes,
       COALESCE(lp.device_total_memory_mb, 0), COALESCE(lp.graphics_memory_mb, 0),
       COALESCE(rs.natural_runs_count, 0), COALESCE(rs.completed_runs_count, 0),
       ai.falls_count, ai.placements_count,
       COALESCE(ia.total_active_ms, 0), ai.memory_sample_count,
       ai.last_allocated_memory_mb, ai.last_reserved_memory_mb, ai.last_mono_used_memory_mb
FROM active_installs ai
LEFT JOIN latest_profiles lp ON lp.install_id = ai.install_id
LEFT JOIN run_stats rs ON rs.install_id = ai.install_id
LEFT JOIN install_active ia ON ia.install_id = ai.install_id
ORDER BY ai.last_seen_at DESC
LIMIT 500`

// GetPuzzleDevices queries anonymous device profiles and performance metrics.
func GetPuzzleDevices(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, buildNumber *string) (*PuzzleDeviceListResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := pool.Query(ctx, deviceDiagnosticsSQL, projectID, from, to, buildNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := []PuzzleDeviceSummary{}
	for rows.Next() {
		var d PuzzleDeviceSummary
		if err := rows.Scan(
			&d.InstallID, &d.LastSeenAt, &d.Platform, &d.OSVersion, &d.DeviceClass,
			&d.AppVersion, &d.BuildNumber, &d.Locale, &d.TimezoneOffsetMinutes,
			&d.DeviceTotalMemoryMB, &d.GraphicsMemoryMB,
			&d.NaturalRunsCount, &d.CompletedRunsCount,
			&d.FallsCount, &d.PlacementsCount,
			&d.ActivePlayMS, &d.MemorySampleCount,
			&d.LastAllocatedMemoryMB, &d.LastReservedMemoryMB, &d.LastMonoUsedMemoryMB,
		); err != nil {
			return nil, err
		}
		d.MemorySampleExpected, d.MemorySampleStatus, d.MemorySampleStatusLabel =
			evaluateMemorySampleStatus(d.MemorySampleCount, d.ActivePlayMS)
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	cohorts := BuildMemoryCohorts(devices)
	return &PuzzleDeviceListResult{
		Devices: devices,
		Cohorts: cohorts,
	}, nil
}
