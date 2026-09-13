package analytics

import "testing"

func placedDrop(release, target int) PuzzleDrop {
	return PuzzleDrop{Outcome: "placed", TargetID: 1, ReleaseX: release, ReleaseY: release, TargetX: target, TargetY: target, Legacy: true}
}

// The client sends world coordinates while the layout is house-local, so
// a house sitting at a world offset must be shifted back onto its own
// blocks before anything is plotted.
func TestAlignDropsRecoversHouseOffset(t *testing.T) {
	const offset = 11170
	result := &PuzzleDropMap{}
	for i := 0; i < 8; i++ {
		// Releases scatter within the snap radius around the target.
		result.Drops = append(result.Drops, placedDrop(1000+offset+(i-4)*100, 1000))
	}
	alignDrops(result)
	if !result.Aligned {
		t.Fatal("alignDrops refused a tight cluster it should have accepted")
	}
	if result.OffsetX != offset {
		t.Fatalf("OffsetX = %d, want %d", result.OffsetX, offset)
	}
	if got := result.Drops[4].ReleaseX; got != 1000 {
		t.Fatalf("aligned release = %d, want it back on the target at 1000", got)
	}
}

// A map drawn from an untrustworthy offset is worse than no map: it looks
// authoritative and points at the wrong details.
func TestAlignDropsRefusesWhenSpreadTooWide(t *testing.T) {
	result := &PuzzleDropMap{}
	for i := 0; i < 8; i++ {
		result.Drops = append(result.Drops, placedDrop(1000+i*9000, 1000))
	}
	alignDrops(result)
	if result.Aligned {
		t.Fatal("alignDrops trusted an offset spread far wider than the snap radius")
	}
	if len(result.Drops) != 0 {
		t.Fatalf("kept %d drops it could not align", len(result.Drops))
	}
	if result.AlignmentIssue != "inconsistent" {
		t.Fatalf("AlignmentIssue = %q, want \"inconsistent\"", result.AlignmentIssue)
	}
}

func TestAlignDropsNeedsEnoughPlacedDrops(t *testing.T) {
	result := &PuzzleDropMap{Drops: []PuzzleDrop{placedDrop(1000, 1000), placedDrop(1010, 1000)}}
	alignDrops(result)
	if result.Aligned || len(result.Drops) != 0 {
		t.Fatal("alignDrops estimated an offset from too few placed drops")
	}
	// Distinct from "inconsistent": this one resolves itself with more play.
	if result.AlignmentIssue != "too_few_placed" {
		t.Fatalf("AlignmentIssue = %q, want \"too_few_placed\"", result.AlignmentIssue)
	}
}

// Falls are the interesting drops, but they cannot anchor the estimate —
// only placed drops are known to be near their target.
func TestAlignDropsShiftsFallsUsingPlacedAnchor(t *testing.T) {
	const offset = 5000
	result := &PuzzleDropMap{}
	for i := 0; i < 6; i++ {
		result.Drops = append(result.Drops, placedDrop(2000+offset, 2000))
	}
	result.Drops = append(result.Drops, PuzzleDrop{Outcome: "fell_missing_support", TargetID: 2, ReleaseX: 9000 + offset, ReleaseY: 9000 + offset, TargetX: 9000, TargetY: 9000, Legacy: true})
	alignDrops(result)
	fall := result.Drops[len(result.Drops)-1]
	if fall.ReleaseX != 9000 {
		t.Fatalf("fall release = %d, want 9000 after the placed-anchored shift", fall.ReleaseX)
	}
}

func TestAlignDropsKeepsCorrectedHouseLocalCoordinates(t *testing.T) {
	result := &PuzzleDropMap{Drops: []PuzzleDrop{{Outcome: "placed", TargetID: 1, ReleaseX: 1234, ReleaseY: 5678, TargetX: 1200, TargetY: 5600}}}
	alignDrops(result)
	if !result.Aligned || result.OffsetX != 0 || result.Drops[0].ReleaseX != 1234 {
		t.Fatalf("corrected coordinates were unexpectedly fitted: %+v", result)
	}
}

func TestPreserveNegativeCoordinatesDirectHouseLocal(t *testing.T) {
	// Schema v2 house_local coordinates can be negative and must not be shifted or filtered out.
	candX, candY := -400, -650
	drop := PuzzleDrop{
		BlockID:                   5,
		CandidateTargetID:         12,
		CandidateTargetX:          &candX,
		CandidateTargetY:          &candY,
		NearestCompatibleTargetID: 12,
		NearestTargetX:            &candX,
		NearestTargetY:            &candY,
		ReleaseX:                  -450,
		ReleaseY:                  -680,
		Outcome:                   "placed",
		AttemptID:                 "att-neg-1",
		Legacy:                    false,
	}
	result := &PuzzleDropMap{Drops: []PuzzleDrop{drop}}
	alignDrops(result)

	if !result.Aligned {
		t.Fatal("expected aligned = true for house_local schema v2 drops")
	}
	if !result.TrustedCoordinates {
		t.Fatal("expected trusted_coordinates = true")
	}
	if result.OffsetX != 0 || result.OffsetY != 0 {
		t.Fatalf("expected 0 offset for house_local, got dx=%d, dy=%d", result.OffsetX, result.OffsetY)
	}
	if result.Drops[0].ReleaseX != -450 || result.Drops[0].ReleaseY != -680 {
		t.Fatalf("negative coordinates altered: got (%d, %d), want (-450, -680)", result.Drops[0].ReleaseX, result.Drops[0].ReleaseY)
	}
}

