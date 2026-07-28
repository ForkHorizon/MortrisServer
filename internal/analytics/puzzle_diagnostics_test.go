package analytics

import (
	"context"
	"testing"
	"time"
)

// TestLoadGameplaySummary_CapsLongBackgroundGap guards the fix for a
// production bug: an attempt left backgrounded for days (app never
// force-closed) was inflating the whole-project wall-time sum by that many
// days, even though only a couple of minutes were actually played.
func TestLoadGameplaySummary_CapsLongBackgroundGap(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	installID := "77777777-7777-4777-8777-777777777777"
	seedInstallation(t, pool, projectID, installID, &now)

	props := func(activeMs int64) map[string]any {
		return map[string]any{
			"attempt_id":        "attempt-1",
			"city_id":           1,
			"house_id":          1,
			"wave_index":        1,
			"active_elapsed_ms": activeMs,
		}
	}

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "f1111111-1111-4111-8111-111111111111", InstallID: installID, SessionID: "s1", Sequence: 1, Name: "wave_started", Kind: "product", EffectiveAt: now, Properties: props(0)},
		{EventID: "f2222222-2222-4222-8222-222222222222", InstallID: installID, SessionID: "s1", Sequence: 2, Name: "app_backgrounded", Kind: "product", EffectiveAt: now.Add(time.Minute), Properties: props(60_000)},
		// Resumed 3 days later — without capping this alone would add ~3 days to wall time.
		{EventID: "f3333333-3333-4333-8333-333333333333", InstallID: installID, SessionID: "s2", Sequence: 1, Name: "placement_resolved", Kind: "product", EffectiveAt: now.Add(time.Minute + 72*time.Hour), Properties: props(120_000)},
	})

	result, err := GetGameplayDiagnostics(context.Background(), pool, projectID, now.Add(-time.Hour), now.Add(73*time.Hour), time.UTC, GameplayFilter{})
	if err != nil {
		t.Fatalf("GetGameplayDiagnostics: %v", err)
	}

	// One 1-minute gap (event 1->2) plus one capped 30-minute gap (event 2->3),
	// instead of the raw ~72h1m gap.
	wantMaxWallMS := int64(time.Minute/time.Millisecond) + pauseGapCapMS
	if result.Summary.WallElapsedMS != wantMaxWallMS {
		t.Errorf("WallElapsedMS = %d, want %d (capped)", result.Summary.WallElapsedMS, wantMaxWallMS)
	}
	if result.Summary.WallElapsedMS >= int64(24*time.Hour/time.Millisecond) {
		t.Errorf("WallElapsedMS = %d looks uncapped (>=24h)", result.Summary.WallElapsedMS)
	}
}
