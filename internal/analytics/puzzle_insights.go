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
	Tries             int
	TakenAt           int64
	HasUnresolvedTake bool
	ActiveBlock       int
	LastFailedBlock   int
}

type puzzleMetricSamples struct {
	tries    map[int][]int
	times    map[int][]int64
	examples map[int][]string
}

func loadPuzzleHouseInsights(ctx context.Context, pool *pgxpool.Pool, detail *PuzzleHouseDetail, projectID string, from, to time.Time, scope TrafficScope, build *string) error {
	events, err := loadPuzzleMetricEvents(ctx, pool, projectID, detail.CityID, detail.HouseID, from, to, scope, build)
	if err != nil {
		return err
	}
	// Retry/time-to-place samples and wave entry counts are attempt-scoped
	// (plan section 6, rules 2-3: "interactions whose whole chain is
	// natural and belongs to one attempt," "include only fully natural
	// house runs"), unlike the per-block fall-rate metrics events feeds,
	// which stay event-level (rule 1). eligible is which attempt_ids
	// qualify at the coarser, whole-attempt granularity.
	eligible, err := loadEligibleAttempts(ctx, pool, projectID, detail.CityID, detail.HouseID, from, to, scope)
	if err != nil {
		return err
	}
	applyPuzzleInsights(detail, events, eligible)
	return loadPuzzleDataQuality(ctx, pool, detail, projectID, from, to, build)
}

func loadPuzzleMetricEvents(ctx context.Context, pool *pgxpool.Pool, projectID string, cityID, houseID int, from, to time.Time, scope TrafficScope, build *string) ([]puzzleMetricEvent, error) {
	rows, err := pool.Query(ctx, `
SELECT name, effective_at, properties FROM events
WHERE project_id=$1 AND effective_at>=$4 AND effective_at<$5
  AND ($6::text IS NULL OR build_number=$6)
  AND (properties->>'city_id')::int=$2 AND (properties->>'house_id')::int=$3
  AND name IN ('detail_taken','placement_resolved','hint_used','interaction_abandoned')
  AND `+scope.eventPredicate()+`
ORDER BY properties->>'attempt_id', CASE WHEN properties->>'attempt_event_index' ~ '^[0-9]+$' THEN (properties->>'attempt_event_index')::int ELSE 2147483647 END, effective_at, event_id`, projectID, cityID, houseID, from, to, build)
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

// loadEligibleAttempts is the attempt-level half of the same scope this
// house's event-level metrics already use — see loadPuzzleHouseInsights.
func loadEligibleAttempts(ctx context.Context, pool *pgxpool.Pool, projectID string, cityID, houseID int, from, to time.Time, scope TrafficScope) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `
SELECT attempt_id FROM puzzle_wave_attempt_classification
WHERE project_id=$1 AND city_id=$2 AND house_id=$3 AND last_event_at>=$4 AND last_event_at<$5 AND `+scope.runPredicate("fully_natural"),
		projectID, cityID, houseID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	eligible := map[string]bool{}
	for rows.Next() {
		var attemptID string
		if err := rows.Scan(&attemptID); err != nil {
			return nil, err
		}
		eligible[attemptID] = true
	}
	return eligible, rows.Err()
}

