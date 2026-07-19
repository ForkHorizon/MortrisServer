package httpapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/ingest"
	"github.com/ForkHorizon/Mortris/internal/store"
)

// These are true handler + PostgreSQL contract tests. They are intentionally
// opt-in so a normal go test does not require a local database:
// MORTRIS_TEST_DSN=postgres://... go test ./internal/httpapi/...
func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("MORTRIS_TEST_DSN")
	if dsn == "" {
		t.Skip("MORTRIS_TEST_DSN not set, skipping ingestion integration tests")
	}
	pool, err := store.NewPool(context.Background(), dsn, 5)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.ApplyMigrations(context.Background(), pool, "../../migrations"); err != nil {
		pool.Close()
		t.Fatalf("apply migrations: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func integrationProject(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	projectID := fmt.Sprintf("httpapi-%d", time.Now().UnixNano())
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO projects (id, environment, display_name, strict_catalog, enabled)
		VALUES ($1, 'test', $1, false, true)
	`, projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return projectID
}

func testCredential(t *testing.T) string {
	t.Helper()
	var credential [32]byte
	if _, err := rand.Read(credential[:]); err != nil {
		t.Fatalf("credential: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(credential[:])
}

func registerBody(projectID, installID, credential string) map[string]any {
	return map[string]any{
		"schema_version":          1,
		"project_id":              projectID,
		"install_id":              installID,
		"installation_credential": credential,
		"sdk_name":                "contract-test",
		"sdk_version":             "1.0.0",
		"app_version":             "1.0.0",
		"build_number":            "1",
		"platform":                "android",
	}
}

func event(eventID, name string) map[string]any {
	return map[string]any{
		"event_id":                eventID,
		"session_id":              "33cef303-b1e3-47b9-a6e6-28322cd927ee",
		"sequence":                1,
		"session_elapsed_ms":      1,
		"name":                    name,
		"occurred_at_client":      "2026-07-16T12:00:00.000Z",
		"app_version":             "1.0.0",
		"build_number":            "1",
		"platform":                "android",
		"os_version":              "15",
		"device_class":            "phone",
		"locale":                  "en-US",
		"timezone_offset_minutes": 0,
		"properties":              map[string]any{},
	}
}

func serveJSON(t *testing.T, handler http.Handler, ctx context.Context, path string, body any, credential string, gzipBody bool) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var reader *bytes.Reader
	if gzipBody {
		var compressed bytes.Buffer
		gz := gzip.NewWriter(&compressed)
		if _, err := gz.Write(data); err != nil {
			t.Fatalf("gzip request: %v", err)
		}
		if err := gz.Close(); err != nil {
			t.Fatalf("close gzip request: %v", err)
		}
		reader = bytes.NewReader(compressed.Bytes())
	} else {
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(http.MethodPost, path, reader).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	if gzipBody {
		req.Header.Set("Content-Encoding", "gzip")
	}
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d: %s", rec.Code, want, rec.Body.String())
	}
}

func TestSDKContractRateLimitAndFailedRequestDoNotAcknowledge(t *testing.T) {
	pool := integrationPool(t)
	projectID := integrationProject(t, pool)
	service := ingest.NewService(pool)
	service.DailyRegistrationCap = 0
	server := NewServer(service, pool, pool)
	installID := "19ffb634-1792-40cd-bd9e-0a89938ff411"
	credential := testCredential(t)

	limited := serveJSON(t, server.Routes(), context.Background(), "/v1/installs/register", registerBody(projectID, installID, credential), "", false)
	requireStatus(t, limited, http.StatusTooManyRequests)
	if limited.Header().Get("Retry-After") == "" {
		t.Fatal("429 response omitted Retry-After")
	}

	// Register a separate install, then cancel its batch request before any DB
	// operation. The handler returns a retryable 5xx and no event is committed.
	service.DailyRegistrationCap = 10000
	installID = "29ffb634-1792-40cd-bd9e-0a89938ff411"
	credential = testCredential(t)
	registered := serveJSON(t, server.Routes(), context.Background(), "/v1/installs/register", registerBody(projectID, installID, credential), "", false)
	requireStatus(t, registered, http.StatusOK)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	eventID := "a9ff0c7c-10a9-4b95-93c4-186079fa5b41"
	failed := serveJSON(t, server.Routes(), cancelled, "/v1/events/batch", map[string]any{
		"schema_version": 1, "project_id": projectID, "install_id": installID,
		"sdk":            map[string]any{"name": "contract-test", "version": "1.0.0"},
		"sent_at_client": "2026-07-16T12:00:00.000Z", "events": []any{event(eventID, "level_start")},
	}, credential, true)
	requireStatus(t, failed, http.StatusInternalServerError)

	var count int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM events WHERE project_id = $1 AND event_id = $2`, projectID, eventID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed request committed %d events", count)
	}
}
