package analytics

import (
	"context"
	"testing"
)

func TestGetDimensions_DistinctPlatformsAndVersions(t *testing.T) {
	pool := testPool(t)
	projectID, _ := seedRecentEventsFixture(t, pool)

	result, err := GetDimensions(context.Background(), pool, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Platforms) != 2 || result.Platforms[0] != "android" || result.Platforms[1] != "ios" {
		t.Errorf("expected [android ios], got %v", result.Platforms)
	}
	if len(result.AppVersions) == 0 {
		t.Errorf("expected at least one app_version, got none")
	}
}

func TestGetDimensions_EmptyProjectReturnsEmptyNotNull(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)

	result, err := GetDimensions(context.Background(), pool, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Platforms == nil || result.AppVersions == nil {
		t.Errorf("expected non-nil empty slices, got %+v", result)
	}
}
