package analytics

import "testing"

func TestComputeQualityAlerts_AllCategories(t *testing.T) {
	q := &PuzzleQuality{
		RejectedEvents:               5,
		UnknownRevisionEvents:        2,
		TotalGameplayEvents:          100,
		SchemaV2Events:               80,
		SchemaV2Share:                0.80,
		MissingSchemaVersion:         10,
		InvalidCoordinateSpaceEvents: 3,
		SequenceGaps:                 2,
		HouseEventIndexGaps:          1,
		OrphanInteractions:           4,
		CheckpointMismatches:         1,
		UnpairedDeveloperCommands:    1,
		DeliveryDelayMS:              PuzzleQualityDelay{Samples: 10, P90MS: 15000},
	}

	build := "b123"
	alerts := ComputeQualityAlerts(q, "proj-1", &build)
	expectedCategories := map[string]bool{
		"rejection_spike": false, "unknown_revision": false,
		"schema_share_drop": false, "missing_schema_version": false,
		"coordinate_mismatch": false, "sequence_integrity": false,
		"index_gap": false, "orphan_interaction": false,
		"checkpoint_mismatch": false, "unpaired_developer_action": false,
		"delivery_delay_anomaly": false,
	}
	assertAlertCategories(t, alerts, expectedCategories)
}

func assertAlertCategories(t *testing.T, alerts []PuzzleQualityAlert, expected map[string]bool) {
	t.Helper()
	for _, a := range alerts {
		if _, ok := expected[a.Category]; ok {
			expected[a.Category] = true
		}
		if a.ActionLink == "" {
			t.Errorf("alert %s missing action_link", a.Category)
		}
		if a.Severity != "critical" && a.Severity != "warning" {
			t.Errorf("alert %s invalid severity %s", a.Category, a.Severity)
		}
	}
	for cat, found := range expected {
		if !found {
			t.Errorf("expected alert category %s to be triggered", cat)
		}
	}
}

func TestComputeQualityAlerts_CleanIsZeroAlerts(t *testing.T) {
	q := &PuzzleQuality{
		TotalGameplayEvents: 100,
		SchemaV2Events:      100,
		SchemaV2Share:       1.0,
		DeliveryDelayMS:     PuzzleQualityDelay{Samples: 10, P90MS: 50},
	}

	alerts := ComputeQualityAlerts(q, "proj-1", nil)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts on clean quality, got %+v", alerts)
	}
}
