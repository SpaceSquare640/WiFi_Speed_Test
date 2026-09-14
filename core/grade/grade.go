// Package grade classifies a throughput figure into a coarse band, so that a
// number the user cannot calibrate becomes a judgement they can act on.
package grade

// Grade is a stable machine-readable band identifier.
//
// The identifier, not its display text, is what appears in JSON output.
// Emitting localised text there would break every downstream parser the moment
// the interface language changed.
type Grade string

const (
	// GradeUnknown is the grade of a rate that was never measured.
	//
	// It exists because the alternative is worse: falling through to the lowest
	// band would report a slow connection for a measurement that never ran,
	// which is a claim the tool has no evidence for.
	GradeUnknown Grade = "unknown"

	GradeExcellent Grade = "excellent"
	GradeVeryGood  Grade = "very_good"
	GradeGood      Grade = "good"
	GradeFair      Grade = "fair"
	GradeSlow      Grade = "slow"
)

// Threshold binds a lower bound in Mbps to the grade earned at or above it.
type Threshold struct {
	MinMbps float64
	Grade   Grade
}

// Thresholds are ordered from highest to lowest. The values carry over from the
// previous generation of the tool, where they proved reasonable in practice.
var Thresholds = []Threshold{
	{500, GradeExcellent},
	{200, GradeVeryGood},
	{100, GradeGood},
	{50, GradeFair},
	{0, GradeSlow},
}

// Evaluate returns the grade earned by the given rate.
//
// A rate below every threshold, including a negative one produced by a failed
// measurement, earns the lowest band rather than an empty grade: the report has
// to say something, and "slow" is the honest reading of "no throughput".
func Evaluate(mbps float64) Grade {
	for _, t := range Thresholds {
		if mbps >= t.MinMbps {
			return t.Grade
		}
	}
	return GradeSlow
}
