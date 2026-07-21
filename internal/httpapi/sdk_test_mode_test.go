package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ForkHorizon/Mortris/internal/contracts"
	"github.com/ForkHorizon/Mortris/internal/sdktest"
)

func TestSDKTestBatchFailuresUseStableResponses(t *testing.T) {
	server := NewServer(nil, nil, nil)
	server.EnableSDKTest("sdk-test", "test-token-123456")
	tests := []struct {
		scenario sdktest.Scenario
		status   int
		code     string
	}{
		{sdktest.UnauthorizedOnce, http.StatusUnauthorized, contracts.CodeUnauthorized},
		{sdktest.PayloadTooLarge, http.StatusRequestEntityTooLarge, contracts.CodePayloadTooLarge},
		{sdktest.RateLimited, http.StatusTooManyRequests, contracts.CodeRateLimited},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodPost, "/v1/events/batch", nil)
		rec := httptest.NewRecorder()
		batch := &contracts.BatchIngestRequest{ProjectID: "sdk-test", InstallID: string(tt.scenario) + "-install"}
		if !server.batchTestFailure(rec, req, "request-id", time.Now(), batch, tt.scenario) {
			t.Fatalf("%s did not intercept the batch", tt.scenario)
		}
		assertSDKTestError(t, rec, tt.status, tt.code)
	}
}

func assertSDKTestError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d", rec.Code, status)
	}
	var response contracts.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if response.Code != code {
		t.Fatalf("code = %q, want %q", response.Code, code)
	}
	if status == http.StatusTooManyRequests && rec.Header().Get("Retry-After") != "2" {
		t.Fatalf("Retry-After = %q, want 2", rec.Header().Get("Retry-After"))
	}
}
