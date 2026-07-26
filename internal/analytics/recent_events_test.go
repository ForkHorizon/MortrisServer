package analytics

import (
	"context"
	"testing"
	"time"
)

func TestGetRecentEvents_OrdersFiltersAndPaginates(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	installID := "55555555-5555-4555-8555-555555555555"
	seedInstallation(t, pool, projectID, installID, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "b1111111-1111-4111-8111-111111111111", InstallID: installID, SessionID: "c1111111-1111-4111-8111-111111111111", Sequence: 1, Name: "level_start", Kind: "product", EffectiveAt: now, Platform: "android"},
		{EventID: "b2222222-2222-4222-8222-222222222222", InstallID: installID, SessionID: "c1111111-1111-4111-8111-111111111111", Sequence: 2, Name: "level_end", Kind: "product", EffectiveAt: now.Add(time.Minute), Platform: "ios"},
		{EventID: "b3333333-3333-4333-8333-333333333333", InstallID: installID, SessionID: "c1111111-1111-4111-8111-111111111111", Sequence: 3, Name: "level_start", Kind: "product", EffectiveAt: now.Add(2 * time.Minute), Platform: "android"},
	})

	ctx := context.Background()

	// Newest-first, unfiltered.
	all, err := GetRecentEvents(ctx, pool, projectID, RecentEventsFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(all.Events))
	}
	if all.Events[0].EventID != "b3333333-3333-4333-8333-333333333333" || all.Events[2].EventID != "b1111111-1111-4111-8111-111111111111" {
		t.Errorf("expected newest-first order, got %+v", all.Events)
	}

	// Filter by name.
	name := "level_start"
	filtered, err := GetRecentEvents(ctx, pool, projectID, RecentEventsFilter{Name: &name, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Events) != 2 {
		t.Fatalf("expected 2 level_start events, got %d", len(filtered.Events))
	}

	// Keyset pagination: a 2-row page must return a cursor, and the next
	// page fetched with it must return exactly the remaining, older event
	// with no overlap.
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
	if len(page2.Events) != 1 || page2.Events[0].EventID != "b1111111-1111-4111-8111-111111111111" {
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
