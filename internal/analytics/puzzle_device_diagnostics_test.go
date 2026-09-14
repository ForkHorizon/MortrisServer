package analytics

import (
	"context"
	"regexp"
	"testing"
	"time"
)

func TestEvaluateMemorySampleStatus(t *testing.T) {
	// Case 1: Short session (< 10 minutes active play, no samples)
	// Must say "Memory sample not expected yet (<10m active play)", NEVER an error or missing data.
	expected, status, label := evaluateMemorySampleStatus(0, 5*60*1000)
	if expected {
		t.Errorf("expected = true, want false for short session")
	}
	if status != MemoryStatusNotExpected {
		t.Errorf("status = %q, want %q", status, MemoryStatusNotExpected)
	}
	if label != "Memory sample not expected yet (<10m active play)" {
		t.Errorf("unexpected label: %q", label)
	}

	// Case 2: Boundary check at 9m 59s
	expected, status, _ = evaluateMemorySampleStatus(0, 599999)
	if expected || status != MemoryStatusNotExpected {
		t.Errorf("wanted not expected at 599,999ms, got expected=%v, status=%v", expected, status)
	}

	// Case 3: Long session (>= 10 minutes active play, no samples)
	expected, status, _ = evaluateMemorySampleStatus(0, 10*60*1000)
	if !expected {
		t.Errorf("expected = false, want true for >=10m session")
	}
	if status != MemoryStatusAbsent {
		t.Errorf("status = %q, want %q", status, MemoryStatusAbsent)
	}

	// Case 4: Samples recorded (even in a short session)
	expected, status, _ = evaluateMemorySampleStatus(2, 2*60*1000)
	if !expected || status != MemoryStatusAvailable {
		t.Errorf("wanted available with samples, got expected=%v, status=%v", expected, status)
	}
}

func sampleCohortDevices() []PuzzleDeviceSummary {
	return []PuzzleDeviceSummary{
		{
			InstallID:           "inst-1",
			DeviceTotalMemoryMB: 3072, // Low (< 4GB)
			NaturalRunsCount:    3,
			CompletedRunsCount:  2,
			PlacementsCount:     10,
			FallsCount:          2,
		},
		{
			InstallID:           "inst-2",
			DeviceTotalMemoryMB: 3800, // Low (< 4GB)
			NaturalRunsCount:    4,
			CompletedRunsCount:  3,
			PlacementsCount:     15,
			FallsCount:          3,
		},
		{
			InstallID:           "inst-3",
			DeviceTotalMemoryMB: 6144, // Medium (4-6GB)
			NaturalRunsCount:    2,
			CompletedRunsCount:  1,
			PlacementsCount:     8,
			FallsCount:          1,
		},
		{
			InstallID:           "inst-4",
			DeviceTotalMemoryMB: 12288, // High (>= 8GB)
			NaturalRunsCount:    6,
			CompletedRunsCount:  5,
			PlacementsCount:     20,
			FallsCount:          2,
		},
		{
			InstallID:           "inst-5",
			DeviceTotalMemoryMB: 0, // Unknown - must not contaminate Low tier
			NaturalRunsCount:    10,
			CompletedRunsCount:  10,
		},
	}
}

func TestBuildMemoryCohorts_GroupingAndRates(t *testing.T) {
	devices := sampleCohortDevices()
	cohorts := BuildMemoryCohorts(devices)
	if len(cohorts) != 3 {
		t.Fatalf("len(cohorts) = %d, want 3", len(cohorts))
	}

	// Low tier: 2 devices, 7 runs, 5 completed (5/7 = 0.714), 5 falls / 25 placements (0.2)
	low := cohorts[0]
	if low.Tier != "low" || low.DeviceCount != 2 || low.NaturalRunsCount != 7 || !low.SampleCountSufficient {
		t.Errorf("unexpected low cohort stats: %+v", low)
	}

	// Medium tier: 1 device, 2 runs (<5 runs -> insufficient)
	med := cohorts[1]
	if med.Tier != "medium" || med.DeviceCount != 1 || med.NaturalRunsCount != 2 || med.SampleCountSufficient {
		t.Errorf("unexpected medium cohort stats: %+v", med)
	}

	// High tier: 1 device, 6 runs (>=5 runs -> sufficient)
	high := cohorts[2]
	if high.Tier != "high" || high.DeviceCount != 1 || high.NaturalRunsCount != 6 || !high.SampleCountSufficient {
		t.Errorf("unexpected high cohort stats: %+v", high)
	}
}

