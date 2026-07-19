package contracts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// DecodeRegisterRequest strictly decodes a registration envelope. An
// unknown top-level field rejects the whole request (section 5.1).
func DecodeRegisterRequest(data []byte) (*RegisterRequest, error) {
	var req RegisterRequest
	if err := decodeStrict(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// DecodePolicyRequest strictly decodes a client-policy probe envelope.
func DecodePolicyRequest(data []byte) (*PolicyRequest, error) {
	var req PolicyRequest
	if err := decodeStrict(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// batchEnvelope mirrors BatchIngestRequest but leaves each event as raw
// JSON, so an unknown field inside one event rejects only that event
// instead of the whole envelope (section 5.1).
type batchEnvelope struct {
	SchemaVersion int               `json:"schema_version"`
	ProjectID     string            `json:"project_id"`
	InstallID     string            `json:"install_id"`
	SDK           SDKInfo           `json:"sdk"`
	SentAtClient  string            `json:"sent_at_client"`
	Events        []json.RawMessage `json:"events"`
}

// DecodeBatchIngestRequest strictly decodes the envelope, then decodes each
// event independently. Events that fail to decode (unknown field or wrong
// type) come back as rejections rather than failing the whole request —
// their siblings can still be accepted (section 5.4).
func DecodeBatchIngestRequest(data []byte) (*BatchIngestRequest, []RejectedEvent, error) {
	var env batchEnvelope
	// Decode only the envelope here. Each event is decoded below so a bad
	// event (including duplicate JSON keys) cannot reject valid siblings.
	if err := decodeBatchEnvelope(data, &env); err != nil {
		return nil, nil, err
	}

	req := &BatchIngestRequest{
		SchemaVersion: env.SchemaVersion,
		ProjectID:     env.ProjectID,
		InstallID:     env.InstallID,
		SDK:           env.SDK,
		SentAtClient:  env.SentAtClient,
		EventCount:    len(env.Events),
	}
	if req.EventCount < minEventsPerBatch || req.EventCount > maxEventsPerBatch {
		return nil, nil, invalid(CodeInvalidBatchSize, fmt.Sprintf("events must contain %d to %d items", minEventsPerBatch, maxEventsPerBatch))
	}
	seen := make(map[string]struct{}, req.EventCount)
	for _, raw := range env.Events {
		if eventID := bestEffortEventID(raw); eventID != "" {
			if _, exists := seen[eventID]; exists {
				return nil, nil, invalid(CodeDuplicateEventIDInBatch, "duplicate event_id within one request: "+eventID)
			}
			seen[eventID] = struct{}{}
		}
	}

	var rejected []RejectedEvent
	for _, raw := range env.Events {
		var e Event
		if err := decodeStrict(raw, &e); err != nil {
			rejected = append(rejected, RejectedEvent{
				EventID: bestEffortEventID(raw),
				Code:    decodeErrorCode(err),
			})
			continue
		}
		req.Events = append(req.Events, e)
	}
	return req, rejected, nil
}

func decodeStrict(data []byte, v any) error {
	if !utf8.Valid(data) {
		return &ValidationError{Code: CodeInvalidRequest, Message: "body must be valid UTF-8"}
	}
	if err := rejectDuplicateKeys(data, true); err != nil {
		return err
	}
	return decodeJSON(data, v)
}

func decodeBatchEnvelope(data []byte, v any) error {
	if !utf8.Valid(data) {
		return &ValidationError{Code: CodeInvalidRequest, Message: "body must be valid UTF-8"}
	}
	// The envelope is strict, but event bodies stay opaque until their
	// independent decode so malformed siblings become per-event rejections.
	if err := rejectBatchEnvelopeDuplicateKeys(data); err != nil {
		return err
	}
	return decodeJSON(data, v)
}

func decodeJSON(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return &ValidationError{Code: CodeInvalidRequest, Message: "body must contain exactly one JSON value"}
	}
	return nil
}

// rejectDuplicateKeys rejects ambiguous JSON objects. recursive is false for
// the batch envelope only: its raw event payloads are validated separately.
func rejectDuplicateKeys(data []byte, recursive bool) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValue(dec, recursive); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return &ValidationError{Code: CodeInvalidRequest, Message: "body must contain exactly one JSON value"}
	}
	return nil
}

func rejectBatchEnvelopeDuplicateKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	token, err := dec.Token()
	if err != nil {
		return err
	}
	if token != json.Delim('{') {
		return &ValidationError{Code: CodeInvalidRequest, Message: "batch envelope must be a JSON object"}
	}

	seen := map[string]struct{}{}
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("object key must be a string")
		}
		if _, exists := seen[key]; exists {
			return &ValidationError{Code: CodeInvalidRequest, Message: "duplicate JSON key: " + key}
		}
		seen[key] = struct{}{}

		if key == "events" {
			// Raw event payloads remain opaque until the per-event decoder runs.
			if err := skipJSONValue(dec); err != nil {
				return err
			}
			continue
		}
		if err := scanJSONValue(dec, true); err != nil {
			return err
		}
	}
	if _, err := dec.Token(); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return &ValidationError{Code: CodeInvalidRequest, Message: "body must contain exactly one JSON value"}
	}
	return nil
}

func scanJSONValue(dec *json.Decoder, recursive bool) error {
	return scanJSONValueAtDepth(dec, recursive, 0)
}

const maxJSONDepth = 64

func scanJSONValueAtDepth(dec *json.Decoder, recursive bool, depth int) error {
	if depth > maxJSONDepth {
		return &ValidationError{Code: CodeInvalidRequest, Message: "JSON nesting exceeds limit"}
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key must be a string")
			}
			if _, exists := seen[key]; exists {
				return &ValidationError{Code: CodeInvalidRequest, Message: "duplicate JSON key: " + key}
			}
			seen[key] = struct{}{}
			if recursive {
				if err := scanJSONValueAtDepth(dec, true, depth+1); err != nil {
					return err
				}
			} else if err := skipJSONValue(dec); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	case '[':
		for dec.More() {
			if err := scanJSONValueAtDepth(dec, recursive, depth+1); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter")
	}
}

func skipJSONValue(dec *json.Decoder) error {
	return skipJSONValueAtDepth(dec, 0)
}

func skipJSONValueAtDepth(dec *json.Decoder, depth int) error {
	if depth > maxJSONDepth {
		return &ValidationError{Code: CodeInvalidRequest, Message: "JSON nesting exceeds limit"}
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim == '{' {
		for dec.More() {
			if _, err := dec.Token(); err != nil {
				return err
			}
			if err := skipJSONValueAtDepth(dec, depth+1); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	if delim == '[' {
		for dec.More() {
			if err := skipJSONValueAtDepth(dec, depth+1); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	return fmt.Errorf("unexpected JSON delimiter")
}

// decodeErrorCode classifies a strict-decode failure into a stable code.
// Good enough for contract tests; the HTTP layer built in Phase S1 can
// refine this further if a case needs a more specific code.
func decodeErrorCode(err error) string {
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Code
	}
	if strings.Contains(err.Error(), "unknown field") {
		return CodeUnknownField
	}
	return CodeInvalidRequest
}

// bestEffortEventID recovers event_id from a raw event that otherwise
// failed strict decoding, so the caller can still report which event was
// rejected.
func bestEffortEventID(raw json.RawMessage) string {
	var probe struct {
		EventID string `json:"event_id"`
	}
	_ = json.Unmarshal(raw, &probe)
	return probe.EventID
}
