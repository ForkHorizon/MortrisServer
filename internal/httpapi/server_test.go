package httpapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

func TestRoutesLive(t *testing.T) {
	srv := NewServer(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRoutesSessionRequiresCookie(t *testing.T) {
	srv := NewServer(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var body contracts.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	if body.Code != adminauth.CodeSessionInvalid {
		t.Fatalf("code = %q, want %q", body.Code, adminauth.CodeSessionInvalid)
	}
}

func TestRoutesRegisterUnknownField(t *testing.T) {
	srv := NewServer(nil, nil, nil)
	body := `{
		"schema_version": 1,
		"project_id": "puzzle-production",
		"install_id": "09ffb634-1792-40cd-bd9e-0a89938ff411",
		"installation_credential": "base64url-encoded-32-random-bytes",
		"sdk_name": "daliys-unity",
		"sdk_version": "0.1.0",
		"app_version": "1.4.0",
		"build_number": "140",
		"platform": "android",
		"unexpected": true
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/installs/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var resp contracts.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	if resp.Code != contracts.CodeUnknownField {
		t.Fatalf("code = %q, want %q", resp.Code, contracts.CodeUnknownField)
	}
}

func TestRoutesBatchRejectsInvalidGzip(t *testing.T) {
	srv := NewServer(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/events/batch", strings.NewReader("not-gzip"))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRoutesBatchRequiresGzip(t *testing.T) {
	srv := NewServer(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/events/batch", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRoutesBatchRejectsOversizedPayload(t *testing.T) {
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(bytes.Repeat([]byte("a"), maxDecompressedBody+1)); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	srv := NewServer(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/events/batch", bytes.NewReader(compressed.Bytes()))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	var response contracts.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	if response.Code != contracts.CodePayloadTooLarge {
		t.Fatalf("code = %q, want %q", response.Code, contracts.CodePayloadTooLarge)
	}
}

func TestReadBodyRejectsOversizedCompressedPayload(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/events/batch", bytes.NewReader(bytes.Repeat([]byte("x"), maxCompressedBody+1)))
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()

	_, err := readBody(rec, req)
	if err != errBodyTooLarge {
		t.Fatalf("err = %v, want %v", err, errBodyTooLarge)
	}
}

func TestReadBodyRejectsOversizedDecompressedPayload(t *testing.T) {
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(bytes.Repeat([]byte("a"), maxDecompressedBody+1)); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/events/batch", bytes.NewReader(compressed.Bytes()))
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()

	_, err := readBody(rec, req)
	if err != errBodyTooLarge {
		t.Fatalf("err = %v, want %v", err, errBodyTooLarge)
	}
}

func TestRequireProjectAccess(t *testing.T) {
	sess := &adminauth.Session{ProjectIDs: []string{"alpha"}}

	t.Run("missing project query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/overview", nil)
		_, err := requireProjectAccess(sess, req)
		if err == nil {
			t.Fatal("expected error for missing project query")
		}
	})

	t.Run("forbidden project", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/overview?project=beta", nil)
		_, err := requireProjectAccess(sess, req)
		if err == nil {
			t.Fatal("expected error for forbidden project")
		}
	})

	t.Run("allowed project", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/overview?project=alpha", nil)
		projectID, err := requireProjectAccess(sess, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if projectID != "alpha" {
			t.Fatalf("projectID = %q, want %q", projectID, "alpha")
		}
	})
}

func TestHandleDashboardServesSPAIndexForClientRoute(t *testing.T) {
	srv := NewServer(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("content-type = %q, want text/html", got)
	}
	if !strings.Contains(rec.Body.String(), `<div id="root"></div>`) {
		t.Fatalf("body did not contain SPA root: %s", rec.Body.String())
	}
}
