package knuthplass

import (
	"fmt"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/schuko/tracing"
)

type cost struct {
	badness  merits // line badness used for pass screening
	demerits merits // line demerits used for path ranking
}

func (c cost) String() string {
	return fmt.Sprintf("C(b:%d d:%d)", c.badness, c.demerits)
}

// terminalHyphenDemerits applies the "last line" discretionary extra at the
// only point where it can be known cheaply: when a transition reaches the final
// forced break of the paragraph. In that situation, the predecessor break is the
// break that created the last line.
func (kp *linebreaker) terminalHyphenDemerits(from, to BreakRef) merits {
	if kp == nil || kp.params == nil {
		return 0
	}
	if to == kp.end && from.IsDiscretionary() {
		return kp.params.FinalHyphenDemerits
	}
	return 0
}

// calcDemerits computes line demerits from a precomputed badness value.
// Forced breaks are handled in buildBreakpointPaths and therefore reach
// this function with a neutralized penalty.
//
// The shape follows the TeX idea:
// - q = linePenalty + badness
// - start with q^2
// - positive penalties add p^2
// - negative penalties subtract p^2
//
// The result is a ranking value among already accepted candidates; screening by
// badness has happened earlier.
func calcDemerits(badness merits, penalty khipu.Penalty, params *Parameters) merits {
	p := clampPenalty(merits(penalty))
	q := params.LinePenalty + badness
	d := q * q
	p2 := p * p
	if p > 0 {
		d += p2
	} else if p < 0 {
		d -= p2
	}
	if trace := tracer(); trace.GetTraceLevel() >= tracing.LevelDebug {
		trace.Debugf("    calculating demerits for p=%d, b=%d: d=%d", p, badness, d)
	}
	return d
}

// clampPenalty caps a penalty value to the TeX-style sentinel range.
func clampPenalty(p merits) merits {
	if p > InfinityDemerits {
		return InfinityDemerits
	}
	if p < InfinityMerits {
		return InfinityMerits
	}
	return p
}
