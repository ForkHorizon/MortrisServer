package analytics

import "time"

func (f *MasterFixture) baseProps(extra map[string]any) map[string]any {
	p := map[string]any{
		"schema_version":   2,
		"content_revision": f.Revision,
		"city_id":          f.CityID,
		"house_id":         f.HouseID,
		"coordinate_space": "house_local",
		"origin":           "player",
		"progress_origin":  "natural",
	}
	if extra["attempt_id"] != nil {
		p["wave_index"] = 0
		p["active_elapsed_ms"] = 5000
	}
	for k, v := range extra {
		p[k] = v
	}
	return p
}

func (f *MasterFixture) addEvent(id, name, kind string, seq int64, t time.Time, props map[string]any, sentAt, recvAt time.Time) {
	if sentAt.IsZero() {
		sentAt = t
	}
	if recvAt.IsZero() {
		recvAt = t
	}
	f.Events = append(f.Events, FixtureEvent{
		EventID:      id,
		InstallID:    f.InstallID,
		SessionID:    f.SessionID,
		Sequence:     seq,
		Name:         name,
		Kind:         kind,
		EffectiveAt:  t,
		AppVersion:   f.AppVersion,
		BuildNumber:  f.BuildNumber,
		Platform:     "android",
		Properties:   props,
		SentAtClient: sentAt,
		ReceivedAt:   recvAt,
	})
}

func (f *MasterFixture) buildNaturalScenarios() {
	f.buildRegistrationAndStart()
	f.buildSuccessfulPlacement()
	f.buildNoTargetFall()
	f.buildMissingSupportFall()
	f.buildCheckpointAndBackground()
	f.buildRecoveryAndCompletion()
	f.buildLateDeliveryScenario()
}

func (f *MasterFixture) buildRegistrationAndStart() {
	// Scenario 1: Device profile
	f.addEvent("00000000-0000-4000-8000-000000000001", "device_profile", "product", 1,
		f.BaseTime, f.baseProps(map[string]any{"device_total_memory_mb": 4096, "graphics_memory_mb": 1024}), time.Time{}, time.Time{})

	// Scenario 2: House and wave start
	f.addEvent("00000000-0000-4000-8000-000000000002", "house_run_started", "product", 2,
		f.BaseTime.Add(1*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "house_event_index": 0,
		}), time.Time{}, time.Time{})

	f.addEvent("00000000-0000-4000-8000-000000000003", "wave_attempt_started", "product", 3,
		f.BaseTime.Add(2*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "wave_index": 0,
			"house_event_index": 1, "attempt_event_index": 0,
		}), time.Time{}, time.Time{})
}

func (f *MasterFixture) buildSuccessfulPlacement() {
	// Scenario 3: Successful placement (detail_taken -> detail_released -> placement_resolved:placed)
	f.addEvent("00000000-0000-4000-8000-000000000004", "detail_taken", "product", 4,
		f.BaseTime.Add(3*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "interaction_id": "int-success",
			"block_id": 10, "house_event_index": 2, "attempt_event_index": 1,
		}), time.Time{}, time.Time{})
	f.addEvent("00000000-0000-4000-8000-000000000005", "detail_released", "product", 5,
		f.BaseTime.Add(4*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "interaction_id": "int-success",
			"block_id": 10, "candidate_target_id": 10, "house_event_index": 3, "attempt_event_index": 2,
			"release_x_milli": 1000, "release_y_milli": 2000,
		}), time.Time{}, time.Time{})
	f.addEvent("00000000-0000-4000-8000-000000000006", "placement_resolved", "product", 6,
		f.BaseTime.Add(5*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "interaction_id": "int-success",
			"block_id": 10, "outcome": "placed", "candidate_target_id": 10,
			"release_x_milli": 1000, "release_y_milli": 2000,
			"house_event_index": 4, "attempt_event_index": 3,
		}), time.Time{}, time.Time{})
}

func (f *MasterFixture) buildNoTargetFall() {
	// Scenario 4: No-target fall and inventory return
	f.addEvent("00000000-0000-4000-8000-000000000007", "detail_taken", "product", 7,
		f.BaseTime.Add(6*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "interaction_id": "int-no-target",
			"block_id": 11, "house_event_index": 5, "attempt_event_index": 4,
		}), time.Time{}, time.Time{})
	f.addEvent("00000000-0000-4000-8000-000000000008", "detail_released", "product", 8,
		f.BaseTime.Add(7*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "interaction_id": "int-no-target",
			"block_id": 11, "release_x_milli": 1500, "release_y_milli": 2500,
			"house_event_index": 6, "attempt_event_index": 5,
		}), time.Time{}, time.Time{})
	f.addEvent("00000000-0000-4000-8000-000000000009", "placement_resolved", "product", 9,
		f.BaseTime.Add(8*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "interaction_id": "int-no-target",
			"block_id": 11, "outcome": "fell_no_snap_target", "candidate_target_id": -1,
			"nearest_compatible_target_id": 11,
			"release_x_milli":              1500, "release_y_milli": 2500,
			"house_event_index": 7, "attempt_event_index": 6,
		}), time.Time{}, time.Time{})
	f.addEvent("00000000-0000-4000-8000-000000000010", "detail_returned", "product", 10,
		f.BaseTime.Add(9*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "interaction_id": "int-no-target",
			"block_id": 11, "house_event_index": 8, "attempt_event_index": 7,
		}), time.Time{}, time.Time{})
}

