package contracts

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidatePropertiesUsesEncodedJSONSize(t *testing.T) {
	properties := EventProperties{}
	for i := 0; i < 8; i++ {
		properties[fmt.Sprintf("x%d", i)] = strings.Repeat("a", 1019)
	}
	err := validateProperties(properties)
	if err == nil {
		t.Fatal("oversized encoded JSON was accepted")
	}
	if got := err.(*ValidationError).Code; got != CodePropertiesTooLarge {
		t.Fatalf("code = %q, want %q", got, CodePropertiesTooLarge)
	}
}
