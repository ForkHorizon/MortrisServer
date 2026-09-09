package analytics

import (
	"encoding/json"
	"testing"
)

func TestPuzzleInsightsAttributeRetriesTimeAndHints(t *testing.T) {
	detail := &PuzzleHouseDetail{Blocks: []PuzzleHouseBlock{{BlockID: 7, Placements: 2}}}
	events := []puzzleMetricEvent{
		metricEvent("detail_taken", map[string]any{"attempt_id": "a", "block_id": 7, "active_elapsed_ms": 100}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 7, "wave_index": 1, "outcome": "fell_missing_support", "active_elapsed_ms": 200}),
		metricEvent("hint_used", map[string]any{"attempt_id": "a"}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 7, "wave_index": 1, "outcome": "placed", "active_elapsed_ms": 500}),
	}
	applyPuzzleInsights(detail, events, map[string]bool{"a": true})
	block := detail.Blocks[0]
	if block.FirstTryFailures != 1 || block.HintPressureCount != 1 {
		t.Fatalf("first failures=%d hints=%d", block.FirstTryFailures, block.HintPressureCount)
	}
	if block.MedianTriesToSuccess != 2 || block.MedianTimeToPlaceMS != 400 {
		t.Fatalf("tries=%d time=%d", block.MedianTriesToSuccess, block.MedianTimeToPlaceMS)
	}
	if block.MissingSupportRate != 0.5 {
		t.Fatalf("support rate=%v", block.MissingSupportRate)
	}
	if len(detail.Waves) != 1 || detail.Waves[0].Falls != 1 || detail.Waves[0].Attempts != 1 {
		t.Fatalf("waves=%+v", detail.Waves)
	}
}

// Retry-to-success/time-to-place samples and wave entry counts are
// attempt-scoped (plan section 6, rules 2-3); per-block fall-rate/first-
// try/missing-support stay event-level (rule 1) regardless of whether the
// attempt as a whole is in scope.
func TestPuzzleInsightsGatesAttemptScopedMetricsOnEligibility(t *testing.T) {
	detail := &PuzzleHouseDetail{Blocks: []PuzzleHouseBlock{{BlockID: 7, Placements: 2}}}
	events := []puzzleMetricEvent{
		metricEvent("detail_taken", map[string]any{"attempt_id": "a", "block_id": 7, "active_elapsed_ms": 100}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 7, "wave_index": 1, "outcome": "fell_missing_support", "active_elapsed_ms": 200}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 7, "wave_index": 1, "outcome": "placed", "active_elapsed_ms": 500}),
	}
	applyPuzzleInsights(detail, events, map[string]bool{}) // attempt "a" is not eligible
	block := detail.Blocks[0]
	if block.FirstTryFailures != 1 || block.MissingSupportRate != 0.5 {
		t.Fatalf("event-level metrics must stay populated even for an ineligible attempt: failures=%d support=%v", block.FirstTryFailures, block.MissingSupportRate)
	}
	if block.MedianTriesToSuccess != 0 || block.MedianTimeToPlaceMS != 0 {
		t.Fatalf("retry/time samples must be excluded for an ineligible attempt, got tries=%d time=%d", block.MedianTriesToSuccess, block.MedianTimeToPlaceMS)
	}
	if len(detail.Waves) != 1 || detail.Waves[0].Attempts != 0 {
		t.Fatalf("wave entry must not count an ineligible attempt: %+v", detail.Waves)
	}
}

func metricEvent(name string, properties map[string]any) puzzleMetricEvent {
	raw, _ := json.Marshal(properties)
	return puzzleMetricEvent{Name: name, Properties: raw}
}

