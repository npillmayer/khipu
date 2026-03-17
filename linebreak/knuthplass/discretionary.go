package knuthplass

import "github.com/npillmayer/khipu"

// DiscretionaryProvider supplies hyphenation alternatives for one textbox knot.
// Ownership of candidate discovery and caching stays with the provider.
type DiscretionaryProvider interface {
	DiscretionaryCandidates(khp *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error)
}

// discretionaryCandidates is the linebreaker's single entry point for
// textbox-local hyphenation opportunities. In production this usually delegates
// to Khipukamayuq; tests may also satisfy it from the Khipu cache directly.
func (kp *linebreaker) discretionaryCandidates(khp *khipu.Khipu, at kinx) ([]khipu.DiscretionaryCandidate, error) {
	if khp == nil || at < 0 || at >= len(khp.Kind) {
		return nil, khipu.ErrIllegalArguments
	}
	if kp != nil && kp.params != nil && kp.params.DiscretionaryProvider != nil {
		return kp.params.DiscretionaryProvider.DiscretionaryCandidates(khp, int(at))
	}
	return khp.DiscretionariesAt(int(at)), nil
}

// isLineDiscardable answers the current linebreaking notion of discardability.
// For now this remains an explicit glue/kern rule, optionally reinforced by the
// flag plane on KnotCore. More script-sensitive trimming rules can later replace
// this without changing the surrounding pass-2 logic.
func isLineDiscardable(k khipu.KnotCore) bool {
	return k.IsDiscardable()
}

// previousNonDiscardable and nextNonDiscardable walk the physical Khipu around a
// candidate breakpoint. They are used to locate hyphenation candidates locally
// instead of storing additional textbox identity in path state.
func previousNonDiscardable(khp *khipu.Khipu, at kinx) (kinx, bool) {
	if khp == nil || at < 0 || at >= len(khp.Kind) {
		return noinx, false
	}
	for i := at; i >= 0; i-- {
		if !isLineDiscardable(khp.KnotByIndex(i)) {
			return i, true
		}
	}
	return noinx, false
}

func nextNonDiscardable(khp *khipu.Khipu, at kinx) (kinx, bool) {
	if khp == nil || at < 0 || at >= len(khp.Kind) {
		return noinx, false
	}
	for i := at; i < len(khp.Kind); i++ {
		if !isLineDiscardable(khp.KnotByIndex(i)) {
			return i, true
		}
	}
	return noinx, false
}

// previousContentBox and nextContentBox narrow the local scans to actual
// textboxes. If a non-discardable non-textbox knot intervenes, the scan stops:
// that local context is not a hyphenation candidate.
func previousContentBox(khp *khipu.Khipu, at kinx) (kinx, bool) {
	if khp == nil || at < 0 || at >= len(khp.Kind) {
		return noinx, false
	}
	for i := at; i >= 0; i-- {
		k := khp.KnotByIndex(i)
		if isLineDiscardable(k) {
			continue
		}
		if k.Kind == khipu.KTTextBox {
			return i, true
		}
		return noinx, false
	}
	return noinx, false
}

func nextContentBox(khp *khipu.Khipu, at kinx) (kinx, bool) {
	if khp == nil || at < 0 || at >= len(khp.Kind) {
		return noinx, false
	}
	for i := at; i < len(khp.Kind); i++ {
		k := khp.KnotByIndex(i)
		if isLineDiscardable(k) {
			continue
		}
		if k.Kind == khipu.KTTextBox {
			return i, true
		}
		return noinx, false
	}
	return noinx, false
}

// wantsHyphenation is the pass-2 gate. A line must already be geometrically
// meaningful; only then does pass 2 ask whether its badness is high enough to
// justify looking for discretionary alternatives.
func (kp *linebreaker) wantsHyphenation(ev lineEvaluation) bool {
	if kp == nil || !kp.hyphenating || ev.disposition == lineInfeasible {
		return false
	}
	return ev.badness > kp.params.PreTolerance
}

// hyphenationCandidateForPass2 picks the textbox to consult for one bad line:
// loose lines ask for the next box that did not fit, tight lines ask for the
// last box that is already on the line.
func hyphenationCandidateForPass2(khp *khipu.Khipu, breakpoint kinx, ev lineEvaluation) (kinx, bool) {
	if khp == nil || ev.disposition == lineInfeasible {
		return noinx, false
	}
	switch {
	case ev.ratio > 0:
		if breakpoint+1 >= len(khp.Kind) {
			return noinx, false
		}
		return nextContentBox(khp, breakpoint+1)
	case ev.ratio < 0:
		return previousContentBox(khp, breakpoint)
	default:
		return noinx, false
	}
}

// requestPass2Discretionaries performs lazy candidate discovery for one local
// pass-2 situation. It does not yet decide which discretionary to use; it only
// ensures the relevant candidates are available on the Khipu for subsequent
// evaluation.
func (kp *linebreaker) requestPass2Discretionaries(khp *khipu.Khipu, breakpoint kinx, ev lineEvaluation) error {
	if !kp.wantsHyphenation(ev) {
		return nil
	}
	candidate, ok := hyphenationCandidateForPass2(khp, breakpoint, ev)
	if !ok {
		return nil
	}
	cands, err := kp.discretionaryCandidates(khp, candidate)
	if err != nil {
		return err
	}
	cacheDiscretionaryCandidates(khp, candidate, cands)
	return nil
}

