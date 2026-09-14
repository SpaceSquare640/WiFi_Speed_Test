package grade

import "testing"

func TestEvaluateBands(t *testing.T) {
	cases := map[float64]Grade{
		940:  GradeExcellent,
		500:  GradeExcellent, // the boundary belongs to the band above it
		499:  GradeVeryGood,
		200:  GradeVeryGood,
		150:  GradeGood,
		100:  GradeGood,
		99.9: GradeFair,
		50:   GradeFair,
		49:   GradeSlow,
		0:    GradeSlow,
		-1:   GradeSlow, // a measured-but-broken link is genuinely slow
	}
	for mbps, want := range cases {
		if got := Evaluate(mbps); got != want {
			t.Errorf("Evaluate(%v) = %q, want %q", mbps, got, want)
		}
	}
}

// An unmeasured rate has no band. Callers must use GradeUnknown rather than
// asking Evaluate to invent one.
func TestUnknownIsDistinctFromSlow(t *testing.T) {
	if GradeUnknown == GradeSlow {
		t.Fatal("unknown and slow must be distinguishable")
	}
	for _, th := range Thresholds {
		if th.Grade == GradeUnknown {
			t.Error("GradeUnknown must not be reachable from a threshold")
		}
	}
}

// The identifiers are emitted in JSON and parsed downstream. Changing one is a
// breaking change, so they are pinned here deliberately.
func TestGradeIdentifiersAreStable(t *testing.T) {
	want := []string{"excellent", "very_good", "good", "fair", "slow"}
	if len(Thresholds) != len(want) {
		t.Fatalf("got %d thresholds, want %d", len(Thresholds), len(want))
	}
	for i, w := range want {
		if string(Thresholds[i].Grade) != w {
			t.Errorf("threshold %d: got %q, want %q", i, Thresholds[i].Grade, w)
		}
	}
}

// Thresholds are searched top down, so an unordered list would silently return
// the wrong band.
func TestThresholdsDescend(t *testing.T) {
	for i := 1; i < len(Thresholds); i++ {
		if Thresholds[i].MinMbps >= Thresholds[i-1].MinMbps {
			t.Errorf("threshold %d (%v) does not descend from %v",
				i, Thresholds[i].MinMbps, Thresholds[i-1].MinMbps)
		}
	}
}
