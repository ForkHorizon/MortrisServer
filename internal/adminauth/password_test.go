package adminauth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	encoded, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword(encoded, "correct horse battery staple") {
		t.Error("expected correct password to verify")
	}
	if VerifyPassword(encoded, "wrong password") {
		t.Error("expected wrong password to fail verification")
	}
	if VerifyPassword("not-a-valid-hash", "anything") {
		t.Error("expected malformed hash to fail verification, not error out")
	}
}

func TestHashPasswordUsesUniqueSalt(t *testing.T) {
	a, err := HashPassword("same password")
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword("same password")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("expected two hashes of the same password to differ (unique salt)")
	}
}
