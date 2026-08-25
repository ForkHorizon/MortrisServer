package analytics

import (
	"context"
	"testing"
	"time"
)

// gameplayTestProps builds the common property set every seeded gameplay
// event needs (content_revision in particular — loadGameplayDaily scans it
// into a non-nullable string column).
func gameplayTestProps(attemptID string, activeMs int64) map[string]any {
	return map[string]any{
		"attempt_id":        attemptID,
		"city_id":           1,
		"house_id":          1,
		"wave_index":        1,
		"content_revision":  "rev1",
		"active_elapsed_ms": activeMs,
	}
}

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

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "f1111111-1111-4111-8111-111111111111", InstallID: installID, SessionID: "b1111111-1111-4111-8111-111111111111", Sequence: 1, Name: "wave_started", Kind: "product", EffectiveAt: now, Properties: gameplayTestProps("attempt-1", 0)},
		{EventID: "f2222222-2222-4222-8222-222222222222", InstallID: installID, SessionID: "b1111111-1111-4111-8111-111111111111", Sequence: 2, Name: "app_backgrounded", Kind: "product", EffectiveAt: now.Add(time.Minute), Properties: gameplayTestProps("attempt-1", 60_000)},
		// Resumed 3 days later and completed — without capping this alone
		// would add ~3 days to wall time. Ends in wave_completed so the
		// attempt is "settled" and counts toward the top-level summary.
		{EventID: "f3333333-3333-4333-8333-333333333333", InstallID: installID, SessionID: "b2222222-2222-4222-8222-222222222222", Sequence: 1, Name: "wave_completed", Kind: "product", EffectiveAt: now.Add(time.Minute + 72*time.Hour), Properties: gameplayTestProps("attempt-1", 120_000)},
	})

	result, err := GetGameplayDiagnostics(context.Background(), pool, projectID, now.Add(-time.Hour), now.Add(73*time.Hour), time.UTC, GameplayFilter{}, ScopeNaturalOnly)
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

// TestLoadGameplayOutcomes_ClassifiesAttempts covers all four outcome
// buckets in one project: an attempt only has a real, final duration once
// it's settled (completed or explicitly gave up) — one still being played
// and one silently lost mid-attempt shouldn't be averaged in as if they
// were finished sessions.
func TestLoadGameplayOutcomes_ClassifiesAttempts(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	installID := "88888888-8888-4888-8888-888888888888"
	seedInstallation(t, pool, projectID, installID, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		// completed: reaches wave_completed.
		{EventID: "a1111111-1111-4111-8111-111111111111", InstallID: installID, SessionID: "b1111111-1111-4111-8111-111111111111", Sequence: 1, Name: "wave_started", Kind: "product", EffectiveAt: now.Add(-2 * time.Hour), Properties: gameplayTestProps("attempt-completed", 0)},
		{EventID: "a2222222-2222-4222-8222-222222222222", InstallID: installID, SessionID: "b1111111-1111-4111-8111-111111111111", Sequence: 2, Name: "wave_completed", Kind: "product", EffectiveAt: now.Add(-2*time.Hour + time.Minute), Properties: gameplayTestProps("attempt-completed", 60_000)},
		// gave_up: closed without ever completing the wave.
		{EventID: "a3333333-3333-4333-8333-333333333333", InstallID: installID, SessionID: "b3333333-3333-4333-8333-333333333333", Sequence: 1, Name: "wave_started", Kind: "product", EffectiveAt: now.Add(-3 * time.Hour), Properties: gameplayTestProps("attempt-gave-up", 0)},
		{EventID: "a4444444-4444-4444-8444-444444444444", InstallID: installID, SessionID: "b3333333-3333-4333-8333-333333333333", Sequence: 2, Name: "attempt_closed", Kind: "product", EffectiveAt: now.Add(-3*time.Hour + time.Minute), Properties: gameplayTestProps("attempt-gave-up", 30_000)},
		// in_progress: no terminal event, last event just a minute ago.
		{EventID: "a5555555-5555-4555-8555-555555555555", InstallID: installID, SessionID: "b4444444-4444-4444-8444-444444444444", Sequence: 1, Name: "wave_started", Kind: "product", EffectiveAt: now.Add(-time.Minute), Properties: gameplayTestProps("attempt-in-progress", 0)},
		// lost: no terminal event, last event well past lostThresholdHours.
		{EventID: "a6666666-6666-4666-8666-666666666666", InstallID: installID, SessionID: "b5555555-5555-4555-8555-555555555555", Sequence: 1, Name: "wave_started", Kind: "product", EffectiveAt: now.Add(-48 * time.Hour), Properties: gameplayTestProps("attempt-lost", 0)},
	})

	result, err := GetGameplayDiagnostics(context.Background(), pool, projectID, now.Add(-72*time.Hour), now.Add(time.Hour), time.UTC, GameplayFilter{}, ScopeNaturalOnly)
	if err != nil {
		t.Fatalf("GetGameplayDiagnostics: %v", err)
	}

	assertOutcomeBuckets(t, result)
}

func assertOutcomeBuckets(t *testing.T, result *GameplayDiagnostics) {
	t.Helper()
	if result.Summary.Attempts != 4 {
		t.Errorf("Attempts = %d, want 4 (all attempts count, regardless of outcome)", result.Summary.Attempts)
	}

	got := map[string]GameplayOutcomeStat{}
	for _, o := range result.Summary.ByOutcome {
		got[o.Outcome] = o
	}
	for _, outcome := range []string{"completed", "gave_up", "in_progress", "lost"} {
		if got[outcome].Attempts != 1 {
			t.Errorf("ByOutcome[%s].Attempts = %d, want 1 (got buckets: %+v)", outcome, got[outcome].Attempts, got)
		}
	}

	// Only completed + gave_up have a settled duration, so the top-level
	// summary should equal their sum (60s + 60s of wall time) and exclude
	// the in_progress/lost attempts entirely.
	wantWallMS := int64(2 * time.Minute / time.Millisecond)
	if result.Summary.WallElapsedMS != wantWallMS {
		t.Errorf("WallElapsedMS = %d, want %d (completed+gave_up only)", result.Summary.WallElapsedMS, wantWallMS)
	}
}
