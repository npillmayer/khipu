package knuthplass

import (
	"github.com/npillmayer/khipu/linebreak"
	"math"

	"github.com/npillmayer/khipu/dimen"
)

// classifyLine is the shared, side-effect-free geometry and tolerance screen
// for ordinary lines and shaped discretionary alternatives. Widths must already
// include trimming and side skips. Breakpoint identity, penalties, path costs,
// hyphen extras and decisions to retire an origin belong to its callers.
//
// extraStretch is virtual finite capacity for emergency scoring only; it never
// changes widths or shrink. Nonpositive values preserve the original calculation.
// Positive values must be below dimen.Fil (validated by public pass setup).
// Max >= Fil deliberately keeps the old numerical fill/saturation behavior:
// linebreak.WSS cannot distinguish actual fill from a saturated sum of finite maxima.
// This compatibility rule does not claim exact symbolic fill semantics.
//
// Shrink is a hard bound: Min > target is overfull regardless of tolerance.
// Stretch is a quality measure, not a hard bound. A short line with neither
// real nor virtual stretch gets badness 10000 and a positive infinite ratio;
// it is not an overfull line and may improve when more material is appended.
// Signed infinities preserve
// the loose/tight distinction when no finite ratio can be computed.
func classifyLine(w linebreak.WSS, target dimen.DU, tolerance merits, extraStretch dimen.DU) lineEvaluation {
	var ratio float64
	var available bool
	difference := int64(target) - int64(w.W)
	if difference < 0 {
		difference = -difference
	}
	stretch := dimen.DU(min(difference, int64(math.MaxInt32)))
	if extraStretch > 0 && w.W < target && w.Max < dimen.Fil {
		// Widen BEFORE subtraction and addition. The virtual denominator may
		// exceed both DU's range and the fill sentinels without becoming fill.
		shortfall := int64(target) - int64(w.W)
		capacity := int64(w.Max) - int64(w.W) + int64(extraStretch)
		if capacity > 0 {
			ratio, available = float64(shortfall)/float64(capacity), true
		}
		// Only the stored adjustment magnitude is bounded; neither operand
		// used for the ratio is narrowed or clamped.
		stretch = dimen.DU(min(shortfall, int64(math.MaxInt32)))
	} else {
		ratio, available = calcAdjustmentRatio(w, target)
	}
	if !available {
		ratio = math.Inf(1)
		if w.W > target {
			ratio = math.Inf(-1)
		}
	}
	ev := lineEvaluation{
		ratio:   ratio,
		stretch: stretch, badness: 10000,
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
