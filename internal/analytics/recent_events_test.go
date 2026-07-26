package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedRecentEventsFixture seeds one installation with 3 events spaced a
// minute apart (level_start, level_end, level_start) for the recent-events
// tests below, and returns their event IDs oldest-first.
func seedRecentEventsFixture(t *testing.T, pool *pgxpool.Pool) (projectID string, eventIDs [3]string) {
	t.Helper()
	projectID = seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	installID := "55555555-5555-4555-8555-555555555555"
	seedInstallation(t, pool, projectID, installID, &now)

	eventIDs = [3]string{
		"b1111111-1111-4111-8111-111111111111",
		"b2222222-2222-4222-8222-222222222222",
		"b3333333-3333-4333-8333-333333333333",
	}
	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: eventIDs[0], InstallID: installID, SessionID: "c1111111-1111-4111-8111-111111111111", Sequence: 1, Name: "level_start", Kind: "product", EffectiveAt: now, Platform: "android"},
		{EventID: eventIDs[1], InstallID: installID, SessionID: "c1111111-1111-4111-8111-111111111111", Sequence: 2, Name: "level_end", Kind: "product", EffectiveAt: now.Add(time.Minute), Platform: "ios"},
		{EventID: eventIDs[2], InstallID: installID, SessionID: "c1111111-1111-4111-8111-111111111111", Sequence: 3, Name: "level_start", Kind: "product", EffectiveAt: now.Add(2 * time.Minute), Platform: "android"},
	})
	return projectID, eventIDs
}

func TestGetRecentEvents_NewestFirst(t *testing.T) {
	pool := testPool(t)
	projectID, eventIDs := seedRecentEventsFixture(t, pool)

	all, err := GetRecentEvents(context.Background(), pool, projectID, RecentEventsFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(all.Events))
	}
	if all.Events[0].EventID != eventIDs[2] || all.Events[2].EventID != eventIDs[0] {
		t.Errorf("expected newest-first order, got %+v", all.Events)
	}
}

func TestGetRecentEvents_FiltersByName(t *testing.T) {
	pool := testPool(t)
	projectID, _ := seedRecentEventsFixture(t, pool)

	name := "level_start"
	filtered, err := GetRecentEvents(context.Background(), pool, projectID, RecentEventsFilter{Name: &name, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Events) != 2 {
		t.Fatalf("expected 2 level_start events, got %d", len(filtered.Events))
	}
}

// A 2-row page must return a cursor, and the next page fetched with it
// must return exactly the remaining, older event with no overlap or gap.
func TestGetRecentEvents_Paginates(t *testing.T) {
	pool := testPool(t)
	projectID, eventIDs := seedRecentEventsFixture(t, pool)
	ctx := context.Background()

	page1, err := GetRecentEvents(ctx, pool, projectID, RecentEventsFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Events) != 2 || page1.NextBeforeReceivedAt == nil || page1.NextBeforeEventID == nil {
		t.Fatalf("expected a full page with a next cursor, got %+v", page1)
	}

	before, err := time.Parse(time.RFC3339Nano, *page1.NextBeforeReceivedAt)
	if err != nil {
		t.Fatal(err)
	}
	page2, err := GetRecentEvents(ctx, pool, projectID, RecentEventsFilter{
		Limit:            2,
		BeforeReceivedAt: &before,
		BeforeEventID:    page1.NextBeforeEventID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Events) != 1 || page2.Events[0].EventID != eventIDs[0] {
		t.Fatalf("expected exactly the remaining older event, got %+v", page2.Events)
	}
	if page2.NextBeforeReceivedAt != nil {
		t.Errorf("expected no further cursor once the feed is exhausted, got %v", *page2.NextBeforeReceivedAt)
	}
}

func TestParseRecentEventsFilter_RequiresCursorPairing(t *testing.T) {
	q := map[string][]string{"before_received_at": {"2024-01-01T00:00:00Z"}}
	if _, err := ParseRecentEventsFilter(q); err == nil {
		t.Error("expected an error when before_received_at is given without before_event_id")
	}
}
