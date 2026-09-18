package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MemoryTimelinePoint represents a single memory observation or lifecycle marker.
type MemoryTimelinePoint struct {
	EventID        string    `json:"event_id"`
	EffectiveAt    time.Time `json:"effective_at"`
	ActiveTimeMS   int64     `json:"active_time_ms"`
	WallTimeMS     int64     `json:"wall_time_ms"`
	EventName      string    `json:"event_name"`
	MarkerType     string    `json:"marker_type"`
	MarkerLabel    string    `json:"marker_label"`
	AppAllocatedMB *int64    `json:"app_allocated_mb,omitempty"`
	AppReservedMB  *int64    `json:"app_reserved_mb,omitempty"`
	MonoUsedMB     *int64    `json:"mono_used_mb,omitempty"`
	CityID         *int      `json:"city_id,omitempty"`
	HouseID        *int      `json:"house_id,omitempty"`
	WaveIndex      *int      `json:"wave_index,omitempty"`
	AttemptID      *string   `json:"attempt_id,omitempty"`
	BuildNumber    string    `json:"build_number"`
	AppVersion     string    `json:"app_version"`
}

// PuzzleMemoryTimeline contains the full memory and lifecycle progression.
type PuzzleMemoryTimeline struct {
	InstallID           string                `json:"install_id"`
	DeviceTotalMemoryMB int64                 `json:"device_total_memory_mb"`
	GraphicsMemoryMB    int64                 `json:"graphics_memory_mb"`
	TotalActiveTimeMS   int64                 `json:"total_active_time_ms"`
	TotalWallTimeMS     int64                 `json:"total_wall_time_ms"`
	SampleCount         int64                 `json:"sample_count"`
	Points              []MemoryTimelinePoint `json:"points"`
}

type rawTimelineRow struct {
	EventID         string
	EffectiveAt     time.Time
	Name            string
	Platform        string
	OSVersion       string
	DeviceClass     string
	AppVersion      string
	BuildNumber     string
	AllocatedMB     *int64
	ReservedMB      *int64
	MonoUsedMB      *int64
	TotalMemoryMB   *int64
	GraphicsMB      *int64
	CityID          *int
	HouseID         *int
	WaveIndex       *int
	AttemptID       *string
	ActiveElapsedMS *int64
}

// classifyMarker determines the semantic marker type and label for a lifecycle event.
func classifyMarker(name, prevBuild, currentBuild string, waveIndex *int, houseID *int) (string, string) {
	if prevBuild != "" && currentBuild != "" && prevBuild != currentBuild {
		return "build_update", fmt.Sprintf("Build updated to %s", currentBuild)
	}
	switch name {
	case "memory_sample":
		return "memory_sample", "Memory sample"
	case "house_run_started", "house_opened":
		h := 0
		if houseID != nil {
			h = *houseID
		}
		return "house_start", fmt.Sprintf("House %d started", h)
	case "wave_attempt_started", "wave_presented":
		w := 0
		if waveIndex != nil {
			w = *waveIndex
		}
		return "wave_start", fmt.Sprintf("Wave %d started", w)
	case "state_checkpoint":
		return "checkpoint", "State checkpoint"
	case "app_backgrounded":
		return "background", "App backgrounded"
	case "app_foregrounded":
		return "foreground", "App foregrounded"
	case "attempt_recovered":
		return "recovery", "Attempt recovered after restart"
	default:
		return "event", name
	}
}

// timelineState tracks progressive active and wall time across events.
type timelineState struct {
	firstAt        time.Time
	lastAt         time.Time
	cumActiveMS    int64
	isBackgrounded bool
	lastBuild      string
}

// updateActiveTime computes the progressive active time without background gaps.
func (s *timelineState) updateActiveTime(now time.Time) int64 {
	if s.firstAt.IsZero() {
		s.firstAt = now
		s.lastAt = now
		return 0
	}
	wallDelta := now.Sub(s.lastAt).Milliseconds()
	if wallDelta < 0 {
		wallDelta = 0
	}
	if !s.isBackgrounded {
		// Cap unexplained long foreground gaps at 30 minutes
		if wallDelta > pauseGapCapMS {
			wallDelta = pauseGapCapMS
		}
		s.cumActiveMS += wallDelta
	}
	s.lastAt = now
	return s.cumActiveMS
}

// createTimelinePoint maps a raw row and tracking state into a timeline point.
func createTimelinePoint(r rawTimelineRow, state *timelineState) MemoryTimelinePoint {
	activeMS := state.updateActiveTime(r.EffectiveAt)
	wallMS := r.EffectiveAt.Sub(state.firstAt).Milliseconds()
	if wallMS < 0 {
		wallMS = 0
	}

	mType, mLabel := classifyMarker(r.Name, state.lastBuild, r.BuildNumber, r.WaveIndex, r.HouseID)
	if r.BuildNumber != "" {
		state.lastBuild = r.BuildNumber
	}
	switch r.Name {
	case "app_backgrounded":
		state.isBackgrounded = true
	case "app_foregrounded":
		state.isBackgrounded = false
	}

	return MemoryTimelinePoint{
		EventID:        r.EventID,
		EffectiveAt:    r.EffectiveAt,
		ActiveTimeMS:   activeMS,
		WallTimeMS:     wallMS,
		EventName:      r.Name,
		MarkerType:     mType,
		MarkerLabel:    mLabel,
		AppAllocatedMB: r.AllocatedMB,
		AppReservedMB:  r.ReservedMB,
		MonoUsedMB:     r.MonoUsedMB,
		CityID:         r.CityID,
		HouseID:        r.HouseID,
		WaveIndex:      r.WaveIndex,
		AttemptID:      r.AttemptID,
		BuildNumber:    r.BuildNumber,
		AppVersion:     r.AppVersion,
	}
}

