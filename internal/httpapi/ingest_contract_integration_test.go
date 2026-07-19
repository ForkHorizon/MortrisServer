package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ForkHorizon/Mortris/internal/contracts"
	"github.com/ForkHorizon/Mortris/internal/ingest"
)

func TestSDKContractRegistrationAndBatchRetries(t *testing.T) {
	pool := integrationPool(t)
	projectID := integrationProject(t, pool)
	server := NewServer(ingest.NewService(pool), pool, pool)
	installID, credential := registerContractInstall(t, server, projectID)
	assertBatchDeliveryRetries(t, server, projectID, installID, credential)
	assertPartialBatchAndPolicy(t, server, projectID, installID, credential)
}

func registerContractInstall(t *testing.T, server *Server, projectID string) (string, string) {
	t.Helper()
	installID := "09ffb634-1792-40cd-bd9e-0a89938ff411"
	credential := testCredential(t)
	for range 2 {
		rec := serveJSON(t, server.Routes(), context.Background(), "/v1/installs/register", registerBody(projectID, installID, credential), "", false)
		requireStatus(t, rec, http.StatusOK)
	}
	assertRegistrationConflict(t, server, projectID, installID)
	return installID, credential
}

func assertRegistrationConflict(t *testing.T, server *Server, projectID, installID string) {
	t.Helper()
	rec := serveJSON(t, server.Routes(), context.Background(), "/v1/installs/register", registerBody(projectID, installID, testCredential(t)), "", false)
	requireStatus(t, rec, http.StatusConflict)
	var body contracts.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != contracts.CodeInstallConflict {
		t.Fatalf("conflict code = %q", body.Code)
	}
}

func assertBatchDeliveryRetries(t *testing.T, server *Server, projectID, installID, credential string) {
	t.Helper()
	eventID := "79ff0c7c-10a9-4b95-93c4-186079fa5b41"
	body := batchBody(projectID, installID, event(eventID, "level_start"))
	first := serveJSON(t, server.Routes(), context.Background(), "/v1/events/batch", body, credential, true)
	requireStatus(t, first, http.StatusOK)
	assertAcknowledgement(t, first, eventID, false)
	second := serveJSON(t, server.Routes(), context.Background(), "/v1/events/batch", body, credential, true)
	requireStatus(t, second, http.StatusOK)
	assertAcknowledgement(t, second, eventID, true)
}

func assertAcknowledgement(t *testing.T, rec *httptest.ResponseRecorder, eventID string, duplicate bool) {
	t.Helper()
	var body contracts.BatchIngestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	ids := body.Accepted
	if duplicate {
		ids = body.Duplicates
	}
	if len(ids) != 1 || ids[0] != eventID {
		t.Fatalf("batch response = %#v", body)
	}
}

func assertPartialBatchAndPolicy(t *testing.T, server *Server, projectID, installID, credential string) {
	t.Helper()
	validID := "89ff0c7c-10a9-4b95-93c4-186079fa5b41"
	invalidID := "99ff0c7c-10a9-4b95-93c4-186079fa5b41"
	rec := serveJSON(t, server.Routes(), context.Background(), "/v1/events/batch", batchBody(projectID, installID, event(validID, "level_start"), event(invalidID, "BadName")), credential, true)
	requireStatus(t, rec, http.StatusOK)
	var body contracts.BatchIngestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Accepted) != 1 || len(body.Rejected) != 1 || body.Rejected[0].EventID != invalidID {
		t.Fatalf("partial response = %#v", body)
	}
	policy := serveJSON(t, server.Routes(), context.Background(), "/v1/client/policy", policyBody(projectID, installID), credential, false)
	requireStatus(t, policy, http.StatusOK)
}

func batchBody(projectID, installID string, events ...map[string]any) map[string]any {
	return map[string]any{"schema_version": 1, "project_id": projectID, "install_id": installID, "sdk": map[string]any{"name": "contract-test", "version": "1.0.0"}, "sent_at_client": "2026-07-16T12:00:00.000Z", "events": events}
}

func policyBody(projectID, installID string) map[string]any {
	return map[string]any{"schema_version": 1, "project_id": projectID, "install_id": installID, "sdk": map[string]any{"name": "contract-test", "version": "1.0.0"}, "app_version": "1.0.0", "build_number": "1", "platform": "android"}
}
