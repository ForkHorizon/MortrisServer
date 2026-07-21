package httpapi

import (
	"strings"
	"testing"

	"github.com/ForkHorizon/Mortris/internal/sdktest"
)

func TestGeneratedIDIsOpaqueAndUnique(t *testing.T) {
	first, err := generatedID("project")
	if err != nil {
		t.Fatal(err)
	}
	second, err := generatedID("project")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first, "project_") || first == second {
		t.Fatalf("generated IDs must be opaque and unique: %q, %q", first, second)
	}
}

func TestKnownSDKScenarioRejectsUnknownControls(t *testing.T) {
	if !knownSDKScenario(sdktest.RateLimited) {
		t.Fatal("expected documented SDK test scenario to be accepted")
	}
	if knownSDKScenario("production_fault") {
		t.Fatal("unknown fault controls must not be accepted")
	}
}
