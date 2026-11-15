package knuthplass

import (
	"slices"

	"github.com/npillmayer/khipu"
)

type pathQuality struct {
	totalCost    merits // sum of demerits along the winning path
	worstBadness merits // highest local badness under this pass's scoring assumptions
}

// collectOptimalBreakRefs walks the predecessor table backwards from the
// paragraph terminus and returns the single cheapest breakpoint sequence.
// Equal total costs prefer fewer lines, independent of map iteration order.
func (kp *linebreaker) collectOptimalBreakRefs(end BreakRef) ([]BreakRef, pathQuality, bool) {
	bestLine := lineNo(0)
	bestQuality := pathQuality{}
	found := false
	for line, path := range kp.paths[end] {
		if !found || path.totalcost < bestQuality.totalCost ||
			(path.totalcost == bestQuality.totalCost && line < bestLine) {
			bestLine = line
			bestQuality = pathQuality{
				totalCost:    path.totalcost,
				worstBadness: path.worstbadness,
			}
			found = true
		}
	}
	if !found {
		return nil, pathQuality{}, false
	}
	breaks := make([]BreakRef, 0, bestLine)
	cur := end
	line := bestLine
	for cur != kp.root {
		breaks = append(breaks, cur)
		pred, ok := kp.pathTable.Pred(cur, line)
		if !ok {
			return nil, pathQuality{}, false
		}
		cur = pred.from
		line--
	}
	slices.Reverse(breaks)
	return breaks, bestQuality, true
}

func projectBreakRefs(breakrefs []BreakRef) []kinx {
	if len(breakrefs) == 0 {
		return nil
	}
	breakpoints := make([]kinx, 0, len(breakrefs))
	for _, br := range breakrefs {
		breakpoints = append(breakpoints, br.At)
	}
	return breakpoints
}

// recordSelectedDiscretionaries persists only the discretionary choices from
// the winning path back into the Khipu. Ordinary breakpoints continue to be
// represented by the returned []kinx; SelectedDiscretionaries stores the extra
// variant information needed for later paragraph processing stages.
func recordSelectedDiscretionaries(khp *khipu.Khipu, breakrefs []BreakRef) {
	if khp == nil {
		return
	}
	if khp.SelectedDiscretionaries != nil {
		// The final Khipu should reflect only the winning path of the current
		// linebreaking run; stale selections from earlier runs must not survive.
		clear(khp.SelectedDiscretionaries)
	}
	for _, br := range breakrefs {
		if !br.IsDiscretionary() {
			continue
		}
		khp.SelectDiscretionary(br.At, khipu.DiscretionarySelection{
			Source:  br.At,
			Variant: br.Variant,
		})
	}
}
