package analytics

import "time"

func (f *MasterFixture) buildDeveloperScenarios() {
	f.buildDeveloperCompletion()
	f.buildDeveloperReset()
	f.buildDeveloperFailAndNoop()
}

func (f *MasterFixture) buildDeveloperCompletion() {
	// Scenario 10: Developer completion command with progress mutation
	t := f.BaseTime.Add(40 * time.Second)
	actionID := "act-dev-comp"
	devRunID := "run-dev-cheated-01"

	f.buildDeveloperCompletionCommand(t, actionID, devRunID)
	f.buildDeveloperCheatedRun(t.Add(3*time.Second), devRunID)
}

func (f *MasterFixture) devActionBaseProps(actionID, command string) map[string]any {
	return map[string]any{
		"schema_version":      2,
		"content_revision":    f.Revision,
		"city_id":             f.CityID,
		"house_id":            f.HouseID,
		"coordinate_space":    "house_local",
		"origin":              "developer_menu",
		"progress_origin":     "developer_modified",
		"developer_action_id": actionID,
		"developer_command":   command,
	}
}

func (f *MasterFixture) buildDeveloperCompletionCommand(t time.Time, actionID, devRunID string) {
	devProps := f.devActionBaseProps(actionID, "CompleteHouse99")
	f.addEvent("00000000-0000-4000-8000-000000000030", "developer_command_started", "product", 22,
		t, devProps, time.Time{}, time.Time{})

	mutProps := f.devActionBaseProps(actionID, "CompleteHouse99")
	mutProps["before_state_hash"] = CheckpointHash("10")
	mutProps["after_state_hash"] = CheckpointHash("10,11,12")
	mutProps["before_completed"] = false
	mutProps["after_completed"] = true
	mutProps["house_run_id"] = devRunID
	f.addEvent("00000000-0000-4000-8000-000000000031", "developer_progress_mutated", "product", 23,
		t.Add(time.Second), mutProps, time.Time{}, time.Time{})

	compProps := f.devActionBaseProps(actionID, "CompleteHouse99")
	compProps["developer_result"] = "success"
	compProps["progress_changed"] = true
	f.addEvent("00000000-0000-4000-8000-000000000032", "developer_command_completed", "product", 24,
		t.Add(2*time.Second), compProps, time.Time{}, time.Time{})
}

func (f *MasterFixture) buildDeveloperCheatedRun(t time.Time, devRunID string) {
	runCompProps := map[string]any{
		"schema_version":    2,
		"content_revision":  f.Revision,
		"city_id":           f.CityID,
		"house_id":          f.HouseID,
		"coordinate_space":  "house_local",
		"origin":            "player",
		"progress_origin":   "developer_modified",
		"house_run_id":      devRunID,
		"house_event_index": 0,
	}
	f.addEvent("00000000-0000-4000-8000-000000000033", "house_completed", "product", 25,
		t, runCompProps, time.Time{}, time.Time{})
}

func (f *MasterFixture) buildDeveloperReset() {
	// Scenario 11: Developer reset command (ResetHouse)
	t := f.BaseTime.Add(50 * time.Second)
	actionID := "act-dev-reset"

	devProps := f.devActionBaseProps(actionID, "ResetHouse")
	devProps["progress_origin"] = "developer_reset"
	f.addEvent("00000000-0000-4000-8000-000000000034", "developer_command_started", "product", 26,
		t, devProps, time.Time{}, time.Time{})

	mutProps := f.devActionBaseProps(actionID, "ResetHouse")
	mutProps["progress_origin"] = "developer_reset"
	mutProps["before_state_hash"] = CheckpointHash("10,11,12")
	mutProps["after_state_hash"] = CheckpointHash("")
	mutProps["before_completed"] = true
	mutProps["after_completed"] = false
	f.addEvent("00000000-0000-4000-8000-000000000035", "developer_progress_mutated", "product", 27,
		t.Add(time.Second), mutProps, time.Time{}, time.Time{})

	compProps := f.devActionBaseProps(actionID, "ResetHouse")
	compProps["progress_origin"] = "developer_reset"
	compProps["developer_result"] = "success"
	compProps["progress_changed"] = true
	f.addEvent("00000000-0000-4000-8000-000000000036", "developer_command_completed", "product", 28,
		t.Add(2*time.Second), compProps, time.Time{}, time.Time{})
}

func (f *MasterFixture) buildDeveloperFailAndNoop() {
	// Scenario 12: Failed and no-op developer commands
	tFail := f.BaseTime.Add(60 * time.Second)
	failID := "act-dev-fail"
	f.addEvent("00000000-0000-4000-8000-000000000037", "developer_command_started", "product", 29,
		tFail, map[string]any{
			"schema_version": 2, "content_revision": f.Revision, "developer_action_id": failID,
			"developer_command": "InvalidCommand", "origin": "developer_menu", "progress_origin": "natural",
		}, time.Time{}, time.Time{})
	f.addEvent("00000000-0000-4000-8000-000000000038", "developer_command_failed", "product", 30,
		tFail.Add(time.Second), map[string]any{
			"schema_version": 2, "content_revision": f.Revision, "developer_action_id": failID,
			"developer_command": "InvalidCommand", "developer_result": "failed", "origin": "developer_menu", "progress_origin": "natural",
		}, time.Time{}, time.Time{})

	tNoop := f.BaseTime.Add(65 * time.Second)
	noopID := "act-dev-noop"
	f.addEvent("00000000-0000-4000-8000-000000000039", "developer_command_started", "product", 31,
		tNoop, map[string]any{
			"schema_version": 2, "content_revision": f.Revision, "developer_action_id": noopID,
			"developer_command": "NoopCheck", "origin": "developer_menu", "progress_origin": "natural",
		}, time.Time{}, time.Time{})
	f.addEvent("00000000-0000-4000-8000-000000000040", "developer_command_noop", "product", 32,
		tNoop.Add(time.Second), map[string]any{
			"schema_version": 2, "content_revision": f.Revision, "developer_action_id": noopID,
			"developer_command": "NoopCheck", "developer_result": "noop", "origin": "developer_menu", "progress_origin": "natural",
		}, time.Time{}, time.Time{})
}

func (f *MasterFixture) buildRetryAndCatalogScenarios() {
	// Baseline clean ingestion stats
	f.Stats = append(f.Stats, FixtureIngestionStat{
		InstallID:  f.InstallID,
		Accepted:   int64(len(f.Events)),
		Duplicates: 0,
		Rejected:   0,
		ReceivedAt: f.BaseTime.Add(30 * time.Second),
	})
}

// AddNetworkRetryScenario adds duplicate event stats (Scenario 13).
func (f *MasterFixture) AddNetworkRetryScenario() {
	if len(f.Stats) > 0 {
		f.Stats[0].Duplicates = 2
	}
}

// AddStrictCatalogRejectionScenario adds a rejected event (Scenario 15).
func (f *MasterFixture) AddStrictCatalogRejectionScenario() {
	rejectionTime := f.BaseTime.Add(70 * time.Second)
	f.Stats = append(f.Stats, FixtureIngestionStat{
		InstallID:  f.InstallID,
		Accepted:   0,
		Duplicates: 0,
		Rejected:   1,
		ReceivedAt: rejectionTime,
	})
	f.Rejections = append(f.Rejections, FixtureRejection{
		InstallID:  f.InstallID,
		Name:       "placement_resolved",
		Code:       "undeclared_property",
		ReceivedAt: rejectionTime,
	})
}
