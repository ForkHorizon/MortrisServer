package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

type managedProject struct {
	ID              string     `json:"id"`
	DisplayName     string     `json:"display_name"`
	Environment     string     `json:"environment"`
	RetentionDays   int        `json:"retention_days"`
	StrictCatalog   bool       `json:"strict_catalog"`
	Enabled         bool       `json:"enabled"`
	ArchivedAt      *time.Time `json:"archived_at,omitempty"`
	SDKTestEnabled  bool       `json:"sdk_test_enabled"`
	SDKTestScenario string     `json:"sdk_test_scenario"`
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) error {
	return decodeRequestWithLimits(w, r, target, maxCompressedBody, maxDecompressedBody)
}

func decodeRequestWithLimits(w http.ResponseWriter, r *http.Request, target any, maxCompressed, maxDecompressed int64) error {
	data, err := readBodyWithLimits(w, r, maxCompressed, maxDecompressed)
	if err != nil {
		return badRequest(err)
	}
	if err := decodeJSONStrict(data, target); err != nil {
		return decodeErr(err)
	}
	return nil
}

func generatedID(prefix string) (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(bytes), nil
}

func trimmed(value *string) *string {
	if value == nil {
		return nil
	}
	result := strings.TrimSpace(*value)
	return &result
}

var projectPurgeQueries = []string{
	`DELETE FROM ingestion_stats WHERE project_id = $1`,
	`DELETE FROM daily_registration_counters WHERE project_id = $1`,
	`DELETE FROM client_policy_rules WHERE project_id = $1`,
	`DELETE FROM event_catalog WHERE project_id = $1`,
	`DELETE FROM events WHERE project_id = $1`,
	`DELETE FROM installations WHERE project_id = $1`,
	`DELETE FROM admin_user_projects WHERE project_id = $1`,
	`DELETE FROM admin_audit_log WHERE detail->>'project_id' = $1`,
	`DELETE FROM projects WHERE id = $1`,
}
