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
	applyPuzzleInsights(detail, events)
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

func metricEvent(name string, properties map[string]any) puzzleMetricEvent {
	raw, _ := json.Marshal(properties)
	return puzzleMetricEvent{Name: name, Properties: raw}
}