func TestClassifyMarker(t *testing.T) {
	waveIdx := 2
	houseID := 5

	mType, mLabel := classifyMarker("memory_sample", "b1", "b1", nil, nil)
	if mType != "memory_sample" || mLabel != "Memory sample" {
		t.Errorf("memory_sample mismatch: %s, %s", mType, mLabel)
	}

	mType, mLabel = classifyMarker("wave_presented", "b1", "b1", &waveIdx, nil)
	if mType != "wave_start" || mLabel != "Wave 2 started" {
		t.Errorf("wave_start mismatch: %s, %s", mType, mLabel)
	}

	mType, mLabel = classifyMarker("house_run_started", "b1", "b1", nil, &houseID)
	if mType != "house_start" || mLabel != "House 5 started" {
		t.Errorf("house_start mismatch: %s, %s", mType, mLabel)
	}

	// Build update takes precedence
	mType, _ = classifyMarker("memory_sample", "b1", "b2", nil, nil)
	if mType != "build_update" {
		t.Errorf("expected build_update, got %s", mType)
	}
}

func TestTimelineActiveTimeTracking(t *testing.T) {
	state := &timelineState{}
	t0 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	// Event 1 at t0
	act0 := state.updateActiveTime(t0)
	if act0 != 0 {
		t.Errorf("act0 = %d, want 0", act0)
	}

	// Event 2 after 1 minute active foreground
	act1 := state.updateActiveTime(t0.Add(time.Minute))
	if act1 != 60000 {
		t.Errorf("act1 = %d, want 60000", act1)
	}

	// App backgrounded for 2 hours
	state.isBackgrounded = true
	act2 := state.updateActiveTime(t0.Add(time.Minute + 2*time.Hour))
	if act2 != 60000 {
		t.Errorf("active time should not advance while backgrounded, got %d", act2)
	}

	// App foregrounded and plays for another 30 seconds
	state.isBackgrounded = false
	act3 := state.updateActiveTime(t0.Add(time.Minute + 2*time.Hour + 30*time.Second))
	if act3 != 90000 {
		t.Errorf("act3 = %d, want 90000", act3)
	}

	// Long unexplained foreground gap of 2 hours: capped at 30 minutes (pauseGapCapMS)
	act4 := state.updateActiveTime(t0.Add(time.Minute + 4*time.Hour + 30*time.Second))
	if act4 != 90000+pauseGapCapMS {
		t.Errorf("act4 = %d, want %d", act4, 90000+pauseGapCapMS)
	}
}

func TestRegexSafetyForMemoryNumbers(t *testing.T) {
	reg := regexp.MustCompile(`^[0-9]{1,9}$`)

	valid := []string{"0", "128", "4096", "16384", "999999999"}
	for _, v := range valid {
		if !reg.MatchString(v) {
			t.Errorf("expected valid for %s", v)
		}
	}

	invalid := []string{"-1", "0x123", "abc", "4096.5", "10000000000", "'; DROP TABLE events;--", ""}
	for _, inv := range invalid {
		if reg.MatchString(inv) {
			t.Errorf("expected invalid for %s", inv)
		}
	}
}

func TestPuzzleDevices_DeduplicationIntegration(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "99999999-9999-4999-8999-999999999991"
	seedInstallation(t, pool, projectID, install, &now)

	// Seed 3 device_profile events for the same install (simulating scene reloads)
	seedEvents(t, pool, projectID, []seedEvent{
		{
			EventID: "e0000000-0000-4000-8000-000000000001", InstallID: install,
			SessionID: "a0000000-0000-4000-8000-000000000001", Sequence: 1,
			Name: "device_profile", Kind: "product", EffectiveAt: now,
			Properties: map[string]any{"device_total_memory_mb": 3000, "graphics_memory_mb": 512},
		},
		{
			EventID: "e0000000-0000-4000-8000-000000000002", InstallID: install,
			SessionID: "a0000000-0000-4000-8000-000000000001", Sequence: 2,
			Name: "device_profile", Kind: "product", EffectiveAt: now.Add(time.Second),
			Properties: map[string]any{"device_total_memory_mb": 4096, "graphics_memory_mb": 1024},
		},
	})

	res, err := GetPuzzleDevices(ctx, pool, projectID, now.Add(-time.Hour), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("GetPuzzleDevices: %v", err)
	}
	if len(res.Devices) != 1 {
		t.Fatalf("expected 1 deduplicated device, got %d", len(res.Devices))
	}
	if res.Devices[0].DeviceTotalMemoryMB != 4096 {
		t.Errorf("expected latest profile 4096MB, got %d", res.Devices[0].DeviceTotalMemoryMB)
	}
}
