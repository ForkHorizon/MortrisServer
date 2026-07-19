// Package fixtures loads every JSON fixture under contracts/fixtures and
// checks it against internal/contracts, so the Unity SDK team (and anyone
// else implementing the wire contract independently) has a working
// reference for exactly which requests are valid and which error code an
// invalid one produces (Phase S0 exit gate).
package fixtures

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ForkHorizon/Mortris/internal/contracts"
)

type perEventExpectation struct {
	Index  int    `json:"index"`
	Result string `json:"result"`
	Code   string `json:"code,omitempty"`
}

type expectation struct {
	Envelope string                `json:"envelope"`
	Code     string                `json:"code"`
	PerEvent []perEventExpectation `json:"perEvent"`
}

type manifestEntry struct {
	File   string          `json:"file"`
	Kind   string          `json:"kind"`
	Expect json.RawMessage `json:"expect"`
}

type manifestFile struct {
	Fixtures []manifestEntry `json:"fixtures"`
}

func loadManifest(t *testing.T) manifestFile {
	t.Helper()
	data, err := os.ReadFile("fixtures/manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m manifestFile
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

// TestManifestCoversEveryFixture guards against a fixture file being added
// without being wired into the manifest, which would silently drop it out
// of coverage.
func TestManifestCoversEveryFixture(t *testing.T) {
	m := loadManifest(t)
	listed := make(map[string]bool, len(m.Fixtures))
	for _, e := range m.Fixtures {
		listed[e.File] = true
	}

	entries, err := os.ReadDir("fixtures")
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "manifest.json" || filepath.Ext(name) != ".json" {
			continue
		}
		if !listed[name] {
			t.Errorf("fixtures/%s exists but is not listed in manifest.json", name)
		}
	}
}

func TestFixtures(t *testing.T) {
	m := loadManifest(t)
	for _, entry := range m.Fixtures {
		entry := entry
		t.Run(entry.File, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("fixtures", entry.File))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			isValid, valErr := isPlainValid(entry.Expect)
			if valErr != nil {
				t.Fatalf("parse expect: %v", valErr)
			}

			switch entry.Kind {
			case "register":
				checkRegister(t, data, isValid, entry.Expect)
			case "batch":
				checkBatch(t, data, isValid, entry.Expect)
			default:
				t.Fatalf("unknown fixture kind %q", entry.Kind)
			}
		})
	}
}

func isPlainValid(raw json.RawMessage) (bool, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s == "valid", nil
	}
	return false, nil
}

func checkRegister(t *testing.T, data []byte, wantValid bool, raw json.RawMessage) {
	t.Helper()
	req, decodeErr := contracts.DecodeRegisterRequest(data)

	if wantValid {
		if decodeErr != nil {
			t.Fatalf("expected valid fixture to decode, got: %v", decodeErr)
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("expected valid fixture to pass Validate, got: %v", err)
		}
		return
	}

	var exp expectation
	if err := json.Unmarshal(raw, &exp); err != nil {
		t.Fatalf("parse expect: %v", err)
	}
	if exp.Envelope != "invalid" {
		t.Fatalf("register fixture expectation must be \"valid\" or {envelope:invalid,...}, got %s", raw)
	}

	var gotCode string
	if decodeErr != nil {
		gotCode = classifyDecodeError(decodeErr)
	} else if verr := req.Validate(); verr != nil {
		gotCode = verr.(*contracts.ValidationError).Code
	} else {
		t.Fatalf("expected fixture to be invalid, but it decoded and validated cleanly")
	}
	if gotCode != exp.Code {
		t.Errorf("expected error code %q, got %q", exp.Code, gotCode)
	}
}

// classifyDecodeError mirrors internal/contracts' unexported
// decodeErrorCode so the test can assert on it without exporting an
// internal-only helper just for tests.
func classifyDecodeError(err error) string {
	if validationErr, ok := err.(*contracts.ValidationError); ok {
		return validationErr.Code
	}
	if strings.Contains(err.Error(), "unknown field") {
		return contracts.CodeUnknownField
	}
	return contracts.CodeInvalidRequest
}