func TestPuzzleInsightsRetryLadderChainResetOnBlockSwitch(t *testing.T) {
	detail := &PuzzleHouseDetail{
		Blocks: []PuzzleHouseBlock{
			{BlockID: 7, Placements: 3},
			{BlockID: 8, Placements: 1},
		},
	}
	events := []puzzleMetricEvent{
		// Detail 7: Try 1 fails, Try 2 fails
		metricEvent("detail_taken", map[string]any{"attempt_id": "a", "block_id": 7, "active_elapsed_ms": 100}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 7, "wave_index": 0, "outcome": "fell_missing_support", "active_elapsed_ms": 200}),
		metricEvent("detail_taken", map[string]any{"attempt_id": "a", "block_id": 7, "active_elapsed_ms": 250}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 7, "wave_index": 0, "outcome": "fell_no_snap_target", "active_elapsed_ms": 350}),

		// Switch to Detail 8: Detail 7 chain must reset here!
		metricEvent("detail_taken", map[string]any{"attempt_id": "a", "block_id": 8, "active_elapsed_ms": 400}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 8, "wave_index": 0, "outcome": "placed", "active_elapsed_ms": 500}),

		// Switch back to Detail 7: Starts a brand new chain, succeeds on 1st try of this chain
		metricEvent("detail_taken", map[string]any{"attempt_id": "a", "block_id": 7, "active_elapsed_ms": 600}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 7, "wave_index": 0, "outcome": "placed", "active_elapsed_ms": 700}),
	}
	applyPuzzleInsights(detail, events, map[string]bool{"a": true})

	b7 := detail.Blocks[0]
	if b7.RetryLadder.NeverSucceeded != 1 {
		t.Fatalf("expected 1 never-succeeded chain for block 7 after switch, got %d", b7.RetryLadder.NeverSucceeded)
	}
	if b7.RetryLadder.Success1stTry != 1 {
		t.Fatalf("expected 1 success_1st_try for block 7 on second chain, got %d", b7.RetryLadder.Success1stTry)
	}
	if b7.RetryLadder.Success3rdTry != 0 || b7.RetryLadder.Success2ndTry != 0 {
		t.Fatalf("retries must NOT group across block switches: 2nd=%d 3rd=%d", b7.RetryLadder.Success2ndTry, b7.RetryLadder.Success3rdTry)
	}
	if b7.RetryLadder.SampleCount != 2 {
		t.Fatalf("expected sample count 2 for block 7, got %d", b7.RetryLadder.SampleCount)
	}

	b8 := detail.Blocks[1]
	if b8.RetryLadder.Success1stTry != 1 || b8.RetryLadder.NeverSucceeded != 0 {
		t.Fatalf("block 8 should have 1 success_1st_try and 0 never_succeeded, got %+v", b8.RetryLadder)
	}
}

func TestPuzzleInsightsConsecutiveSameBlockRetries(t *testing.T) {
	detail := &PuzzleHouseDetail{
		Blocks: []PuzzleHouseBlock{{BlockID: 7, Placements: 3}},
	}
	events := []puzzleMetricEvent{
		metricEvent("detail_taken", map[string]any{"attempt_id": "a", "block_id": 7, "active_elapsed_ms": 100}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 7, "wave_index": 0, "outcome": "fell_missing_support", "active_elapsed_ms": 200}),
		metricEvent("detail_taken", map[string]any{"attempt_id": "a", "block_id": 7, "active_elapsed_ms": 300}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 7, "wave_index": 0, "outcome": "fell_missing_support", "active_elapsed_ms": 400}),
		metricEvent("detail_taken", map[string]any{"attempt_id": "a", "block_id": 7, "active_elapsed_ms": 500}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 7, "wave_index": 0, "outcome": "placed", "active_elapsed_ms": 650}),
	}
	applyPuzzleInsights(detail, events, map[string]bool{"a": true})

	b := detail.Blocks[0]
	if b.RetryLadder.Success3rdTry != 1 {
		t.Fatalf("expected success on 3rd try, got %+v", b.RetryLadder)
	}
	if b.RetryLadder.NeverSucceeded != 0 || b.RetryLadder.Success1stTry != 0 || b.RetryLadder.Success2ndTry != 0 {
		t.Fatalf("unexpected retry distribution: %+v", b.RetryLadder)
	}
	if b.RetryLadder.MedianTries != 3 {
		t.Fatalf("expected median tries 3, got %d", b.RetryLadder.MedianTries)
	}
}