// buildTimelinePoints processes raw rows into sequential timeline points.
func buildTimelinePoints(rows []rawTimelineRow) ([]MemoryTimelinePoint, int64, int64, int64) {
	points := make([]MemoryTimelinePoint, 0, len(rows))
	state := &timelineState{}
	var totalMemoryMB, graphicsMB, sampleCount int64

	for _, r := range rows {
		if r.TotalMemoryMB != nil && *r.TotalMemoryMB > totalMemoryMB {
			totalMemoryMB = *r.TotalMemoryMB
		}
		if r.GraphicsMB != nil && *r.GraphicsMB > graphicsMB {
			graphicsMB = *r.GraphicsMB
		}
		if r.Name == "memory_sample" {
			sampleCount++
		}
		points = append(points, createTimelinePoint(r, state))
	}

	return points, totalMemoryMB, graphicsMB, sampleCount
}

// memoryTimelineSQL is the SQL query for querying timeline events.
const memoryTimelineSQL = `
SELECT event_id::text, effective_at, name,
       platform, os_version, device_class, app_version, build_number,
       CASE WHEN properties->>'app_allocated_memory_mb' ~ '^[0-9]{1,9}$' THEN (properties->>'app_allocated_memory_mb')::bigint ELSE NULL END,
       CASE WHEN properties->>'app_reserved_memory_mb' ~ '^[0-9]{1,9}$' THEN (properties->>'app_reserved_memory_mb')::bigint ELSE NULL END,
       CASE WHEN properties->>'mono_used_memory_mb' ~ '^[0-9]{1,9}$' THEN (properties->>'mono_used_memory_mb')::bigint ELSE NULL END,
       CASE WHEN properties->>'device_total_memory_mb' ~ '^[0-9]{1,9}$' THEN (properties->>'device_total_memory_mb')::bigint ELSE NULL END,
       CASE WHEN properties->>'graphics_memory_mb' ~ '^[0-9]{1,9}$' THEN (properties->>'graphics_memory_mb')::bigint ELSE NULL END,
       CASE WHEN properties->>'city_id' ~ '^[0-9]{1,9}$' THEN (properties->>'city_id')::int ELSE NULL END,
       CASE WHEN properties->>'house_id' ~ '^[0-9]{1,9}$' THEN (properties->>'house_id')::int ELSE NULL END,
       CASE WHEN properties->>'wave_index' ~ '^[0-9]{1,9}$' THEN (properties->>'wave_index')::int ELSE NULL END,
       properties->>'attempt_id',
       CASE WHEN properties->>'active_elapsed_ms' ~ '^[0-9]{1,9}$' THEN (properties->>'active_elapsed_ms')::bigint ELSE NULL END
FROM events
WHERE project_id=$1 AND install_id=$2::uuid
  AND (
    name IN ('memory_sample', 'device_profile', 'house_run_started', 'house_opened',
             'wave_attempt_started', 'wave_presented', 'state_checkpoint',
             'app_backgrounded', 'app_foregrounded', 'attempt_recovered')
  )
ORDER BY effective_at ASC, sequence ASC
LIMIT 1000`

// GetPuzzleMemoryTimeline queries and reconstructs the memory progression
// and interleaved lifecycle markers for an anonymous install.
func GetPuzzleMemoryTimeline(ctx context.Context, pool *pgxpool.Pool, projectID, installID string) (*PuzzleMemoryTimeline, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := pool.Query(ctx, memoryTimelineSQL, projectID, installID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rawRows := []rawTimelineRow{}
	for rows.Next() {
		var r rawTimelineRow
		if err := rows.Scan(
			&r.EventID, &r.EffectiveAt, &r.Name,
			&r.Platform, &r.OSVersion, &r.DeviceClass, &r.AppVersion, &r.BuildNumber,
			&r.AllocatedMB, &r.ReservedMB, &r.MonoUsedMB,
			&r.TotalMemoryMB, &r.GraphicsMB,
			&r.CityID, &r.HouseID, &r.WaveIndex,
			&r.AttemptID, &r.ActiveElapsedMS,
		); err != nil {
			return nil, err
		}
		rawRows = append(rawRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	points, totalRAM, graphicsRAM, sampleCount := buildTimelinePoints(rawRows)
	var totalActiveMS, totalWallMS int64
	if len(points) > 0 {
		totalActiveMS = points[len(points)-1].ActiveTimeMS
		totalWallMS = points[len(points)-1].WallTimeMS
	}

	return &PuzzleMemoryTimeline{
		InstallID:           installID,
		DeviceTotalMemoryMB: totalRAM,
		GraphicsMemoryMB:    graphicsRAM,
		TotalActiveTimeMS:   totalActiveMS,
		TotalWallTimeMS:     totalWallMS,
		SampleCount:         sampleCount,
		Points:              points,
	}, nil
}