func (f *MasterFixture) buildMissingSupportFall() {
	// Scenario 5: Missing-support fall and inventory return
	f.addEvent("00000000-0000-4000-8000-000000000011", "detail_taken", "product", 11,
		f.BaseTime.Add(10*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "interaction_id": "int-missing-sup",
			"block_id": 12, "house_event_index": 9, "attempt_event_index": 8,
		}), time.Time{}, time.Time{})
	f.addEvent("00000000-0000-4000-8000-000000000012", "detail_released", "product", 12,
		f.BaseTime.Add(11*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "interaction_id": "int-missing-sup",
			"block_id": 12, "release_x_milli": 1200, "release_y_milli": 2200,
			"house_event_index": 10, "attempt_event_index": 9,
		}), time.Time{}, time.Time{})
	f.addEvent("00000000-0000-4000-8000-000000000013", "placement_resolved", "product", 13,
		f.BaseTime.Add(12*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "interaction_id": "int-missing-sup",
			"block_id": 12, "outcome": "fell_missing_support", "candidate_target_id": 12,
			"release_x_milli": 1200, "release_y_milli": 2200,
			"house_event_index": 11, "attempt_event_index": 10,
		}), time.Time{}, time.Time{})
	f.addEvent("00000000-0000-4000-8000-000000000014", "detail_returned", "product", 14,
		f.BaseTime.Add(13*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "interaction_id": "int-missing-sup",
			"block_id": 12, "house_event_index": 12, "attempt_event_index": 11,
		}), time.Time{}, time.Time{})
}

func (f *MasterFixture) buildCheckpointAndBackground() {
	// Scenario 6: State checkpoint with valid hash
	f.addEvent("00000000-0000-4000-8000-000000000015", "state_checkpoint", "product", 15,
		f.BaseTime.Add(14*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "placed_block_ids": "10",
			"placed_state_hash": CheckpointHash("10"), "house_event_index": 13, "attempt_event_index": 12,
		}), time.Time{}, time.Time{})

	// Scenario 7: App background and foreground cycle
	f.addEvent("00000000-0000-4000-8000-000000000016", "app_backgrounded", "product", 16,
		f.BaseTime.Add(15*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID,
			"house_event_index": 14, "attempt_event_index": 13,
		}), time.Time{}, time.Time{})
	f.addEvent("00000000-0000-4000-8000-000000000017", "app_foregrounded", "product", 17,
		f.BaseTime.Add(20*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID,
			"house_event_index": 15, "attempt_event_index": 14,
		}), time.Time{}, time.Time{})
}

func (f *MasterFixture) buildRecoveryAndCompletion() {
	// Scenario 8: Attempt recovery after app restart
	f.addEvent("00000000-0000-4000-8000-000000000018", "attempt_recovered", "product", 18,
		f.BaseTime.Add(25*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID,
			"placed_block_ids": "10", "placed_state_hash": CheckpointHash("10"),
			"house_event_index": 16, "attempt_event_index": 15,
		}), time.Time{}, time.Time{})

	// Scenario 9: Natural house completion
	f.addEvent("00000000-0000-4000-8000-000000000019", "wave_completed", "product", 19,
		f.BaseTime.Add(26*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "wave_index": 0,
			"house_event_index": 17, "attempt_event_index": 16,
		}), time.Time{}, time.Time{})
	f.addEvent("00000000-0000-4000-8000-000000000020", "house_completed", "product", 20,
		f.BaseTime.Add(27*time.Second), f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "house_event_index": 18,
		}), time.Time{}, time.Time{})
}

func (f *MasterFixture) buildLateDeliveryScenario() {
	// Scenario 14: Late out-of-order delivery with valid internal monotonic sequence
	// Client sent at BaseTime.Add(28s), received out-of-order at BaseTime.Add(45s) after S10 commands
	tOccurred := f.BaseTime.Add(28 * time.Second)
	tSent := tOccurred
	tReceived := f.BaseTime.Add(45 * time.Second)

	f.addEvent("00000000-0000-4000-8000-000000000021", "attempt_closed", "product", 21,
		tOccurred, f.baseProps(map[string]any{
			"house_run_id": f.HouseRunID, "attempt_id": f.AttemptID, "close_reason": "completed",
			"house_event_index": 19, "attempt_event_index": 17,
		}), tSent, tReceived)
}
