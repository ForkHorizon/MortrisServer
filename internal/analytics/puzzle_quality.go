package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// loadPuzzleDataQuality and coordinateStatus predate this file's
// PuzzleQuality control plane and keep their narrower, per-house-detail
// and per-overview scope: legacy-revision visibility inline with a
// single house or the house wall, not the full Stage 2 range report.
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
  AND e.name='placement_resolved'
  AND COALESCE(e.properties->>'origin','player')='player'
  AND COALESCE(e.properties->>'progress_origin','natural')='natural'`, projectID, from, to, detail.CityID, detail.HouseID).Scan(&legacy, &corrected)
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

// addIf appends a formatted reason to *reasons when cond is true — every
// PuzzleQuality status reason is built this way so a number is never
// shown without the sentence explaining it.
func addIf(reasons *[]string, cond bool, format string, args ...any) {
	if cond {
		*reasons = append(*reasons, fmt.Sprintf(format, args...))
	}
}

// PuzzleQuality is Stage 2's data-quality control plane (docs/puzzle-analytics-remaining-plan.md
// section 5): everything a designer needs to decide whether the selected
// date range is safe to draw a difficulty conclusion from, surfaced
// before any house ranking is trusted. build, when non-empty, scopes
// every per-event count except Builds itself, which always reports every
// build in range so the dashboard can offer it as a filter.
type PuzzleQuality struct {
	ReceivedEvents  int64                     `json:"received_events"`
	AcceptedEvents  int64                     `json:"accepted_events"`
	DuplicateEvents int64                     `json:"duplicate_events"`
	RejectedEvents  int64                     `json:"rejected_events"`
	Rejections      []PuzzleQualityRejection  `json:"rejections"`

	TotalGameplayEvents  int64 `json:"total_gameplay_events"`
	SchemaV2Events       int64 `json:"schema_v2_events"`
	MissingSchemaVersion int64 `json:"missing_schema_version_events"`

	UnknownRevisionEvents        int64 `json:"unknown_revision_events"`
	InvalidCoordinateSpaceEvents int64 `json:"invalid_coordinate_space_events"`

	DistinctInstalls     int64 `json:"distinct_installs"`
	DistinctSessions     int64 `json:"distinct_sessions"`
	DistinctHouseRuns    int64 `json:"distinct_house_runs"`
	DistinctWaveAttempts int64 `json:"distinct_wave_attempts"`
	DistinctInteractions int64 `json:"distinct_interactions"`

	SequenceGaps       int64 `json:"sequence_gaps"`
	SequenceDuplicates int64 `json:"sequence_duplicates"`

	HouseEventIndexGaps       int64 `json:"house_event_index_gaps"`
	HouseEventIndexDuplicates int64 `json:"house_event_index_duplicates"`

	AttemptEventIndexGaps       int64 `json:"attempt_event_index_gaps"`
	AttemptEventIndexDuplicates int64 `json:"attempt_event_index_duplicates"`

	OrphanInteractions   int64 `json:"orphan_interactions"`
	CheckpointMismatches int64 `json:"checkpoint_mismatches"`
	RecoveredAttempts    int64 `json:"recovered_attempts"`

	// Recent means the run/attempt's last event is within lostThresholdHours
	// (puzzle_diagnostics.go) — reused rather than a second definition, per
	// the plan doc. Stale means older than that with no terminal event.
	OpenHouseRunsRecent int64 `json:"open_house_runs_recent"`
	OpenHouseRunsStale  int64 `json:"open_house_runs_stale"`
	OpenAttemptsRecent  int64 `json:"open_attempts_recent"`
	OpenAttemptsStale   int64 `json:"open_attempts_stale"`

	UnpairedDeveloperCommands  int64 `json:"unpaired_developer_commands"`
	UnpairedDeveloperMutations int64 `json:"unpaired_developer_mutations"`

	DeliveryDelayMS PuzzleQualityDelay `json:"delivery_delay_ms"`

	Builds []PuzzleQualityBuild `json:"builds"`

	// Status is "grey" (no data), "green", "amber" or "red" per the plan
	// doc's exact definitions; Reasons is the plain-language drill-down so
	// a number is never shown without an explanation.
	Status  string   `json:"status"`
	Reasons []string `json:"status_reasons"`
}

type PuzzleQualityRejection struct {
	Name  string `json:"name"`
	Code  string `json:"code"`
	Count int64  `json:"count"`
}

type PuzzleQualityDelay struct {
	Samples int64   `json:"samples"`
	P50MS   float64 `json:"p50_ms"`
	P90MS   float64 `json:"p90_ms"`
	P99MS   float64 `json:"p99_ms"`
	MaxMS   float64 `json:"max_ms"`
}

type PuzzleQualityBuild struct {
	AppVersion  string `json:"app_version"`
	BuildNumber string `json:"build_number"`
	Events      int64  `json:"events"`
}

func GetPuzzleQuality(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, build *string) (*PuzzleQuality, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	q := &PuzzleQuality{Rejections: []PuzzleQualityRejection{}, Builds: []PuzzleQualityBuild{}, Reasons: []string{}}
	// Builds is always unfiltered: it is what lets the dashboard offer the
	// filter in the first place.
	if err := loadQualityBuilds(ctx, pool, q, projectID, from, to); err != nil {
		return nil, err
	}
	if err := loadQualityIngestion(ctx, pool, q, projectID, from, to); err != nil {
		return nil, err
	}
	if err := loadQualityRejections(ctx, pool, q, projectID, from, to); err != nil {
		return nil, err
	}
	if err := loadQualitySchemaVersion(ctx, pool, q, projectID, from, to, build); err != nil {
		return nil, err
	}
	if err := loadQualityRevisionAndCoordinateSpace(ctx, pool, q, projectID, from, to, build); err != nil {
		return nil, err
	}
	if err := loadQualityDistinctCounts(ctx, pool, q, projectID, from, to, build); err != nil {
		return nil, err
	}
	if err := loadQualityIndexIntegrity(ctx, pool, q, projectID, from, to, build); err != nil {
		return nil, err
	}
	if err := loadQualityOrphanInteractions(ctx, pool, q, projectID, from, to, build); err != nil {
		return nil, err
	}
	if err := loadQualityCheckpointMismatches(ctx, pool, q, projectID, from, to, build); err != nil {
		return nil, err
	}
	if err := loadQualityRecoveredAndOpenRuns(ctx, pool, q, projectID, from, to, build); err != nil {
		return nil, err
	}
	if err := loadQualityDeveloperPairing(ctx, pool, q, projectID, from, to, build); err != nil {
		return nil, err
	}
	if err := loadQualityDeliveryDelay(ctx, pool, q, projectID, from, to, build); err != nil {
		return nil, err
	}
	q.computeStatus()
	return q, nil
}

// computeStatus applies the plan doc's exact green/amber/red rules
// (section 5, "Required dashboard work"). Grey means no data — never
// green, since a green light on zero events would look like a verdict.
func (q *PuzzleQuality) computeStatus() {
	if q.TotalGameplayEvents == 0 {
		q.Status, q.Reasons = "grey", []string{"no gameplay events in range"}
		return
	}
	var red []string
	addIf(&red, q.RejectedEvents > 0, "%d rejected events", q.RejectedEvents)
	addIf(&red, q.UnknownRevisionEvents > 0, "%d events with an unrecognized content revision", q.UnknownRevisionEvents)
	addIf(&red, q.InvalidCoordinateSpaceEvents > 0, "%d placement events outside house_local coordinates", q.InvalidCoordinateSpaceEvents)
	addIf(&red, q.SequenceGaps > 0 || q.SequenceDuplicates > 0, "%d SDK sequence gaps/duplicates", q.SequenceGaps+q.SequenceDuplicates)
	addIf(&red, q.HouseEventIndexGaps > 0 || q.HouseEventIndexDuplicates > 0, "%d house_event_index gaps/duplicates", q.HouseEventIndexGaps+q.HouseEventIndexDuplicates)
	addIf(&red, q.AttemptEventIndexGaps > 0 || q.AttemptEventIndexDuplicates > 0, "%d attempt_event_index gaps/duplicates", q.AttemptEventIndexGaps+q.AttemptEventIndexDuplicates)
	addIf(&red, q.OrphanInteractions > 0, "%d interaction chains never reached a terminal event", q.OrphanInteractions)
	addIf(&red, q.CheckpointMismatches > 0, "%d state checkpoint hash mismatches", q.CheckpointMismatches)
	addIf(&red, q.UnpairedDeveloperCommands > 0, "%d developer commands with no terminal result", q.UnpairedDeveloperCommands)
	if len(red) > 0 {
		q.Status, q.Reasons = "red", red
		return
	}
	var amber []string
	addIf(&amber, q.TotalGameplayEvents < minPlacementsForRate, "only %d gameplay events in range, too little to judge", q.TotalGameplayEvents)
	addIf(&amber, q.OpenHouseRunsRecent > 0, "%d house runs still open and recent", q.OpenHouseRunsRecent)
	addIf(&amber, q.OpenAttemptsRecent > 0, "%d wave attempts still open and recent", q.OpenAttemptsRecent)
	addIf(&amber, q.MissingSchemaVersion > 0, "%d events missing schema_version", q.MissingSchemaVersion)
	if len(amber) > 0 {
		q.Status, q.Reasons = "amber", amber
		return
	}
	q.Status = "green"
}
