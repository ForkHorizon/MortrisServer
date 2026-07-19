// Package contracts holds the wire types shared by the Go service and the
// Unity SDK's contract tests. Both sides validate fixtures under
// contracts/fixtures against these types and Validate methods, so the two
// implementations cannot silently drift from server_implementation_plan.md
// section 5.
package contracts

// ClientPolicy is embedded in every registration, ingestion, and policy
// response (section 5.5).
type ClientPolicy struct {
	Mode             string `json:"mode"`
	NextCheckSeconds int64  `json:"next_check_seconds"`
	DiscardPending   bool   `json:"discard_pending"`
}

const (
	PolicyModeActive            = "active"
	PolicyModePauseUpload       = "pause_upload"
	PolicyModeDisableCollection = "disable_collection"
)

// RegisterRequest is the body of POST /v1/installs/register (section 5.2).
type RegisterRequest struct {
	SchemaVersion          int    `json:"schema_version"`
	ProjectID              string `json:"project_id"`
	InstallID              string `json:"install_id"`
	InstallationCredential string `json:"installation_credential"`
	SDKName                string `json:"sdk_name"`
	SDKVersion             string `json:"sdk_version"`
	AppVersion             string `json:"app_version"`
	BuildNumber            string `json:"build_number"`
	Platform               string `json:"platform"`
}

type RegisterResponse struct {
	ServerTime         string       `json:"server_time"`
	InstallationStatus string       `json:"installation_status"`
	ClientPolicy       ClientPolicy `json:"client_policy"`
}

// EventProperties is a flat property bag: string, finite number, bool, or
// null values only (section 5.4). json.Number is used so integers and
// floats both decode without losing the "is it finite" check to float64
// rounding, and so we can distinguish numbers from bare strings.
type EventProperties map[string]any

// SDKInfo identifies the sending SDK in a batch envelope.
type SDKInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Event is one item inside a batch ingestion request (section 5.4).
type Event struct {
	EventID               string          `json:"event_id"`
	SessionID             string          `json:"session_id"`
	Sequence              int64           `json:"sequence"`
	SessionElapsedMs      int64           `json:"session_elapsed_ms"`
	Name                  string          `json:"name"`
	OccurredAtClient      string          `json:"occurred_at_client"`
	AppVersion            string          `json:"app_version"`
	BuildNumber           string          `json:"build_number"`
	Platform              string          `json:"platform"`
	OSVersion             string          `json:"os_version"`
	DeviceClass           string          `json:"device_class"`
	Locale                string          `json:"locale"`
	TimezoneOffsetMinutes int             `json:"timezone_offset_minutes"`
	Properties            EventProperties `json:"properties"`
}

// BatchIngestRequest is the body of POST /v1/events/batch (section 5.4).
type BatchIngestRequest struct {
	SchemaVersion int     `json:"schema_version"`
	ProjectID     string  `json:"project_id"`
	InstallID     string  `json:"install_id"`
	SDK           SDKInfo `json:"sdk"`
	SentAtClient  string  `json:"sent_at_client"`
	Events        []Event `json:"events"`
	// EventCount preserves the original envelope size while Events contains
	// only successfully decoded entries. It is not part of the wire format.
	EventCount int `json:"-"`
}

type RejectedEvent struct {
	EventID string `json:"event_id"`
	Code    string `json:"code"`
}

type BatchIngestResponse struct {
	ServerTime   string          `json:"server_time"`
	Accepted     []string        `json:"accepted"`
	Duplicates   []string        `json:"duplicates"`
	Rejected     []RejectedEvent `json:"rejected"`
	ClientPolicy ClientPolicy    `json:"client_policy"`
}

// PolicyRequest is the body of POST /v1/client/policy (section 5.5). It
// carries no events.
type PolicyRequest struct {
	SchemaVersion int     `json:"schema_version"`
	ProjectID     string  `json:"project_id"`
	InstallID     string  `json:"install_id"`
	SDK           SDKInfo `json:"sdk"`
	AppVersion    string  `json:"app_version"`
	BuildNumber   string  `json:"build_number"`
	Platform      string  `json:"platform"`
}

type PolicyResponse struct {
	ServerTime   string       `json:"server_time"`
	ClientPolicy ClientPolicy `json:"client_policy"`
}

// ErrorResponse is the stable shape of every non-2xx response (section 5.1).
type ErrorResponse struct {
	ServerTime string `json:"server_time"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id"`
}
