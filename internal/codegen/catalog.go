// Package codegen turns events/catalog.yaml — the source of truth for
// event names described in the file's own header comment — into
// generated client code, so a typo in an event or property name becomes
// a client build error instead of a silent server-side rejection.
package codegen

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Catalog struct {
	Version int        `yaml:"version"`
	Events  []EventDef `yaml:"events"`
}

type EventDef struct {
	Name               string        `yaml:"name"`
	Kind               string        `yaml:"kind"`
	Description        string        `yaml:"description"`
	Owner              string        `yaml:"owner"`
	FirstSchemaVersion int           `yaml:"first_schema_version"`
	Properties         []PropertyDef `yaml:"properties"`
}

type PropertyDef struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description"`
}

func LoadCatalog(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var catalog Catalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}
	return &catalog, nil
}