func TestPuzzleInsightsActiveElapsedMsExcludesBackground(t *testing.T) {
	detail := &PuzzleHouseDetail{
		Blocks: []PuzzleHouseBlock{{BlockID: 7, Placements: 1}},
	}
	// Player picks block at active_elapsed_ms=1000.
	// Wall clock advances 2 hours in background, but active_elapsed_ms only advances during foreground work to 1500.
	events := []puzzleMetricEvent{
		metricEvent("detail_taken", map[string]any{"attempt_id": "a", "block_id": 7, "active_elapsed_ms": 1000}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 7, "wave_index": 0, "outcome": "placed", "active_elapsed_ms": 1500}),
	}
	applyPuzzleInsights(detail, events, map[string]bool{"a": true})

	b := detail.Blocks[0]
	if b.TimeToPlace.SampleCount != 1 {
		t.Fatalf("expected 1 time-to-place sample, got %d", b.TimeToPlace.SampleCount)
	}
	if b.TimeToPlace.MedianMS != 500 {
		t.Fatalf("expected active elapsed 500ms, got %dms", b.TimeToPlace.MedianMS)
	}
}

func TestPuzzleInsightsIncompleteAndAbandonedInteractions(t *testing.T) {
	detail := &PuzzleHouseDetail{
		Blocks: []PuzzleHouseBlock{
			{BlockID: 7, Placements: 0},
			{BlockID: 8, Placements: 0},
			{BlockID: 9, Placements: 0},
		},
	}
	events := []puzzleMetricEvent{
		// Block 7: Taken but never resolved (attempt ended/closed)
		metricEvent("detail_taken", map[string]any{"attempt_id": "a", "block_id": 7, "active_elapsed_ms": 100}),

		// Block 8: Taken but explicitly abandoned
		metricEvent("detail_taken", map[string]any{"attempt_id": "b", "block_id": 8, "active_elapsed_ms": 200}),
		metricEvent("interaction_abandoned", map[string]any{"attempt_id": "b", "block_id": 8}),

		// Block 9: Taken, then player switched to Block 7 without resolving Block 9
		metricEvent("detail_taken", map[string]any{"attempt_id": "c", "block_id": 9, "active_elapsed_ms": 300}),
		metricEvent("detail_taken", map[string]any{"attempt_id": "c", "block_id": 7, "active_elapsed_ms": 400}),
	}
	applyPuzzleInsights(detail, events, map[string]bool{"a": true, "b": true, "c": true})

	for _, b := range detail.Blocks {
		if b.TimeToPlace.SampleCount != 0 {
			t.Fatalf("block %d should have 0 completed placement samples, got %d", b.BlockID, b.TimeToPlace.SampleCount)
		}
		if b.TimeToPlace.IncompleteInteractions == 0 {
			t.Fatalf("block %d should have incomplete interactions recorded, got 0", b.BlockID)
		}
		if b.RetryLadder.NeverSucceeded == 0 {
			t.Fatalf("block %d should have never_succeeded recorded, got 0", b.BlockID)
		}
	}
}

