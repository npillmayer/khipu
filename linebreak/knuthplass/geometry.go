package knuthplass

import (
	"math"

	"github.com/npillmayer/khipu/dimen"
)

// classifyLine is the shared, side-effect-free geometry and tolerance screen
// for ordinary lines and shaped discretionary alternatives. Widths must already
// include trimming and side skips. Breakpoint identity, penalties, path costs,
// hyphen extras and decisions to retire an origin belong to its callers.
//
// Shrink is a hard bound: Min > target is overfull regardless of tolerance.
// Stretch is a quality measure, not a hard bound. A short line with no stretch
// gets badness 10000 and a positive infinite ratio; it is not an overfull line
// and may improve when more material is appended. Signed infinities preserve
// the loose/tight distinction when no finite ratio can be computed.
func classifyLine(w WSS, target dimen.DU, tolerance merits) lineEvaluation {
	ratio, available := calcAdjustmentRatio(w, target)
	if !available {
		ratio = math.Inf(1)
		if w.W > target {
			ratio = math.Inf(-1)
		}
	}
	ev := lineEvaluation{
		ratio:   ratio,
		stretch: absD(target - w.W), badness: 10000,
		disposition: lineOverfull,
	}
	if w.Min > target {
		return ev
	}
	ev.badness = badnessForRatio(ratio)
	ev.disposition = lineScreenedOut
	if ev.badness <= tolerance {
		ev.disposition = lineAccepted
	}
	return ev
}

// badnessForRatio keeps the existing cubic rounding and cap. Infinities map
// to the cap as well. It does not decide whether a shrinking line is overfull.
func badnessForRatio(ratio float64) merits {
	r := absFloat64(ratio)
	return merits(min(100.0, r*r*r) * 100.0)
}