func TestComputeDropDistanceEuclidean(t *testing.T) {
	nearX, nearY := 130, 240
	drop := PuzzleDrop{ReleaseX: 100, ReleaseY: 200}
	computeDropDistance(&drop, &nearX, &nearY, nil)
	if drop.NearestDistance == nil || *drop.NearestDistance != 50 {
		t.Fatalf("NearestDistance = %v, want 50 (30-40-50 triangle)", drop.NearestDistance)
	}

	// Negative coordinates Euclidean distance: dx = -300, dy = 400 -> dist = 500
	negNearX, negNearY := -200, -600
	negDrop := PuzzleDrop{ReleaseX: -500, ReleaseY: -200}
	computeDropDistance(&negDrop, &negNearX, &negNearY, nil)
	if negDrop.NearestDistance == nil || *negDrop.NearestDistance != 500 {
		t.Fatalf("Negative coords NearestDistance = %v, want 500", negDrop.NearestDistance)
	}

	// Fallback to client distance if target coords not resolved in layout
	clientDist := 75
	fallbackDrop := PuzzleDrop{ReleaseX: 10, ReleaseY: 10}
	computeDropDistance(&fallbackDrop, nil, nil, &clientDist)
	if fallbackDrop.NearestDistance == nil || *fallbackDrop.NearestDistance != 75 {
		t.Fatalf("Fallback NearestDistance = %v, want 75", fallbackDrop.NearestDistance)
	}
}

func TestResolveDropTargetCandidate(t *testing.T) {
	candX, candY := 100, 200
	nearX, nearY := 150, 250

	// 1. Candidate target present and resolved in layout
	drop1 := PuzzleDrop{CandidateTargetID: 10, NearestCompatibleTargetID: 12}
	if !resolveDropTarget(&drop1, &candX, &candY, &nearX, &nearY) {
		t.Fatal("expected drop1 target resolved")
	}
	if drop1.TargetID != 10 || drop1.TargetX != 100 || drop1.TargetY != 200 {
		t.Fatalf("drop1 target mismatch: %+v", drop1)
	}

	// 2. Candidate target >= 0 but missing in layout catalog -> unresolved
	drop3 := PuzzleDrop{CandidateTargetID: 99, NearestCompatibleTargetID: -1}
	if resolveDropTarget(&drop3, nil, nil, nil, nil) {
		t.Fatal("expected drop3 to fail resolution (unresolved target)")
	}

	// 3. Candidate target >= 0 missing in layout catalog fails resolution even if nearest target is in layout
	drop3b := PuzzleDrop{CandidateTargetID: 99, NearestCompatibleTargetID: 12}
	if resolveDropTarget(&drop3b, nil, nil, &nearX, &nearY) {
		t.Fatal("expected drop3b to fail resolution when candidate target is missing from layout")
	}
}

func TestResolveDropTargetNearestFallback(t *testing.T) {
	nearX, nearY := 150, 250

	// 1. Candidate target is -1 (no snap candidate), falls back to nearest compatible target
	drop2 := PuzzleDrop{CandidateTargetID: -1, NearestCompatibleTargetID: 12}
	if !resolveDropTarget(&drop2, nil, nil, &nearX, &nearY) {
		t.Fatal("expected drop2 target resolved via nearest compatible")
	}
	if drop2.TargetID != 12 || drop2.TargetX != 150 || drop2.TargetY != 250 {
		t.Fatalf("drop2 fallback mismatch: %+v", drop2)
	}

	// 2. Drop with neither target specified (-1, -1) -> valid drop without aim vector
	drop4 := PuzzleDrop{CandidateTargetID: -1, NearestCompatibleTargetID: -1}
	if !resolveDropTarget(&drop4, nil, nil, nil, nil) {
		t.Fatal("expected drop4 to resolve without target")
	}
	if drop4.TargetID != -1 {
		t.Fatalf("expected TargetID -1, got %d", drop4.TargetID)
	}

	// 3. CandidateTargetID is -1 and nearest target not in layout -> valid drop with TargetID -1
	drop5 := PuzzleDrop{CandidateTargetID: -1, NearestCompatibleTargetID: 42}
	if !resolveDropTarget(&drop5, nil, nil, nil, nil) {
		t.Fatal("expected drop5 to resolve with TargetID -1")
	}
	if drop5.TargetID != -1 {
		t.Fatalf("expected drop5 TargetID -1, got %d", drop5.TargetID)
	}
}

func TestAlignDropsRecalculatesLegacyNearestDistance(t *testing.T) {
	// 5 placed drops establishing offset (1000, 2000)
	drops := make([]PuzzleDrop, 5)
	for i := 0; i < 5; i++ {
		targetX, targetY := 100, 200
		drops[i] = PuzzleDrop{
			Outcome:        "placed",
			TargetID:       i,
			ReleaseX:       1100, // World coordinate: 100 + 1000
			ReleaseY:       2200, // World coordinate: 200 + 2000
			TargetX:        100,
			TargetY:        200,
			NearestTargetX: &targetX,
			NearestTargetY: &targetY,
			Legacy:         true,
		}
		// Initially computed using raw world coords before alignment:
		computeDropDistance(&drops[i], &targetX, &targetY, nil)
	}

	dropMap := &PuzzleDropMap{Drops: drops}
	alignDrops(dropMap)

	if !dropMap.Aligned || dropMap.OffsetX != 1000 || dropMap.OffsetY != 2000 {
		t.Fatalf("expected offset 1000,2000, got %d,%d", dropMap.OffsetX, dropMap.OffsetY)
	}
	// After alignment, release coords are (100, 200), distance to (100, 200) should be 0!
	for i, d := range dropMap.Drops {
		if d.NearestDistance == nil || *d.NearestDistance != 0 {
			t.Fatalf("drop[%d] post-alignment distance = %v, want 0", i, d.NearestDistance)
		}
	}
}
