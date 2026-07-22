package analytics

import "testing"

func testCatalog() PuzzleCatalog {
	return PuzzleCatalog{
		SchemaVersion: 1, ContentRevision: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Cities: []PuzzleCatalogCity{{CityID: 1, Houses: []PuzzleCatalogHouse{{
			HouseID:         2,
			Blocks:          []PuzzleCatalogBlock{{BlockID: 10}, {BlockID: 11}},
			Waves:           []PuzzleCatalogWave{{WaveIndex: 0, BlockIDs: []int{10, 11}}},
			InventoryGroups: []PuzzleInventoryGroup{{InventoryGroupID: "2:10-11", BlockIDs: []int{10, 11}}},
			Targets:         []PuzzleCatalogTarget{{TargetID: 10, WaveIndex: 0, CompatibleBlockIDs: []int{10}}},
			PlacementRules:  []PuzzlePlacementRule{{TargetID: 10, AlternativeRequiredGroups: [][]int{{11}, {10, 11}}}},
		}}}},
	}
}

func TestPuzzleCatalogValidationRejectsBrokenReferences(t *testing.T) {
	catalog := testCatalog()
	catalog.Cities[0].Houses[0].Targets[0].CompatibleBlockIDs = []int{999}
	if err := catalog.Validate(); err == nil {
		t.Fatal("Validate accepted unknown compatible block")
	}
}

func TestPuzzleMissingGroupsKeepsAlternativesSeparate(t *testing.T) {
	missing := missingGroups(testCatalog(), 1, 2, 10, map[int]bool{10: true})
	if len(missing) != 2 || len(missing[0]) != 1 || missing[0][0] != 11 || len(missing[1]) != 1 || missing[1][0] != 11 {
		t.Fatalf("missing groups = %#v", missing)
	}
}
