package analytics

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PuzzleWaveSummary tells the reader where progress starts to break down.
type PuzzleWaveSummary struct {
	WaveIndex  int     `json:"wave_index"`
	Attempts   int64   `json:"attempts"`
	Placements int64   `json:"placements"`
	Falls      int64   `json:"falls"`
	FallRate   float64 `json:"fall_rate"`
}

// PuzzleDataQuality keeps legacy data visible without letting it look as
// trustworthy as corrected client telemetry.
type PuzzleDataQuality struct {
	LegacyRevisionEvents int64  `json:"legacy_revision_events"`
	CoordinateStatus     string `json:"coordinate_status"`
}

type puzzleMetricEvent struct {
	Name        string
	EffectiveAt time.Time
	Properties  json.RawMessage
}

type puzzleMetricState struct {
	Tries           int
	TakenAt         int64
	ActiveBlock     int
	LastFailedBlock int
}

type puzzleMetricSamples struct {
	tries map[int][]int
	times map[int][]int64
}

func loadPuzzleHouseInsights(ctx context.Context, pool *pgxpool.Pool, detail *PuzzleHouseDetail, projectID string, from, to time.Time) error {
	events, err := loadPuzzleMetricEvents(ctx, pool, projectID, detail.CityID, detail.HouseID, from, to)
	if err != nil {
		return err
	}
	applyPuzzleInsights(detail, events)
	return loadPuzzleDataQuality(ctx, pool, detail, projectID, from, to)
}

