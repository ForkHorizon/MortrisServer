package codegen

import "testing"

func TestLoadCatalog(t *testing.T) {
	catalog, err := LoadCatalog("../../events/catalog.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Version != 1 {
		t.Errorf("expected version 1, got %d", catalog.Version)
	}
	if len(catalog.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(catalog.Events))
	}
	health := catalog.Events[2]
	if health.Name != "sys_sdk_health" {
		t.Fatalf("expected sys_sdk_health, got %s", health.Name)
	}
	if len(health.Properties) != 6 {
		t.Errorf("expected 6 properties on sys_sdk_health, got %d", len(health.Properties))
	}
}
