package httpapi

import (
	"testing"

	"github.com/ForkHorizon/Mortris/internal/analytics"
)

func TestNLQueryValues(t *testing.T) {
	name := "level_end"
	platform := "ios"
	limit := 50

	v := nlQueryValues(analytics.NLQueryParams{
		Name:     &name,
		Platform: &platform,
		Limit:    &limit,
	})

	if got := v.Get("name"); got != "level_end" {
		t.Errorf("name = %q, want level_end", got)
	}
	if got := v.Get("platform"); got != "ios" {
		t.Errorf("platform = %q, want ios", got)
	}
	if got := v.Get("limit"); got != "50" {
		t.Errorf("limit = %q, want 50", got)
	}
	if v.Has("from") || v.Has("steps") {
		t.Error("unset pointer fields must not appear in the query values")
	}
}

func TestNLQueryValuesEmptyStringOmitted(t *testing.T) {
	empty := ""
	v := nlQueryValues(analytics.NLQueryParams{Name: &empty})
	if v.Has("name") {
		t.Error("an empty-string param must be treated as unset, not sent as name=")
	}
}