func TestPuzzleInsightsSampleThresholdConfidence(t *testing.T) {
	detailUnder := &PuzzleHouseDetail{
		Blocks: []PuzzleHouseBlock{{BlockID: 7}},
	}
	// 4 samples (< 5): should NOT be reliable and percentiles should be 0
	var eventsUnder []puzzleMetricEvent
	for i := 1; i <= 4; i++ {
		att := string(rune('a' + i))
		eventsUnder = append(eventsUnder,
			metricEvent("detail_taken", map[string]any{"attempt_id": att, "block_id": 7, "active_elapsed_ms": int64(i * 1000)}),
			metricEvent("placement_resolved", map[string]any{"attempt_id": att, "block_id": 7, "wave_index": 0, "outcome": "placed", "active_elapsed_ms": int64(i*1000 + i*100)}),
		)
	}
	eligibleUnder := map[string]bool{"b": true, "c": true, "d": true, "e": true}
	applyPuzzleInsights(detailUnder, eventsUnder, eligibleUnder)

	bUnder := detailUnder.Blocks[0]
	if bUnder.RetryLadder.Reliable || bUnder.TimeToPlace.Reliable {
		t.Fatalf("4 samples must NOT be declared reliable: retry=%v time=%v", bUnder.RetryLadder.Reliable, bUnder.TimeToPlace.Reliable)
	}
	if bUnder.RetryLadder.P75Tries != 0 || bUnder.RetryLadder.P90Tries != 0 {
		t.Fatalf("unreliable retry ladder must have 0 for p75/p90: p75=%d p90=%d", bUnder.RetryLadder.P75Tries, bUnder.RetryLadder.P90Tries)
	}
	if bUnder.TimeToPlace.P75MS != 0 || bUnder.TimeToPlace.P90MS != 0 {
		t.Fatalf("unreliable time to place must have 0 for p75/p90: p75=%d p90=%d", bUnder.TimeToPlace.P75MS, bUnder.TimeToPlace.P90MS)
	}

	// 5 samples (>= 5): SHOULD be reliable and have p75/p90 computed
	detailOver := &PuzzleHouseDetail{
		Blocks: []PuzzleHouseBlock{{BlockID: 7}},
	}
	var eventsOver []puzzleMetricEvent
	eligibleOver := map[string]bool{}
	for i := 1; i <= 5; i++ {
		att := string(rune('a' + i))
		eligibleOver[att] = true
		eventsOver = append(eventsOver,
			metricEvent("detail_taken", map[string]any{"attempt_id": att, "block_id": 7, "active_elapsed_ms": int64(i * 1000)}),
			metricEvent("placement_resolved", map[string]any{"attempt_id": att, "block_id": 7, "wave_index": 0, "outcome": "placed", "active_elapsed_ms": int64(i*1000 + i*100)}),
		)
	}
	applyPuzzleInsights(detailOver, eventsOver, eligibleOver)

	bOver := detailOver.Blocks[0]
	if !bOver.RetryLadder.Reliable || !bOver.TimeToPlace.Reliable {
		t.Fatalf("5 samples MUST be declared reliable: retry=%v time=%v", bOver.RetryLadder.Reliable, bOver.TimeToPlace.Reliable)
	}
	if bOver.RetryLadder.P75Tries == 0 || bOver.RetryLadder.P90Tries == 0 {
		t.Fatalf("reliable retry ladder must have p75/p90: p75=%d p90=%d", bOver.RetryLadder.P75Tries, bOver.RetryLadder.P90Tries)
	}
	if bOver.TimeToPlace.P75MS == 0 || bOver.TimeToPlace.P90MS == 0 {
		t.Fatalf("reliable time to place must have p75/p90: p75=%d p90=%d", bOver.TimeToPlace.P75MS, bOver.TimeToPlace.P90MS)
	}
}

func TestPuzzleInsightsAuditEdgeCases(t *testing.T) {
	// 1. number64 error handling: invalid strings shouldn't count as 0ms placements
	detail := &PuzzleHouseDetail{
		Blocks: []PuzzleHouseBlock{{BlockID: 7}},
	}
	events := []puzzleMetricEvent{
		// Attempt a: detail_taken has valid active_elapsed_ms, placement_resolved has invalid string
		metricEvent("detail_taken", map[string]any{"attempt_id": "a", "block_id": 7, "active_elapsed_ms": "1000"}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "a", "block_id": 7, "wave_index": 0, "outcome": "placed", "active_elapsed_ms": "not-a-number"}),

		// Attempt b: Consecutive detail_taken on same block without placement_resolved in between
		metricEvent("detail_taken", map[string]any{"attempt_id": "b", "block_id": 7, "active_elapsed_ms": 2000}),
		metricEvent("detail_taken", map[string]any{"attempt_id": "b", "block_id": 7, "active_elapsed_ms": 3000}),
		metricEvent("placement_resolved", map[string]any{"attempt_id": "b", "block_id": 7, "wave_index": 0, "outcome": "placed", "active_elapsed_ms": 4000}),
	}
	applyPuzzleInsights(detail, events, map[string]bool{"a": true, "b": true})

	b := detail.Blocks[0]
	// Attempt a should not contribute a valid placement time (sample count should be 1 from attempt b)
	if b.TimeToPlace.SampleCount != 1 {
		t.Fatalf("expected 1 valid sample duration, got %d", b.TimeToPlace.SampleCount)
	}
	if b.TimeToPlace.MedianMS != 1000 { // 4000 - 3000 = 1000ms
		t.Fatalf("expected 1000ms median placement duration, got %d", b.TimeToPlace.MedianMS)
	}

	// Attempt b had 2 consecutive takes before placement, so it should land in 2nd try bucket
	if b.RetryLadder.Success2ndTry != 1 {
		t.Fatalf("expected attempt b to succeed on 2nd try due to consecutive detail_taken, got %d", b.RetryLadder.Success2ndTry)
	}
	if b.RetryLadder.Success1stTry != 1 {
		// Attempt a had 1 try and placed (even though timing string was invalid)
		t.Fatalf("expected attempt a to succeed on 1st try, got %d", b.RetryLadder.Success1stTry)
	}
}
