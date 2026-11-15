package knuthplass

import (
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

// calcBadness composes the production ratio and cubic badness calculations for
// scoring assertions. Pass acceptance, including excessive shrink rejection,
// is tested separately through classifyLine.
func calcBadness(segwss linebreak.WSS, linelen dimen.DU) (merits, bool) {
	ratio, ok := calcAdjustmentRatio(segwss, linelen)
	if !ok {
		return 0, false
	}
	return badnessForRatio(ratio), true
}

// collectOptimalBreakpoints projects the winning path for knot-index assertions.
func (kp *linebreaker) collectOptimalBreakpoints(end BreakRef) ([]kinx, pathQuality, bool) {
	breakrefs, quality, ok := kp.collectOptimalBreakRefs(end)
	return projectBreakRefs(breakrefs), quality, ok
}

func absD(n dimen.DU) dimen.DU {
	if n < 0 {
		return -n
	}
	return n
}
