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