func loadPuzzleMetricEvents(ctx context.Context, pool *pgxpool.Pool, projectID string, cityID, houseID int, from, to time.Time) ([]puzzleMetricEvent, error) {
	rows, err := pool.Query(ctx, `
SELECT name, effective_at, properties FROM events
WHERE project_id=$1 AND effective_at>=$4 AND effective_at<$5
  AND (properties->>'city_id')::int=$2 AND (properties->>'house_id')::int=$3
  AND name IN ('detail_taken','placement_resolved','hint_used')
  AND COALESCE(properties->>'origin','player')='player'
  AND COALESCE(properties->>'progress_origin','natural')='natural'
ORDER BY properties->>'attempt_id', CASE WHEN properties->>'attempt_event_index' ~ '^[0-9]+$' THEN (properties->>'attempt_event_index')::int ELSE 2147483647 END, effective_at, event_id`, projectID, cityID, houseID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []puzzleMetricEvent{}
	for rows.Next() {
		var event puzzleMetricEvent
		if err := rows.Scan(&event.Name, &event.EffectiveAt, &event.Properties); err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}

func applyPuzzleInsights(detail *PuzzleHouseDetail, events []puzzleMetricEvent) {
	blocks := puzzleBlockIndex(detail)
	states := map[string]*puzzleMetricState{}
	waveAttempts := map[int]map[string]bool{}
	waves := map[int]*PuzzleWaveSummary{}
	samples := puzzleMetricSamples{tries: map[int][]int{}, times: map[int][]int64{}}
	for _, event := range events {
		applyPuzzleInsightEvent(blocks, states, waves, waveAttempts, samples, event)
	}
	for i := range detail.Blocks {
		finalizePuzzleBlockInsight(&detail.Blocks[i])
	}
	applyPuzzleMedians(detail, samples)
	for index, attempts := range waveAttempts {
		waveFor(waves, index).Attempts = int64(len(attempts))
	}
	detail.Waves = sortedPuzzleWaves(waves)
}

func puzzleBlockIndex(detail *PuzzleHouseDetail) map[int]*PuzzleHouseBlock {
	result := make(map[int]*PuzzleHouseBlock, len(detail.Blocks))
	for i := range detail.Blocks {
		result[detail.Blocks[i].BlockID] = &detail.Blocks[i]
	}
	return result
}

func applyPuzzleInsightEvent(blocks map[int]*PuzzleHouseBlock, states map[string]*puzzleMetricState, waves map[int]*PuzzleWaveSummary, waveAttempts map[int]map[string]bool, samples puzzleMetricSamples, event puzzleMetricEvent) {
	payload, ok := attemptEventPayload(event.Properties)
	if !ok {
		return
	}
	attemptID, _ := payload["attempt_id"].(string)
	if attemptID == "" {
		return
	}
	state := states[attemptID]
	if state == nil {
		state = &puzzleMetricState{TakenAt: -1, ActiveBlock: -1, LastFailedBlock: -1}
		states[attemptID] = state
	}
	if event.Name == "hint_used" {
		applyHintPressure(blocks, state)
		return
	}
	blockID := number(payload["block_id"])
	block := blocks[blockID]
	if block == nil {
		return
	}
	if event.Name == "detail_taken" {
		blockID := number(payload["block_id"])
		if state.ActiveBlock != blockID {
			state.Tries, state.LastFailedBlock = 0, -1
		}
		state.ActiveBlock = blockID
		state.TakenAt = number64(payload["active_elapsed_ms"])
		return
	}
	applyPlacementInsight(block, state, waves, waveAttempts, samples, attemptID, payload)
}

func applyHintPressure(blocks map[int]*PuzzleHouseBlock, state *puzzleMetricState) {
	if block := blocks[state.LastFailedBlock]; block != nil {
		block.HintPressureCount++
	}
	state.LastFailedBlock = -1
}

func applyPlacementInsight(block *PuzzleHouseBlock, state *puzzleMetricState, waves map[int]*PuzzleWaveSummary, waveAttempts map[int]map[string]bool, samples puzzleMetricSamples, attemptID string, payload map[string]any) {
	if state.ActiveBlock != block.BlockID {
		state.Tries, state.TakenAt, state.ActiveBlock = 0, -1, block.BlockID
	}
	state.Tries++
	outcome, _ := payload["outcome"].(string)
	waveIndex := number(payload["wave_index"])
	wave := waveFor(waves, waveIndex)
	if waveAttempts[waveIndex] == nil {
		waveAttempts[waveIndex] = map[string]bool{}
	}
	waveAttempts[waveIndex][attemptID] = true
	wave.Placements++
	if state.Tries == 1 {
		block.FirstTryAttempts++
		if outcome != "placed" {
			block.FirstTryFailures++
		}
	}
	if outcome == "fell_no_snap_target" {
		block.NoSnapRate += 1
	}
	if outcome == "fell_missing_support" {
		block.MissingSupportRate += 1
	}
	if len(outcome) > 5 && outcome[:5] == "fell_" {
		wave.Falls++
		state.LastFailedBlock = block.BlockID
	}
	if outcome != "placed" {
		return
	}
	block.SuccessfulPlacements++
	samples.tries[block.BlockID] = append(samples.tries[block.BlockID], state.Tries)
	if elapsed := number64(payload["active_elapsed_ms"]); elapsed >= state.TakenAt && state.TakenAt >= 0 {
		block.TimeToPlaceSamples++
		samples.times[block.BlockID] = append(samples.times[block.BlockID], elapsed-state.TakenAt)
	}
	state.Tries, state.TakenAt, state.ActiveBlock, state.LastFailedBlock = 0, -1, -1, -1
}

func waveFor(waves map[int]*PuzzleWaveSummary, index int) *PuzzleWaveSummary {
	if waves[index] == nil {
		waves[index] = &PuzzleWaveSummary{WaveIndex: index}
	}
	return waves[index]
}

func finalizePuzzleBlockInsight(block *PuzzleHouseBlock) {
	if block.FirstTryAttempts > 0 {
		block.FirstTryFailureRate = float64(block.FirstTryFailures) / float64(block.FirstTryAttempts)
	}
	block.FirstTryReliable = block.FirstTryAttempts >= minPlacementsForRate
	if block.Placements > 0 {
		block.NoSnapRate /= float64(block.Placements)
		block.MissingSupportRate /= float64(block.Placements)
		block.HintPressureRate = float64(block.HintPressureCount) / float64(block.Placements)
	}
}

func applyPuzzleMedians(detail *PuzzleHouseDetail, samples puzzleMetricSamples) {
	for i := range detail.Blocks {
		block := &detail.Blocks[i]
		block.MedianTriesToSuccess = medianInts(samples.tries[block.BlockID])
		block.MedianTimeToPlaceMS = int64Median(samples.times[block.BlockID])
	}
}

func sortedPuzzleWaves(waves map[int]*PuzzleWaveSummary) []PuzzleWaveSummary {
	result := make([]PuzzleWaveSummary, 0, len(waves))
	for _, wave := range waves {
		if wave.Placements > 0 {
			wave.FallRate = float64(wave.Falls) / float64(wave.Placements)
		}
		result = append(result, *wave)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].WaveIndex < result[j].WaveIndex })
	return result
}

func medianInts(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	return sorted[len(sorted)/2]
}

func int64Median(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/2]
}

func number64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case string:
		result, _ := strconv.ParseInt(typed, 10, 64)
		return result
	}
	return -1
}
