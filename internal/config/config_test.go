package config

import "testing"

func TestValidateSDKTest(t *testing.T) {
	tests := []struct {
		name string
		cfg  SDKTestConfig
		want bool
	}{
		{"disabled", SDKTestConfig{}, true},
		{"production blocked", SDKTestConfig{Enabled: true, Deployment: "production", ProjectID: "sdk-test", Token: "test-token-123456"}, false},
		{"missing project", SDKTestConfig{Enabled: true, Deployment: "staging", Token: "test-token-123456"}, false},
		{"short token", SDKTestConfig{Enabled: true, Deployment: "staging", ProjectID: "sdk-test", Token: "short"}, false},
		{"staging enabled", SDKTestConfig{Enabled: true, Deployment: "staging", ProjectID: "sdk-test", Token: "test-token-123456"}, true},
	}
	for _, tt := range tests {
		err := (Config{SDKTest: tt.cfg}).ValidateSDKTest()
		if (err == nil) != tt.want {
			t.Fatalf("%s: ValidateSDKTest() error = %v", tt.name, err)
		}
	}
}
