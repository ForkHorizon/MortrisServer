package contracts

// Stable machine error codes (section 5.1, 5.2, 5.4, 12). docs/errors.md
// mirrors this list with HTTP status and retry classification — keep both
// in sync by hand, there are few enough codes that generating the doc isn't
// worth the indirection.
const (
	CodeInstallConflict         = "install_conflict"
	CodeInvalidRequest          = "invalid_request"
	CodeUnknownField            = "unknown_field"
	CodeInvalidCredential       = "invalid_credential"
	CodeInvalidUUID             = "invalid_uuid"
	CodeInvalidTimestamp        = "invalid_timestamp"
	CodeInvalidBatchSize        = "invalid_batch_size"
	CodeDuplicateEventIDInBatch = "duplicate_event_id_in_batch"
	CodeInvalidEventName        = "invalid_event_name"
	CodeReservedEventName       = "reserved_event_name"
	CodeTooManyProperties       = "too_many_properties"
	CodeInvalidPropertyKey      = "invalid_property_key"
	CodeInvalidPropertyType     = "invalid_property_type"
	CodePropertyTooLarge        = "property_too_large"
	CodePropertiesTooLarge      = "properties_too_large"
	CodeUnauthorized            = "unauthorized"
	CodeRateLimited             = "rate_limited"
	CodeServerStoragePressure   = "server_storage_pressure"
)

// ValidationError reports one structural contract violation. For batch
// ingestion, an error on one event's fields never surfaces here — it goes
// in that event's RejectedEvent entry instead, per the "valid siblings
// still commit" rule (section 5.4).
type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func invalid(code, message string) *ValidationError {
	return &ValidationError{Code: code, Message: message}
}
