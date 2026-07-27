package analytics

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGetDigestRequiresClient(t *testing.T) {
	_, err := GetDigest(context.Background(), nil, nil, "proj", time.UTC)
	if !errors.Is(err, ErrAIUnavailable) {
		t.Fatalf("expected ErrAIUnavailable, got %v", err)
	}
}
