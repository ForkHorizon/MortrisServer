package contracts

import "testing"

func TestDecodeRegisterRequestRejectsDuplicateKeysAndTrailingJSON(t *testing.T) {
	duplicate := []byte(`{"schema_version":1,"schema_version":1}`)
	if _, err := DecodeRegisterRequest(duplicate); err == nil {
		t.Fatal("duplicate key was accepted")
	}

	trailing := []byte(`{"schema_version":1} {}`)
	if _, err := DecodeRegisterRequest(trailing); err == nil {
		t.Fatal("trailing JSON value was accepted")
	}
}

func TestDecodeBatchRejectsOnlyEventWithDuplicateKeys(t *testing.T) {
	body := []byte(`{
  "schema_version": 1,
  "project_id": "puzzle-production",
  "install_id": "09ffb634-1792-40cd-bd9e-0a89938ff411",
  "sdk": {"name":"daliys-unity","version":"0.1.0"},
  "sent_at_client": "2026-07-16T12:00:00.000Z",
  "events": [
    {"event_id":"79ff0c7c-10a9-4b95-93c4-186079fa5b41","event_id":"79ff0c7c-10a9-4b95-93c4-186079fa5b41"},
    {"event_id":"89ff0c7c-10a9-4b95-93c4-186079fa5b41","session_id":"33cef303-b1e3-47b9-a6e6-28322cd927ee","sequence":1,"session_elapsed_ms":1,"name":"level_start","occurred_at_client":"2026-07-16T12:00:00.000Z","app_version":"1","build_number":"1","platform":"android","os_version":"15","device_class":"phone","locale":"en-US","timezone_offset_minutes":0,"properties":{}}
  ]
}`)

	req, rejected, err := DecodeBatchIngestRequest(body)
	if err != nil {
		t.Fatalf("decode batch: %v", err)
	}
	if len(req.Events) != 1 {
		t.Fatalf("valid events = %d, want 1", len(req.Events))
	}
	if len(rejected) != 1 || rejected[0].Code != CodeInvalidRequest {
		t.Fatalf("rejected = %#v, want one invalid_request", rejected)
	}
}

func TestDecodeBatchRejectsDuplicateEnvelopeKeysOutsideEvents(t *testing.T) {
	body := []byte(`{
  "schema_version": 1,
  "project_id": "puzzle-production",
  "install_id": "09ffb634-1792-40cd-bd9e-0a89938ff411",
  "sdk": {"name":"daliys-unity","name":"other","version":"0.1.0"},
  "sent_at_client": "2026-07-16T12:00:00.000Z",
  "events": []
}`)
	if _, _, err := DecodeBatchIngestRequest(body); err == nil {
		t.Fatal("duplicate SDK key was accepted")
	}
}

func TestDecodeBatchRejectsDuplicateIDsBeforeDroppingMalformedEvents(t *testing.T) {
	body := []byte(`{
  "schema_version": 1,
  "project_id": "puzzle-production",
  "install_id": "09ffb634-1792-40cd-bd9e-0a89938ff411",
  "sdk": {"name":"daliys-unity","version":"0.1.0"},
  "sent_at_client": "2026-07-16T12:00:00.000Z",
  "events": [
    {"event_id":"79ff0c7c-10a9-4b95-93c4-186079fa5b41","unknown":true},
    {"event_id":"79ff0c7c-10a9-4b95-93c4-186079fa5b41","session_id":"33cef303-b1e3-47b9-a6e6-28322cd927ee","sequence":1,"session_elapsed_ms":1,"name":"level_start","occurred_at_client":"2026-07-16T12:00:00.000Z","app_version":"1","build_number":"1","platform":"android","os_version":"15","device_class":"phone","locale":"en-US","timezone_offset_minutes":0,"properties":{}}
  ]
}`)
	if _, _, err := DecodeBatchIngestRequest(body); err == nil {
		t.Fatal("duplicate event IDs were accepted")
	} else if got := err.(*ValidationError).Code; got != CodeDuplicateEventIDInBatch {
		t.Fatalf("code = %q, want %q", got, CodeDuplicateEventIDInBatch)
	}
}
