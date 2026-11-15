package knuthplass

import (
	"math"

	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

type lineDisposition uint8

const (
	// lineOverfull means the effective minimum width exceeds the target. No
	// tolerance can make this an acceptable ordinary or discretionary line.
	lineOverfull lineDisposition = iota
	// lineScreenedOut means the segment is geometrically feasible, but its badness
	// exceeds the active threshold of the current pass.
	lineScreenedOut
	// lineAccepted means the segment survives pass screening and therefore receives
	// a demerit score for path ranking.
	lineAccepted
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

// calcAdjustmentRatio computes the TeX-style adjustment ratio for one line.
// Positive ratios mean stretching, negative ratios mean shrinking.
// Except at exact fit, the ratio requires stretch or shrink capacity in the
// relevant direction. The classifier supplies signed infinity when it is absent.
//
// In TeX terms:
// - short lines divide by available stretch
// - long lines divide by available shrink
//
// This function is pure geometry. It does not decide whether the line is
// acceptable in the current pass.
func calcAdjustmentRatio(segwss linebreak.WSS, linelen dimen.DU) (float64, bool) {
	if segwss.W == linelen {
		return 0, true
	}
	if segwss.W < linelen {
		available := int64(segwss.Max) - int64(segwss.W)
		if available <= 0 {
			return 0, false
		}
		return float64(int64(linelen)-int64(segwss.W)) / float64(available), true
	}
	available := int64(segwss.W) - int64(segwss.Min)
	if available <= 0 {
		return 0, false
	}
	return -float64(int64(segwss.W)-int64(linelen)) / float64(available), true
}

// badnessForRatio keeps the existing cubic rounding and cap. Infinities map
// to the cap as well. It does not decide whether a shrinking line is overfull.
func badnessForRatio(ratio float64) merits {
	r := absFloat64(ratio)
	return merits(min(100.0, r*r*r) * 100.0)
}

func absFloat64(n float64) float64 {
	if n < 0 {
		return -n
	}
	return n
}
