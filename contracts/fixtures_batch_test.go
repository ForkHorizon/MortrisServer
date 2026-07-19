package fixtures

import (
	"encoding/json"
	"testing"

	"github.com/ForkHorizon/Mortris/internal/contracts"
)

func checkBatch(t *testing.T, data []byte, wantValid bool, raw json.RawMessage) {
	t.Helper()
	req, rejected, decodeErr := contracts.DecodeBatchIngestRequest(data)
	if decodeErr != nil {
		assertInvalidBatchEnvelope(t, raw, decodeErr)
		return
	}
	if wantValid {
		assertValidBatch(t, req, rejected)
		return
	}
	assertInvalidBatch(t, req, rejected, raw)
}

func assertInvalidBatchEnvelope(t *testing.T, raw json.RawMessage, err error) {
	t.Helper()
	exp := parseExpectation(t, raw)
	if exp.Envelope != "invalid" {
		t.Fatalf("envelope failed to decode but fixture expected %s: %v", raw, err)
	}
	if got := classifyDecodeError(err); got != exp.Code {
		t.Errorf("expected error code %q, got %q", exp.Code, got)
	}
}

func assertValidBatch(t *testing.T, req *contracts.BatchIngestRequest, rejected []contracts.RejectedEvent) {
	t.Helper()
	if err := req.Validate(); err != nil {
		t.Fatalf("expected valid envelope, got: %v", err)
	}
	if len(rejected) != 0 {
		t.Fatalf("expected no per-event decode rejections, got: %+v", rejected)
	}
	for i, event := range req.Events {
		if err := contracts.ValidateEvent(&event); err != nil {
			t.Errorf("event %d expected valid, got: %v", i, err)
		}
	}
}

func assertInvalidBatch(t *testing.T, req *contracts.BatchIngestRequest, rejected []contracts.RejectedEvent, raw json.RawMessage) {
	t.Helper()
	exp := parseExpectation(t, raw)
	if exp.Envelope == "invalid" {
		assertInvalidEnvelope(t, req, exp)
		return
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected envelope to be valid (only per-event rejections expected), got: %v", err)
	}
	if len(rejected) != 0 {
		t.Fatalf("expected no per-event decode rejections, got: %+v", rejected)
	}
	assertPerEventExpectations(t, req, exp)
}

func parseExpectation(t *testing.T, raw json.RawMessage) expectation {
	t.Helper()
	var exp expectation
	if err := json.Unmarshal(raw, &exp); err != nil {
		t.Fatalf("parse expect: %v", err)
	}
	return exp
}

func assertInvalidEnvelope(t *testing.T, req *contracts.BatchIngestRequest, exp expectation) {
	t.Helper()
	err := req.Validate()
	if err == nil {
		t.Fatalf("expected envelope to be invalid with code %q, but it validated cleanly", exp.Code)
	}
	if got := err.(*contracts.ValidationError).Code; got != exp.Code {
		t.Errorf("expected error code %q, got %q", exp.Code, got)
	}
}

func assertPerEventExpectations(t *testing.T, req *contracts.BatchIngestRequest, exp expectation) {
	t.Helper()
	if len(exp.PerEvent) != len(req.Events) {
		t.Fatalf("manifest lists %d per-event expectations but fixture has %d events", len(exp.PerEvent), len(req.Events))
	}
	for _, expected := range exp.PerEvent {
		assertEventExpectation(t, req.Events[expected.Index], expected)
	}
}

func assertEventExpectation(t *testing.T, event contracts.Event, expected perEventExpectation) {
	t.Helper()
	err := contracts.ValidateEvent(&event)
	if expected.Result == "valid" && err != nil {
		t.Errorf("event %d expected valid, got: %v", expected.Index, err)
		return
	}
	if expected.Result == "invalid" {
		if err == nil {
			t.Errorf("event %d expected invalid with code %q, but validated cleanly", expected.Index, expected.Code)
		} else if got := err.(*contracts.ValidationError).Code; got != expected.Code {
			t.Errorf("event %d: expected error code %q, got %q", expected.Index, expected.Code, got)
		}
		return
	}
	if expected.Result != "valid" {
		t.Fatalf("unknown perEvent result %q", expected.Result)
	}
}
