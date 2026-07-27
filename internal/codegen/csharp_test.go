package codegen

import (
	"strings"
	"testing"
)

func TestGenerateCSharp(t *testing.T) {
	catalog := &Catalog{
		Version: 1,
		Events: []EventDef{
			{Name: "sys_session_start", Kind: "system", Description: "session start"},
			{
				Name:        "level_end",
				Kind:        "product",
				Description: "level finished",
				Properties: []PropertyDef{
					{Name: "level_id", Type: "string", Required: true},
					{Name: "score", Type: "number", Required: true},
					{Name: "bonus_applied", Type: "boolean", Required: false},
				},
			},
		},
	}

	source := GenerateCSharp(catalog)

	wantContains := []string{
		`public const string SysSessionStart = "sys_session_start";`,
		`public const string LevelEnd = "level_end";`,
		`public static Dictionary<string, object> BuildLevelEnd(string levelId, double score, bool? bonusApplied = null)`,
		`properties["level_id"] = levelId;`,
		`properties["score"] = score;`,
		`if (bonusApplied != null) properties["bonus_applied"] = bonusApplied;`,
	}
	for _, want := range wantContains {
		if !strings.Contains(source, want) {
			t.Errorf("generated source missing %q\n---\n%s", want, source)
		}
	}

	// sys_session_start has no properties, so it must not get a builder.
	if strings.Contains(source, "BuildSysSessionStart") {
		t.Error("expected no builder for an event with no declared properties")
	}
}

func TestIdentifierCasing(t *testing.T) {
	cases := []struct {
		in, wantPascal, wantCamel string
	}{
		{"sys_session_start", "SysSessionStart", "sysSessionStart"},
		{"handoff_drops", "HandoffDrops", "handoffDrops"},
		{"score", "Score", "score"},
	}
	for _, c := range cases {
		if got := pascalCase(c.in); got != c.wantPascal {
			t.Errorf("pascalCase(%q) = %q, want %q", c.in, got, c.wantPascal)
		}
		if got := camelCase(c.in); got != c.wantCamel {
			t.Errorf("camelCase(%q) = %q, want %q", c.in, got, c.wantCamel)
		}
	}
}
