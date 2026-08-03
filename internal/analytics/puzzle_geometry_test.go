package analytics

import "testing"

// testGeometry describes block 10 of the house testCatalog() builds, as a
// drawable square, so each case below can break exactly one invariant.
func testGeometry() PuzzleGeometry {
	return PuzzleGeometry{
		SchemaVersion:   1,
		ContentRevision: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Blocks: []PuzzleBlockGeometry{{
			CityID: 1, HouseID: 2, BlockID: 10,
			Bounds:  PuzzleBlockBounds{MinX: -100, MinY: 0, MaxX: 100, MaxY: 200},
			Outline: []PuzzlePoint{{-100, 0}, {100, 0}, {100, 200}, {-100, 200}},
		}},
	}
}

func TestGeometryValidates(t *testing.T) {
	if err := testGeometry().Validate(); err != nil {
		t.Fatalf("Validate rejected well-formed geometry: %v", err)
	}
}

// Bounds alone are enough to draw a rectangle. Only the silhouette is
// optional, so a block without an outline must still validate.
func TestGeometryOutlineIsOptional(t *testing.T) {
	geometry := testGeometry()
	geometry.Blocks[0].Outline = nil
	if err := geometry.Validate(); err != nil {
		t.Fatalf("Validate rejected bounds without an outline: %v", err)
	}
}

// The catalogue itself must never carry shapes — that would change
// content_revision and orphan every event already collected against the
// old one. This pins the separation.
func TestCatalogValidatesWithoutGeometry(t *testing.T) {
	if err := testCatalog().Validate(); err != nil {
		t.Fatalf("Validate rejected a geometry-free catalogue: %v", err)
	}
}

func TestGeometryRejectsUndrawableShapes(t *testing.T) {
	tests := map[string]func(*PuzzleGeometry){
		"inverted bounds": func(g *PuzzleGeometry) {
			b := &g.Blocks[0].Bounds
			b.MinX, b.MaxX = b.MaxX, b.MinX
		},
		"degenerate bounds": func(g *PuzzleGeometry) { g.Blocks[0].Bounds.MaxY = g.Blocks[0].Bounds.MinY },
		"too few points":    func(g *PuzzleGeometry) { g.Blocks[0].Outline = g.Blocks[0].Outline[:2] },
		"point outside bounds": func(g *PuzzleGeometry) {
			g.Blocks[0].Outline[2] = PuzzlePoint{100, 201}
		},
		"too many points": func(g *PuzzleGeometry) {
			g.Blocks[0].Outline = make([]PuzzlePoint, maxOutlinePoints+1)
		},
		"no blocks":       func(g *PuzzleGeometry) { g.Blocks = nil },
		"bad revision":    func(g *PuzzleGeometry) { g.ContentRevision = "not-a-sha" },
		"wrong schema":    func(g *PuzzleGeometry) { g.SchemaVersion = 2 },
		"duplicate block": func(g *PuzzleGeometry) { g.Blocks = append(g.Blocks, g.Blocks[0]) },
	}
	for name, breakIt := range tests {
		t.Run(name, func(t *testing.T) {
			geometry := testGeometry()
			breakIt(&geometry)
			if err := geometry.Validate(); err == nil {
				t.Fatalf("Validate accepted %s", name)
			}
		})
	}
}

func TestOutlineEncodesAsCompactPairs(t *testing.T) {
	encoded := outlineJSON(testGeometry().Blocks[0].Outline)
	if encoded == nil {
		t.Fatal("outlineJSON returned NULL for a present outline")
	}
	if want := `[[-100,0],[100,0],[100,200],[-100,200]]`; *encoded != want {
		t.Fatalf("outline JSON = %s, want %s", *encoded, want)
	}
	if outlineJSON(nil) != nil {
		t.Fatal("outlineJSON should be NULL when there is no outline")
	}
}