func applyPuzzleInsights(detail *PuzzleHouseDetail, events []puzzleMetricEvent, eligibleAttempts map[string]bool) {
	blocks := puzzleBlockIndex(detail)
	states := map[string]*puzzleMetricState{}
	waveAttempts := map[int]map[string]bool{}
	waves := map[int]*PuzzleWaveSummary{}
	samples := puzzleMetricSamples{
		tries:    map[int][]int{},
		times:    map[int][]int64{},
		examples: map[int][]string{},
	}
	for _, event := range events {
		applyPuzzleInsightEvent(blocks, states, waves, waveAttempts, samples, event, eligibleAttempts)
	}
	attemptIDs := make([]string, 0, len(states))
	for attID := range states {
		attemptIDs = append(attemptIDs, attID)
	}
	sort.Strings(attemptIDs)
	for _, attID := range attemptIDs {
		flushAttemptChain(blocks, states[attID], samples, eligibleAttempts[attID], attID)
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

func flushAttemptChain(blocks map[int]*PuzzleHouseBlock, state *puzzleMetricState, samples puzzleMetricSamples, attemptEligible bool, attemptID string) {
	if state == nil || state.ActiveBlock == -1 {
		return
	}
	block := blocks[state.ActiveBlock]
	if block != nil && attemptEligible {
		if state.Tries > 0 || state.HasUnresolvedTake {
			block.RetryLadder.NeverSucceeded++
			recordExampleAttempt(samples.examples, block.BlockID, attemptID)
		}
		if state.HasUnresolvedTake {
			block.TimeToPlace.IncompleteInteractions++
			recordExampleAttempt(samples.examples, block.BlockID, attemptID)
		}
	}
	state.Tries = 0
	state.TakenAt = -1
	state.HasUnresolvedTake = false
	state.ActiveBlock = -1
	state.LastFailedBlock = -1
}

func recordExampleAttempt(examples map[int][]string, blockID int, attemptID string) {
	if attemptID == "" {
		return
	}
	list := examples[blockID]
	for _, existing := range list {
		if existing == attemptID {
			return
		}
	}
	if len(list) < 5 {
		examples[blockID] = append(list, attemptID)
	}
}

func puzzleBlockIndex(detail *PuzzleHouseDetail) map[int]*PuzzleHouseBlock {
	result := make(map[int]*PuzzleHouseBlock, len(detail.Blocks))
	for i := range detail.Blocks {
		result[detail.Blocks[i].BlockID] = &detail.Blocks[i]
	}
	return result
}

func applyPuzzleInsightEvent(blocks map[int]*PuzzleHouseBlock, states map[string]*puzzleMetricState, waves map[int]*PuzzleWaveSummary, waveAttempts map[int]map[string]bool, samples puzzleMetricSamples, event puzzleMetricEvent, eligibleAttempts map[string]bool) {
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
	attemptEligible := eligibleAttempts[attemptID]
	if event.Name == "hint_used" {
		applyHintPressure(blocks, state)
		return
	}
	if event.Name == "interaction_abandoned" {
		if state.ActiveBlock != -1 && state.HasUnresolvedTake {
			if b := blocks[state.ActiveBlock]; b != nil && attemptEligible {
				b.TimeToPlace.IncompleteInteractions++
				recordExampleAttempt(samples.examples, b.BlockID, attemptID)
			}
			state.HasUnresolvedTake = false
			state.TakenAt = -1
			state.Tries++
		}
		return
	}
	blockID := number(payload["block_id"])
	block := blocks[blockID]
	if block == nil {
		return
	}
	if event.Name == "detail_taken" {
		if state.ActiveBlock != -1 && state.ActiveBlock != blockID {
			flushAttemptChain(blocks, state, samples, attemptEligible, attemptID)
		} else if state.ActiveBlock == blockID && state.HasUnresolvedTake {
			if attemptEligible {
				block.TimeToPlace.IncompleteInteractions++
				recordExampleAttempt(samples.examples, block.BlockID, attemptID)
			}
			state.Tries++
		}
		state.ActiveBlock = blockID
		state.TakenAt = number64(payload["active_elapsed_ms"])
		state.HasUnresolvedTake = true
		return
	}
	applyPlacementInsight(block, state, waves, waveAttempts, samples, attemptID, payload, attemptEligible, blocks)
}

func applyHintPressure(blocks map[int]*PuzzleHouseBlock, state *puzzleMetricState) {
	if block := blocks[state.LastFailedBlock]; block != nil {
		block.HintPressureCount++
	}
	state.LastFailedBlock = -1
}

// attemptEligible gates the attempt-scoped accumulations (wave entry,
// retry-to-success and time-to-place samples — plan section 6, rules 2-3)
// on top of the event-level scope already applied by the caller's query.
// Per-block fall-rate/no-snap/missing-support/first-try/hint-pressure
// stay ungated: rule 1 keeps those event-level regardless of whether the
// rest of the attempt was touched.
func applyPlacementInsight(block *PuzzleHouseBlock, state *puzzleMetricState, waves map[int]*PuzzleWaveSummary, waveAttempts map[int]map[string]bool, samples puzzleMetricSamples, attemptID string, payload map[string]any, attemptEligible bool, blocks map[int]*PuzzleHouseBlock) {
	if state.ActiveBlock != -1 && state.ActiveBlock != block.BlockID {
		flushAttemptChain(blocks, state, samples, attemptEligible, attemptID)
	}
	if state.ActiveBlock != block.BlockID {
		state.Tries, state.TakenAt, state.HasUnresolvedTake, state.ActiveBlock = 0, -1, false, block.BlockID
	}
	state.Tries++
	outcome, _ := payload["outcome"].(string)
	waveIndex := number(payload["wave_index"])
	wave := waveFor(waves, waveIndex)
	if attemptEligible {
		if waveAttempts[waveIndex] == nil {
			waveAttempts[waveIndex] = map[string]bool{}
		}
		waveAttempts[waveIndex][attemptID] = true
	}
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
		state.HasUnresolvedTake = false
		if attemptEligible {
			recordExampleAttempt(samples.examples, block.BlockID, attemptID)
		}
		return
	}
	block.SuccessfulPlacements++
	if attemptEligible {
		switch state.Tries {
		case 1:
			block.RetryLadder.Success1stTry++
		case 2:
			block.RetryLadder.Success2ndTry++
		case 3:
			block.RetryLadder.Success3rdTry++
		default:
			block.RetryLadder.Success4thPlusTry++
		}
		samples.tries[block.BlockID] = append(samples.tries[block.BlockID], state.Tries)
		elapsed := number64(payload["active_elapsed_ms"])
		if elapsed >= state.TakenAt && state.TakenAt >= 0 {
			samples.times[block.BlockID] = append(samples.times[block.BlockID], elapsed-state.TakenAt)
		}
		recordExampleAttempt(samples.examples, block.BlockID, attemptID)
	}
	state.Tries, state.TakenAt, state.HasUnresolvedTake, state.ActiveBlock, state.LastFailedBlock = 0, -1, false, -1, -1
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
		tries := samples.tries[block.BlockID]
		times := samples.times[block.BlockID]

		ladder := &block.RetryLadder
		ladder.SampleCount = ladder.Success1stTry + ladder.Success2ndTry + ladder.Success3rdTry + ladder.Success4thPlusTry + ladder.NeverSucceeded
		ladder.Reliable = ladder.SampleCount >= minPlacementsForRate
		if len(tries) >= minPlacementsForRate {
			ladder.MedianTries = intPercentile(tries, 50)
			ladder.P75Tries = intPercentile(tries, 75)
			ladder.P90Tries = intPercentile(tries, 90)
		} else {
			ladder.MedianTries = medianInts(tries)
			ladder.P75Tries = 0
			ladder.P90Tries = 0
		}
		if examples, ok := samples.examples[block.BlockID]; ok && len(examples) > 0 {
			ladder.ExampleAttemptIDs = examples
		} else {
			ladder.ExampleAttemptIDs = []string{}
		}

		ttp := &block.TimeToPlace
		ttp.SampleCount = int64(len(times))
		ttp.Reliable = ttp.SampleCount >= minPlacementsForRate
		if ttp.SampleCount >= minPlacementsForRate {
			ttp.MedianMS = int64Percentile(times, 50)
			ttp.P75MS = int64Percentile(times, 75)
			ttp.P90MS = int64Percentile(times, 90)
		} else {
			ttp.MedianMS = int64Median(times)
			ttp.P75MS = 0
			ttp.P90MS = 0
		}

		block.MedianTriesToSuccess = ladder.MedianTries
		block.MedianTimeToPlaceMS = ttp.MedianMS
		block.TimeToPlaceSamples = ttp.SampleCount
	}
}

func intPercentile(values []int, pct int) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	idx := (len(sorted) * pct) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func int64Percentile(values []int64, pct int) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := (len(sorted) * pct) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
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
		result, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return -1
		}
		return result
	}
	return -1
}