// cacheDiscretionaryCandidates merges newly returned candidates into the Khipu
// cache while keeping existing variants stable. The Khipu remains the durable
// store of discovered opportunities for the paragraph instance.
func cacheDiscretionaryCandidates(khp *khipu.Khipu, at kinx, cands []khipu.DiscretionaryCandidate) {
	if khp == nil || at < 0 || len(cands) == 0 {
		return
	}
	known := make(map[uint16]struct{})
	for _, cand := range khp.DiscretionariesAt(int(at)) {
		known[cand.Variant] = struct{}{}
	}
	for _, cand := range cands {
		if _, ok := known[cand.Variant]; ok {
			continue
		}
		khp.AddDiscretionaryCandidate(int(at), cand)
		known[cand.Variant] = struct{}{}
	}
}

// appendKnotToBook applies ordinary trimming semantics to one KnotCore. It is
// shared by physical Khipu knots and by synthetic discretionary fragments.
func appendKnotToBook(book *bookkeeping, k khipu.KnotCore) {
	wss := WSS{}.SetFromKnot(k)
	cls := classifyLineItem(k)
	book.appendItem(cls, wss)
}

// seedBookkeepingFromKnot creates the initial bookkeeping state for the next
// line after a discretionary break. In practice this means: start the successor
// segment with the post-break fragment already present.
func seedBookkeepingFromKnot(k khipu.KnotCore) bookkeeping {
	var seed bookkeeping
	appendKnotToBook(&seed, k)
	return seed
}

// appendActualKnots replays the physical knots of the paragraph into a temporary
// bookkeeping state. This is the key to evaluating discretionary variants
// without mutating the live path state during candidate scoring.
func (kp *linebreaker) appendActualKnots(book *bookkeeping, khp *khipu.Khipu, from, to kinx) {
	if kp == nil || khp == nil || from > to {
		return
	}
	for i := from; i <= to; i++ {
		appendKnotToBook(book, khp.KnotByIndex(i))
	}
}

// discretionaryDemerits computes the local demerits of breaking through a
// discretionary candidate. The breakpoint penalty comes from the candidate's
// PreBreak knot, while the first-line extra remains a linebreaker-local policy
// decision.
//
// Note that DoubleHyphenDemerits is intentionally not applied here yet.
// Consecutive discretionary penalties are path-dependent: they need to know
// whether the previous chosen line also ended at a discretionary breakpoint.
// The current implementation does not carry that extra predecessor state.
func (kp *linebreaker) discretionaryDemerits(badness merits, penalty khipu.Penalty, from BreakRef) merits {
	d := calcDemerits(badness, penalty, kp.params)
	if kp != nil && kp.params != nil && from == kp.root {
		d += kp.params.FirstHyphenDemerits
	}
	return d
}

// evaluateDiscretionaryCandidates turns cached discretionary variants for one
// textbox into additional feasible break opportunities.
//
// There are two cases:
//   - candidate > breakpoint: a loose line wants to pull part of the next textbox
//     back onto the current line
//   - candidate <= breakpoint: a tight line wants to split a textbox that is
//     already part of the current segment
//
// Each accepted candidate becomes a BreakRef{At, Variant} plus seed state for
// the successor line starting with the post-break fragment.
func (kp *linebreaker) evaluateDiscretionaryCandidates(khp *khipu.Khipu, from BreakRef, breakpoint, candidate kinx) []breakCandidate {
	if kp == nil || khp == nil || candidate < 0 || candidate >= len(khp.Kind) {
		return nil
	}
	if khp.Kind[candidate] != khipu.KTTextBox {
		return nil
	}
	cands := khp.DiscretionariesAt(int(candidate))
	if len(cands) == 0 {
		return nil
	}
	targetBreak := func(v uint16) BreakRef {
		return BreakRef{At: candidate, Variant: v}
	}
	results := make([]breakCandidate, 0, len(cands))
	for st, path := range kp.paths {
		if st.from != from {
			continue
		}
		linelen := kp.parshape.LineLength(int32(st.line + 1))
		for _, cand := range cands {
			lineBook := bookkeeping{}
			nextSeed := bookkeeping{}
			switch {
			case candidate > breakpoint:
				lineBook = path.segmentState()
				kp.appendActualKnots(&lineBook, khp, breakpoint+1, candidate-1)
				appendKnotToBook(&lineBook, cand.PreBreak)
				nextSeed = seedBookkeepingFromKnot(cand.PostBreak)
			default:
				seed, ok := kp.seeds[origin{from, st.line}]
				if !ok {
					continue
				}
				lineBook = seed.segmentState()
				kp.appendActualKnots(&lineBook, khp, from.At+1, candidate-1)
				appendKnotToBook(&lineBook, cand.PreBreak)
				nextSeed = seedBookkeepingFromKnot(cand.PostBreak)
				kp.appendActualKnots(&nextSeed, khp, candidate+1, breakpoint)
			}
			segwss := lineBook.effectiveWidth(kp.params)
			ratio, ok := calcAdjustmentRatio(segwss, linelen)
			if !ok {
				continue
			}
			badness, _ := calcBadness(segwss, linelen)
			if badness > kp.badnessLimit() {
				continue
			}
			results = append(results, breakCandidate{
				eval: lineEvaluation{
					bp:          from,
					line:        st.line,
					ratio:       ratio,
					stretch:     absD(linelen - segwss.W),
					disposition: lineAccepted,
					badness:     badness,
					demerits:    kp.discretionaryDemerits(badness, cand.PreBreak.Penalty, from),
				},
				target: targetBreak(cand.Variant),
				seed:   nextSeed,
			})
		}
	}
	return results
}
