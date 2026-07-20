package sdktest

import (
	"net/http/httptest"
	"testing"
)

func TestScenarioRequiresMatchingProjectAndToken(t *testing.T) {
	controller := New("sdk-test", "test-token-123456")
	req := httptest.NewRequest("POST", "/v1/events/batch", nil)
	req.Header.Set(HeaderScenario, string(RateLimited))
	req.Header.Set(HeaderToken, "test-token-123456")
	if got := controller.Scenario(req, "sdk-test"); got != RateLimited {
		t.Fatalf("Scenario = %q, want %q", got, RateLimited)
	}
	if got := controller.Scenario(req, "other-project"); got != "" {
		t.Fatalf("wrong project scenario = %q, want empty", got)
	}
	req.Header.Set(HeaderToken, "wrong-token-12345")
	if got := controller.Scenario(req, "sdk-test"); got != "" {
		t.Fatalf("wrong token scenario = %q, want empty", got)
	}
}

func TestUnauthorizedScenarioOnlyFailsOncePerInstallation(t *testing.T) {
	controller := New("sdk-test", "test-token-123456")
	if !controller.FirstUnauthorized("sdk-test", "install-a", UnauthorizedOnce) {
		t.Fatal("first attempt must fail")
	}
	if controller.FirstUnauthorized("sdk-test", "install-a", UnauthorizedOnce) {
		t.Fatal("second attempt must not fail")
	}
}

func TestPolicyScenarios(t *testing.T) {
	for scenario, mode := range map[Scenario]string{PolicyActive: "active", PolicyPauseUpload: "pause_upload", PolicyDisable: "disable_collection"} {
		policy, ok := scenario.Policy()
		if !ok || policy.Mode != mode {
			t.Fatalf("%s policy = %#v, %v", scenario, policy, ok)
		}
	}
}
