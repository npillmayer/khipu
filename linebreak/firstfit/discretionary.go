package firstfit

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

// DiscretionaryProvider supplies hyphenation alternatives for one textbox
// knot. firstfit uses it only as a local overflow rescue for the loose-line
// case.
// Calls are synchronous and have no context argument. Encoding/freezing contexts
// do not propagate here; the standard owner uses Background for lazy shaping.
// Providers must preserve published selections even when discovery fails.
type DiscretionaryProvider interface {
	DiscretionaryCandidates(khp *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error)
}

// discretionaryCandidates is the local provider seam for firstfit. In
// production this usually delegates to Khipukamayuq; tests may also satisfy it
// directly from the Khipu cache.
func discretionaryCandidates(khp *khipu.Khipu, at kinx, params *Parameters) ([]khipu.DiscretionaryCandidate, error) {
	if khp == nil || at < 0 || at >= len(khp.Kind) {
		return nil, khipu.ErrIllegalArguments
	}
	if params != nil && params.DiscretionaryProvider != nil {
		return params.DiscretionaryProvider.DiscretionaryCandidates(khp, int(at))
	}
	return khp.DiscretionariesAt(int(at)), nil
}

// tryLooseHyphenation evaluates discretionary variants for the overflowing
// textbox itself. A candidate is accepted only if its pre-break penalty permits
// breaking, its minimum width fits the current line, and its natural-width gain
// over the last non-hyphenated checkpoint is at least MinHyphenGain.
//
// The selection rule is intentionally greedy and local:
//   - reject PreBreak.Penalty >= InfinityDemerits, even if its width is ideal
//   - keep only variants whose PreBreak fits this line at minimum width
//   - reject variants that do not improve the line enough
//   - among the survivors, prefer the widest PreBreak
//   - use lower breakpoint penalty as a tie-break
//
// Finite penalties do not compete with width: a wider eligible variant wins
// regardless of its finite penalty. Exact width/penalty ties keep provider order.
// Zero MinHyphenGain permits zero improvement. This is a local gain filter, not
// a hyphen-frequency limit or a demerit model; discovery itself stays lazy.
func tryLooseHyphenation(khp *khipu.Khipu, at kinx, base segmentState, checkpointWidth linebreak.WSS, linelen dimen.DU, layout khipu.Params, params *Parameters) (khipu.DiscretionaryCandidate, segmentState, bool, error) {
	cands, err := discretionaryCandidates(khp, at, params)
	if err != nil || len(cands) == 0 {
		return khipu.DiscretionaryCandidate{}, segmentState{}, false, err
	}
	bestWidth := dimen.DU(-1)
	bestPenalty := khipu.Penalty(InfinityDemerits)
	var best khipu.DiscretionaryCandidate
	var bestSeed segmentState
	for _, cand := range cands {
		// Prohibition is eligibility, not a preference used only to break ties.
		// Apply it equally to provider-returned and already cached alternatives.
		if cand.PreBreak.Penalty >= InfinityDemerits {
			continue
		}
		lineSeg := base.segmentState()
		lineSeg.append(cand.PreBreak)
		segw := lineSeg.width(layout)
		if err := segw.Err(); err != nil {
			return khipu.DiscretionaryCandidate{}, segmentState{}, false, err
		}
		if segw.Min > linelen {
			continue
		}
		gain := int64(segw.W) - int64(checkpointWidth.W)
		if gain < int64(params.MinHyphenGain) {
			continue
		}
		if segw.W > bestWidth || (segw.W == bestWidth && cand.PreBreak.Penalty < bestPenalty) {
			best = cand
			bestWidth = segw.W
			bestPenalty = cand.PreBreak.Penalty
			bestSeed = segmentSeedFromKnot(cand.PostBreak)
		}
	}
	if bestWidth < 0 {
		return khipu.DiscretionaryCandidate{}, segmentState{}, false, nil
	}
	return best, bestSeed, true, nil
}
