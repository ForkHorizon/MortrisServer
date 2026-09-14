package analytics

import "fmt"

// PuzzleQualityAlert represents an actionable production alert surfacing
// specific failure classes instead of a generic "analytics error".
type PuzzleQualityAlert struct {
	Category   string `json:"category"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Metric     string `json:"metric"`
	Value      any    `json:"value"`
	ActionLink string `json:"action_link"`
}

// ComputeQualityAlerts evaluates quality metrics and returns actionable alerts.
func ComputeQualityAlerts(q *PuzzleQuality, projectID string, build *string) []PuzzleQualityAlert {
	alerts := make([]PuzzleQualityAlert, 0)
	scope := buildScopeParam(projectID, build)

	appendRejectionAlerts(&alerts, q, scope)
	appendRevisionAndSchemaAlerts(&alerts, q, scope)
	appendCoordinateAlerts(&alerts, q, scope)
	appendIntegrityAlerts(&alerts, q, scope)
	appendDeveloperAlerts(&alerts, q, scope)
	appendDelayAlerts(&alerts, q, scope)

	return alerts
}

func buildScopeParam(projectID string, build *string) string {
	if build != nil && *build != "" {
		return fmt.Sprintf("project=%s&build_number=%s", projectID, *build)
	}
	return fmt.Sprintf("project=%s", projectID)
}

func appendRejectionAlerts(alerts *[]PuzzleQualityAlert, q *PuzzleQuality, scope string) {
	if q.RejectedEvents > 0 {
		*alerts = append(*alerts, PuzzleQualityAlert{
			Category:   "rejection_spike",
			Severity:   "critical",
			Message:    fmt.Sprintf("Strict catalog rejected %d events in scope", q.RejectedEvents),
			Metric:     "rejected_events",
			Value:      q.RejectedEvents,
			ActionLink: fmt.Sprintf("/events?%s", scope),
		})
	}
}

func appendRevisionAndSchemaAlerts(alerts *[]PuzzleQualityAlert, q *PuzzleQuality, scope string) {
	if q.UnknownRevisionEvents > 0 {
		*alerts = append(*alerts, PuzzleQualityAlert{
			Category:   "unknown_revision",
			Severity:   "critical",
			Message:    fmt.Sprintf("%d events reference an unknown/unimported content revision", q.UnknownRevisionEvents),
			Metric:     "unknown_revision_events",
			Value:      q.UnknownRevisionEvents,
			ActionLink: fmt.Sprintf("/catalog?%s", scope),
		})
	}
	if q.TotalGameplayEvents > 0 && q.SchemaV2Share < 0.95 {
		*alerts = append(*alerts, PuzzleQualityAlert{
			Category:   "schema_share_drop",
			Severity:   "warning",
			Message:    fmt.Sprintf("Schema-v2 adoption is %.1f%%, below expected 95%% threshold", q.SchemaV2Share*100),
			Metric:     "schema_v2_share",
			Value:      q.SchemaV2Share,
			ActionLink: fmt.Sprintf("/houses?%s", scope),
		})
	}
	if q.MissingSchemaVersion > 0 {
		*alerts = append(*alerts, PuzzleQualityAlert{
			Category:   "missing_schema_version",
			Severity:   "warning",
			Message:    fmt.Sprintf("%d gameplay events omit schema_version", q.MissingSchemaVersion),
			Metric:     "missing_schema_version_events",
			Value:      q.MissingSchemaVersion,
			ActionLink: fmt.Sprintf("/events?%s", scope),
		})
	}
}

func appendCoordinateAlerts(alerts *[]PuzzleQualityAlert, q *PuzzleQuality, scope string) {
	if q.InvalidCoordinateSpaceEvents > 0 {
		*alerts = append(*alerts, PuzzleQualityAlert{
			Category:   "coordinate_mismatch",
			Severity:   "critical",
			Message:    fmt.Sprintf("%d placement/release events are outside house_local coordinates", q.InvalidCoordinateSpaceEvents),
			Metric:     "invalid_coordinate_space_events",
			Value:      q.InvalidCoordinateSpaceEvents,
			ActionLink: fmt.Sprintf("/events?%s&name=placement_resolved", scope),
		})
	}
}

func appendIntegrityAlerts(alerts *[]PuzzleQualityAlert, q *PuzzleQuality, scope string) {
	if q.SequenceGaps > 0 || q.SequenceDuplicates > 0 {
		*alerts = append(*alerts, PuzzleQualityAlert{
			Category:   "sequence_integrity",
			Severity:   "critical",
			Message:    fmt.Sprintf("%d SDK sequence gaps/duplicates detected", q.SequenceGaps+q.SequenceDuplicates),
			Metric:     "sequence_gaps",
			Value:      q.SequenceGaps + q.SequenceDuplicates,
			ActionLink: fmt.Sprintf("/gameplay?%s", scope),
		})
	}
	if q.HouseEventIndexGaps > 0 || q.AttemptEventIndexGaps > 0 || q.HouseEventIndexDuplicates > 0 || q.AttemptEventIndexDuplicates > 0 {
		total := q.HouseEventIndexGaps + q.AttemptEventIndexGaps + q.HouseEventIndexDuplicates + q.AttemptEventIndexDuplicates
		*alerts = append(*alerts, PuzzleQualityAlert{
			Category:   "index_gap",
			Severity:   "critical",
			Message:    fmt.Sprintf("%d event index gaps/duplicates detected in house/attempt lifecycle", total),
			Metric:     "event_index_gaps",
			Value:      total,
			ActionLink: fmt.Sprintf("/gameplay?%s", scope),
		})
	}
	if q.OrphanInteractions > 0 {
		*alerts = append(*alerts, PuzzleQualityAlert{
			Category:   "orphan_interaction",
			Severity:   "critical",
			Message:    fmt.Sprintf("%d interaction chains never reached terminal placement or return", q.OrphanInteractions),
			Metric:     "orphan_interactions",
			Value:      q.OrphanInteractions,
			ActionLink: fmt.Sprintf("/houses?%s", scope),
		})
	}
	if q.CheckpointMismatches > 0 {
		*alerts = append(*alerts, PuzzleQualityAlert{
			Category:   "checkpoint_mismatch",
			Severity:   "critical",
			Message:    fmt.Sprintf("%d state checkpoint hash mismatches detected", q.CheckpointMismatches),
			Metric:     "checkpoint_mismatches",
			Value:      q.CheckpointMismatches,
			ActionLink: fmt.Sprintf("/events?%s&name=state_checkpoint", scope),
		})
	}
}

func appendDeveloperAlerts(alerts *[]PuzzleQualityAlert, q *PuzzleQuality, scope string) {
	if q.UnpairedDeveloperCommands > 0 || q.UnpairedDeveloperMutations > 0 {
		*alerts = append(*alerts, PuzzleQualityAlert{
			Category:   "unpaired_developer_action",
			Severity:   "critical",
			Message:    fmt.Sprintf("%d unpaired developer commands/mutations detected", q.UnpairedDeveloperCommands+q.UnpairedDeveloperMutations),
			Metric:     "unpaired_developer_actions",
			Value:      q.UnpairedDeveloperCommands + q.UnpairedDeveloperMutations,
			ActionLink: fmt.Sprintf("/gameplay?%s", scope),
		})
	}
}

func appendDelayAlerts(alerts *[]PuzzleQualityAlert, q *PuzzleQuality, scope string) {
	if q.DeliveryDelayMS.Samples > 0 && q.DeliveryDelayMS.P90MS > 10000 {
		*alerts = append(*alerts, PuzzleQualityAlert{
			Category:   "delivery_delay_anomaly",
			Severity:   "warning",
			Message:    fmt.Sprintf("P90 delivery delay is %.1f ms (>10s threshold)", q.DeliveryDelayMS.P90MS),
			Metric:     "delivery_delay_p90_ms",
			Value:      q.DeliveryDelayMS.P90MS,
			ActionLink: fmt.Sprintf("/gameplay?%s", scope),
		})
	}
}
