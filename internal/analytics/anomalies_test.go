package analytics

import (
	"math"
	"testing"
)

func TestMedianOf(t *testing.T) {
	cases := []struct {
		values []float64
		want   float64
	}{
		{[]float64{1, 2, 3}, 2},
		{[]float64{1, 2, 3, 4}, 2.5},
		{[]float64{}, 0},
		{[]float64{5}, 5},
	}
	for _, c := range cases {
		if got := medianOf(c.values); got != c.want {
			t.Errorf("medianOf(%v) = %v, want %v", c.values, got, c.want)
		}
	}
}

func TestModifiedZScore(t *testing.T) {
	// Steady baseline (median 19, small MAD), today drops to 2: should
	// be a large-magnitude, negative anomaly.
	baseline := []float64{18, 19, 19, 20, 19, 18, 20, 19, 21, 19, 18, 20, 19, 19}
	median := medianOf(baseline)
	mad := medianAbsDeviation(baseline, median)
	z := modifiedZScore(2, median, mad)
	if z >= -anomalyZThreshold {
		t.Fatalf("expected a strong negative anomaly, got z=%v (median=%v mad=%v)", z, median, mad)
	}

	// Today matches the baseline: no anomaly.
	zSame := modifiedZScore(19, median, mad)
	if math.Abs(zSame) > anomalyZThreshold {
		t.Fatalf("expected no anomaly for a typical day, got z=%v", zSame)
	}

	// Flat baseline (mad == 0): any deviation is flagged rather than
	// dividing by zero.
	flat := []float64{5, 5, 5, 5}
	flatMedian := medianOf(flat)
	flatMAD := medianAbsDeviation(flat, flatMedian)
	if flatMAD != 0 {
		t.Fatalf("expected mad=0 for a flat baseline, got %v", flatMAD)
	}
	if z := modifiedZScore(0, flatMedian, flatMAD); math.Abs(z) <= anomalyZThreshold {
		t.Fatalf("expected a flat baseline dropping to 0 to be flagged, got z=%v", z)
	}
	if z := modifiedZScore(5, flatMedian, flatMAD); z != 0 {
		t.Fatalf("expected no anomaly when today matches a flat baseline, got z=%v", z)
	}
}
