package analytics

import (
	"context"
	"testing"
	"time"
)

func TestGetPropertyValues_TopValuesByCount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "99999999-9999-4999-8999-999999999999"
	seedInstallation(t, pool, projectID, install, &now)

	if _, err := pool.Exec(ctx, `
		INSERT INTO event_catalog (project_id, name, kind, properties)
		VALUES ($1, 'level_start', 'product', '[{"name":"house_id","type":"string"}]')
		ON CONFLICT (project_id, name) DO NOTHING
	`, projectID); err != nil {
		t.Fatal(err)
	}

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "f1111111-1111-4111-8111-111111111111", InstallID: install, SessionID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Sequence: 1, Name: "level_start", Kind: "product", EffectiveAt: now, Properties: map[string]any{"house_id": "rome"}},
		{EventID: "f2222222-2222-4222-8222-222222222222", InstallID: install, SessionID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Sequence: 2, Name: "level_start", Kind: "product", EffectiveAt: now.Add(time.Minute), Properties: map[string]any{"house_id": "rome"}},
		{EventID: "f3333333-3333-4333-8333-333333333333", InstallID: install, SessionID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Sequence: 3, Name: "level_start", Kind: "product", EffectiveAt: now.Add(2 * time.Minute), Properties: map[string]any{"house_id": "milan"}},
		{EventID: "f4444444-4444-4444-8444-444444444444", InstallID: install, SessionID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Sequence: 4, Name: "level_start", Kind: "product", EffectiveAt: now.Add(3 * time.Minute), Properties: map[string]any{}},
	})

	q := make(map[string][]string)
	q["name"] = []string{"level_start"}
	q["property_key"] = []string{"house_id"}
	filter, err := ParsePropertyValuesFilter(ctx, pool, projectID, q)
	if err != nil {
		t.Fatalf("ParsePropertyValuesFilter: %v", err)
	}

	result, err := GetPropertyValues(ctx, pool, projectID, filter, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Values) != 3 {
		t.Fatalf("expected 3 distinct values (rome, milan, missing), got %d: %+v", len(result.Values), result.Values)
	}
	if result.Values[0].Value != "rome" || result.Values[0].Count != 2 {
		t.Errorf("expected top value rome:2, got %+v", result.Values[0])
	}
}

func TestParsePropertyValuesFilter_RejectsUncatalogedProperty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)

	if _, err := pool.Exec(ctx, `
		INSERT INTO event_catalog (project_id, name, kind, properties)
		VALUES ($1, 'level_start', 'product', '[]')
		ON CONFLICT (project_id, name) DO NOTHING
	`, projectID); err != nil {
		t.Fatal(err)
	}

	q := make(map[string][]string)
	q["name"] = []string{"level_start"}
	q["property_key"] = []string{"not_declared"}
	if _, err := ParsePropertyValuesFilter(ctx, pool, projectID, q); err == nil {
		t.Fatal("expected an error for an uncataloged property key")
	}
}

func TestParsePropertyValuesFilter_RequiresBothParams(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)

	q := make(map[string][]string)
	q["name"] = []string{"level_start"}
	if _, err := ParsePropertyValuesFilter(ctx, pool, projectID, q); err == nil {
		t.Fatal("expected an error when property_key is missing")
	}
}
