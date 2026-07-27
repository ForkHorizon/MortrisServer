package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/ingest"
)

// TestBatchIngest_RecordsSchemaDrift covers Phase 4: a non-strict project
// with a declared property list still accepts an event carrying an
// undeclared property, but records it as schema drift rather than
// silently ignoring the typo.
func TestBatchIngest_RecordsSchemaDrift(t *testing.T) {
	pool := integrationPool(t)
	projectID := integrationProject(t, pool)
	declareCatalogProperties(t, pool, projectID, "level_end", []string{"score"})
	server := NewServer(ingest.NewService(pool), pool, pool)
	installID, credential := registerContractInstall(t, server, projectID)

	e := event("aaaaaaaa-0000-4000-8000-000000000001", "level_end")
	e["properties"] = map[string]any{"score": 10, "scoree": 11}
	rec := serveJSON(t, server.Routes(), context.Background(), "/v1/events/batch", batchBody(projectID, installID, e), credential, true)
	requireStatus(t, rec, http.StatusOK)

	count := queryCount(t, pool, `SELECT COALESCE(SUM(count), 0) FROM event_property_drift WHERE project_id = $1 AND name = 'level_end' AND property_key = 'scoree'`, projectID)
	if count != 1 {
		t.Fatalf("expected 1 drift observation for scoree, got %d", count)
	}
}

// TestBatchIngest_RecordsRejectionStats covers Phase 4's per-event
// rejection counters: a strict-catalog project rejects an unknown event
// name, and that rejection is attributed to the event name that was
// actually sent, not just counted project-wide.
func TestBatchIngest_RecordsRejectionStats(t *testing.T) {
	pool := integrationPool(t)
	projectID := integrationProject(t, pool)
	setStrictCatalog(t, pool, projectID)
	server := NewServer(ingest.NewService(pool), pool, pool)
	installID, credential := registerContractInstall(t, server, projectID)

	rec := serveJSON(t, server.Routes(), context.Background(), "/v1/events/batch", batchBody(projectID, installID, event("bbbbbbbb-0000-4000-8000-000000000001", "never_declared")), credential, true)
	requireStatus(t, rec, http.StatusOK)

	count := queryCount(t, pool, `SELECT COALESCE(SUM(count), 0) FROM event_rejection_stats WHERE project_id = $1 AND name = 'never_declared' AND code = 'unknown_event'`, projectID)
	if count != 1 {
		t.Fatalf("expected 1 rejection for never_declared, got %d", count)
	}
}

func declareCatalogProperties(t *testing.T, pool *pgxpool.Pool, projectID, name string, properties []string) {
	t.Helper()
	props := make([]map[string]any, len(properties))
	for i, p := range properties {
		props[i] = map[string]any{"name": p}
	}
	propsJSON, err := json.Marshal(props)
	if err != nil {
		t.Fatalf("marshal properties: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO event_catalog (project_id, name, kind, description, properties)
		VALUES ($1, $2, 'product', 'test event', $3)
		ON CONFLICT (project_id, name) DO UPDATE SET properties = EXCLUDED.properties
	`, projectID, name, propsJSON); err != nil {
		t.Fatalf("declare catalog properties: %v", err)
	}
}

func setStrictCatalog(t *testing.T, pool *pgxpool.Pool, projectID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE projects SET strict_catalog = true WHERE id = $1`, projectID); err != nil {
		t.Fatalf("set strict_catalog: %v", err)
	}
}

func queryCount(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int64 {
	t.Helper()
	var count int64
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("query count: %v", err)
	}
	return count
}
