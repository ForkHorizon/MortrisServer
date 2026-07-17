package contracts

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"unicode/utf8"
)

var (
	uuidPattern      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	eventNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	// RFC 3339 UTC with exactly millisecond precision, e.g. 2026-07-16T12:00:00.120Z.
	timestampPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`)
)

// ReservedSystemEvents are the only names the server treats as event_kind
// "system" (section 7). Any other sys_-prefixed name is a naming violation,
// not a silently-accepted product event.
var ReservedSystemEvents = map[string]bool{
	"sys_session_start":  true,
	"sys_app_background": true,
	"sys_sdk_health":     true,
}

const (
	maxEventsPerBatch     = 100
	minEventsPerBatch     = 1
	maxProperties         = 32
	maxPropertyKeyBytes   = 64
	maxPropertyValueBytes = 1024
	maxPropertiesBytes    = 8 * 1024
	credentialBytes       = 32
)

func isValidUUID(s string) bool { return uuidPattern.MatchString(s) }

func isValidTimestamp(s string) bool { return timestampPattern.MatchString(s) }

// Validate checks the registration envelope (section 5.2). Unknown JSON
// fields are rejected by the caller's strict decoder, not here.
func (r *RegisterRequest) Validate() error {
	if r.SchemaVersion != 1 {
		return invalid(CodeInvalidRequest, "schema_version must be 1")
	}
	if r.ProjectID == "" {
		return invalid(CodeInvalidRequest, "project_id is required")
	}
	if !isValidUUID(r.InstallID) {
		return invalid(CodeInvalidUUID, "install_id must be a canonical UUIDv4")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(r.InstallationCredential)
	if err != nil || len(decoded) != credentialBytes {
		return invalid(CodeInvalidCredential, "installation_credential must be 32 bytes, unpadded base64url")
	}
	if r.SDKName == "" || r.SDKVersion == "" {
		return invalid(CodeInvalidRequest, "sdk_name and sdk_version are required")
	}
	if r.Platform == "" {
		return invalid(CodeInvalidRequest, "platform is required")
	}
	return nil
}

// Validate checks the batch envelope only: schema/project/install/SDK
// fields, size bounds, and in-request duplicate event IDs. Per-event
// validation is ValidateEvent, called separately so one bad event never
// invalidates its siblings (section 5.4).
func (b *BatchIngestRequest) Validate() error {
	if b.SchemaVersion != 1 {
		return invalid(CodeInvalidRequest, "schema_version must be 1")
	}
	if b.ProjectID == "" {
		return invalid(CodeInvalidRequest, "project_id is required")
	}
	if !isValidUUID(b.InstallID) {
		return invalid(CodeInvalidUUID, "install_id must be a canonical UUIDv4")
	}
	if b.SDK.Name == "" || b.SDK.Version == "" {
		return invalid(CodeInvalidRequest, "sdk.name and sdk.version are required")
	}
	if !isValidTimestamp(b.SentAtClient) {
		return invalid(CodeInvalidTimestamp, "sent_at_client must be RFC 3339 UTC with millisecond precision")
	}
	if len(b.Events) < minEventsPerBatch || len(b.Events) > maxEventsPerBatch {
		return invalid(CodeInvalidBatchSize, fmt.Sprintf("events must contain %d to %d items", minEventsPerBatch, maxEventsPerBatch))
	}
	seen := make(map[string]bool, len(b.Events))
	for _, e := range b.Events {
		if seen[e.EventID] {
			return invalid(CodeDuplicateEventIDInBatch, "duplicate event_id within one request: "+e.EventID)
		}
		seen[e.EventID] = true
	}
	return nil
}

// ValidateEvent checks one event's own fields (section 5.4). The caller
// still applies event_kind assignment (section 7) separately — that
// requires project catalog state, not just the wire shape.
func ValidateEvent(e *Event) error {
	if !isValidUUID(e.EventID) {
		return invalid(CodeInvalidUUID, "event_id must be a canonical UUIDv4")
	}
	if !isValidUUID(e.SessionID) {
		return invalid(CodeInvalidUUID, "session_id must be a canonical UUIDv4")
	}
	if e.Sequence < 0 {
		return invalid(CodeInvalidRequest, "sequence must be non-negative")
	}
	if e.SessionElapsedMs < 0 {
		return invalid(CodeInvalidRequest, "session_elapsed_ms must be non-negative")
	}
	if !eventNamePattern.MatchString(e.Name) {
		return invalid(CodeInvalidEventName, "name must be lowercase snake_case, at most 64 characters")
	}
	if len(e.Name) > 4 && e.Name[:4] == "sys_" && !ReservedSystemEvents[e.Name] {
		return invalid(CodeReservedEventName, "sys_ prefix is reserved for SDK-owned events: "+e.Name)
	}
	if !isValidTimestamp(e.OccurredAtClient) {
		return invalid(CodeInvalidTimestamp, "occurred_at_client must be RFC 3339 UTC with millisecond precision")
	}
	return validateProperties(e.Properties)
}

func validateProperties(props EventProperties) error {
	if len(props) > maxProperties {
		return invalid(CodeTooManyProperties, fmt.Sprintf("at most %d properties are allowed", maxProperties))
	}
	encodedBytes := 0
	for key, value := range props {
		if len(key) > maxPropertyKeyBytes {
			return invalid(CodeInvalidPropertyKey, "property key exceeds 64 characters: "+key)
		}
		encodedBytes += len(key)
		switch v := value.(type) {
		case nil, bool, float64:
			// finite number: encoding/json never produces Inf/NaN from
			// valid JSON text, so float64 here is already finite.
		case string:
			if utf8.RuneCountInString(v) > 0 && len(v) > maxPropertyValueBytes {
				return invalid(CodePropertyTooLarge, "property value exceeds 1024 UTF-8 bytes: "+key)
			}
			encodedBytes += len(v)
		default:
			return invalid(CodeInvalidPropertyType, "property values must be string, number, boolean, or null: "+key)
		}
	}
	if encodedBytes > maxPropertiesBytes {
		return invalid(CodePropertiesTooLarge, "encoded properties exceed 8 KiB")
	}
	return nil
}

// Validate checks the policy-probe envelope (section 5.5). It carries no
// events, so there's nothing beyond the shared identity/SDK fields.
func (p *PolicyRequest) Validate() error {
	if p.SchemaVersion != 1 {
		return invalid(CodeInvalidRequest, "schema_version must be 1")
	}
	if p.ProjectID == "" {
		return invalid(CodeInvalidRequest, "project_id is required")
	}
	if !isValidUUID(p.InstallID) {
		return invalid(CodeInvalidUUID, "install_id must be a canonical UUIDv4")
	}
	if p.SDK.Name == "" || p.SDK.Version == "" {
		return invalid(CodeInvalidRequest, "sdk.name and sdk.version are required")
	}
	return nil
}
