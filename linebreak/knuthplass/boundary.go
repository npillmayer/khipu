package knuthplass

import (
	"math"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/schuko/tracing"
)

// processKnot advances a prepared pass by one logical knot. Updating, rescue
// discovery and boundary commit are separate phases so insertions cannot cause
// the same knot to be accumulated twice. The boolean is ordinary pass liveness;
// errors are reserved for invalid input or failed discretionary providers.
func (kp *linebreaker) processKnot(khp *khipu.Khipu, last kinx) (bool, error) {
	trace := tracer()
	if len(kp.horizon) == 0 {
		return false, nil // a normal exhausted pass, not an input/provider error
	}
	k := khp.KnotByIndex(last)
	if kp.hyphenating && khp.HasExplicitDiscretionary(last) {
		cands, err := kp.discretionaryCandidates(khp, last)
		if err != nil {
			return false, err
		}
		cacheDiscretionaryCandidates(khp, last, cands)
	}
	horizon := kp.snapshotHorizon()
	if trace.GetTraceLevel() >= tracing.LevelDebug {
		trace.Debugf("_______________ %d/%v horizon=%v", last, k, horizon)
	}
	// Advance every existing origin BEFORE any insertion. A retroactive
	// discretionary seed already includes material through last; advancing
	// an overwritten target later in this loop would count that material twice.
	for _, from := range horizon {
		kp.updatePath(from, last, k)
	}
	// Scope reuse to this boundary, including error exits. Pass 1 and forbidden
	// boundaries have no duplicate ordinary scoring and need no cache storage.
	if kp.hyphenating && k.Penalty < InfinityDemerits {
		kp.boundaryEvals.begin()
		defer kp.boundaryEvals.end()
	}
	for _, from := range horizon {
		if from.IsDiscretionary() && last <= from.At {
			continue
		}
		if kp.hyphenating {
			for _, cand := range kp.evaluateDiscretionaryCandidates(khp, from, last, last) {
				kp.activateDiscretionary(from, cand)
			}
		}
		if k.Penalty >= InfinityDemerits || !kp.hyphenating {
			continue
		}
		penalty := k.Penalty
		if penalty <= InfinityMerits {
			penalty = 0
		}
		// Ordinary scratch remains live during nested discretionary scoring.
		// Rescue is state-local, not repeated for all lines in this bucket.
		for _, ev := range kp.evaluateCandidates(from, penalty) {
			if !kp.wantsHyphenation(ev) {
				continue
			}
			candidate, ok := hyphenationCandidateForLine(khp, last, ev)
			if !ok || candidate <= from.At || khp.Penalty[candidate] <= InfinityMerits {
				continue
			}
			if err := kp.requestDiscretionariesForLine(khp, last, ev); err != nil {
				return false, err
			}
			for _, cand := range kp.evaluateDiscretionaryLine(khp, from, ev.line, last, candidate) {
				kp.activateDiscretionary(from, cand)
			}
		}
	}
	if k.Penalty < InfinityDemerits && !kp.finishBoundary(last, k.Penalty) {
		return false, nil
	}
	if kp.compactUniform {
		kp.compactUniformBoundary(PlainBreak(last))
	}
	return true, nil
}

// prepareRetirement checks the entire original paragraph, including the final
// fill, before any state can be discarded. Negative minima can rescue an origin
// later; saturated minimum sums can also invalidate the monotonicity argument.
// Such manually constructed inputs retain their working states until a forced
// boundary. This opt-out changes pruning only, not geometry or pass tolerance.
func (kp *linebreaker) prepareRetirement(khp *khipu.Khipu) {
	kp.prune = true
	kp.minimumSum = 0
	for _, m := range khp.MinW {
		if m < 0 {
			kp.prune = false
		}
		kp.minimumSum += int64(max(0, m))
		if kp.minimumSum > math.MaxInt32 {
			kp.prune = false
			break
		}
	}
	if !kp.hyphenating {
		return
	}
	// Reserve enough headroom for both a post-break seed and a pre-break
	// prefix on the same line. Lazy providers obey the same conservative bound.
	for _, cands := range khp.DiscretionaryCandidates {
		for _, cand := range cands {
			if !kp.safeFragment(cand.PreBreak) || !kp.safeFragment(cand.PostBreak) {
				kp.prune = false
			}
		}
	}
}

func (kp *linebreaker) safeFragment(k khipu.KnotCore) bool {
	return k.MinW >= 0 && int64(k.MinW) <= (math.MaxInt32-kp.minimumSum)/2
}

// activateDiscretionary separates current segment bookkeeping from the immutable
// rescan seed. Retroactive discovery has already consumed the suffix after the
// box; storing that suffix in the seed would duplicate it on the next rescan.
func (kp *linebreaker) activateDiscretionary(from BreakRef, cand breakCandidate) {
	line := cand.eval.line + 1
	if kp.evalNewSegmentSeed(from, cand.target, line, cand.eval.lineCost().cost, cand.seed) {
		kp.seeds[origin{cand.target, line}] = cand.initial
		kp.horizon[cand.target] = struct{}{}
	}
}

// retireState deletes only working data. Historical predecessors remain usable
// by descendants and the collector. Tombstones also cover root/forced-barrier
// retirement, preventing a later retroactive discovery from reviving the state.
func (kp *linebreaker) retireState(o origin) {
	kp.boundaryEvals.invalidate(o.from)
	if kp.metrics != nil {
		if _, live := kp.paths[o.from][o.line]; live {
			kp.metrics.live--
		}
	}
	delete(kp.paths[o.from], o.line)
	delete(kp.seeds, o)
	kp.retired[o] = struct{}{}
	if len(kp.paths[o.from]) == 0 {
		delete(kp.paths, o.from)
		delete(kp.horizon, o.from)
	}
}

type fallbackChoice struct {
	from  origin
	local cost
	total merits
	valid bool
}

// considerFallback ranks the complete transition, including the terminal hyphen
// extra. Canonical ties must not depend on line-map or horizon iteration order.
// Source data stays live until the winner is inserted.
func (kp *linebreaker) considerFallback(best *fallbackChoice, ev lineEvaluation, to BreakRef) {
	c := cost{badness: ev.badness, demerits: AwfulDemerits + kp.terminalHyphenDemerits(ev.bp, to)}
	total := kp.paths[ev.bp][ev.line].totalcost + c.demerits
	order := compareBreakRef(ev.bp, best.from.from)
	if !best.valid || total < best.total || total == best.total &&
		(order < 0 || order == 0 && ev.line < best.from.line) {
		*best = fallbackChoice{from: origin{ev.bp, ev.line}, local: c, total: total, valid: true}
	}
}

// finishBoundary is the commit phase for one permitted boundary. Rescue has
// finished, so this snapshot includes newly discovered discretionary successors.
// Their segments already extend through last: score them without advancing them
// again. This is essential when last is forced and there is no next iteration
// in which a discovered post-break remainder could reach the boundary.
//
// Evaluate first, remember retirements by ID, then choose any final-pass fallback
// while its source still exists. Only then delete working states. The terminal
// bucket survives for collection; all other buckets stop at a forced boundary.
// False means the pass has no solution, not an operational error.
func (kp *linebreaker) finishBoundary(last kinx, penalty khipu.Penalty) bool {
	to := PlainBreak(last)
	forced := penalty <= InfinityMerits
	if forced {
		penalty = 0
	}
	kp.retirementScratch = kp.retirementScratch[:0]
	var fallback fallbackChoice
	survivor := false
	for _, from := range kp.snapshotHorizon() {
		if from.At >= last {
			// A lookahead discretionary has not reached its box yet. Its
			// post-break fragment is not a line ending at this earlier knot.
			survivor = true
			continue
		}
		for _, ev := range kp.evaluateCandidates(from, penalty) {
			kp.considerFallback(&fallback, ev, to)
			if ev.accepted() {
				c := ev.lineCost().cost
				c.demerits += kp.terminalHyphenDemerits(from, to)
				if kp.evalNewSegment(from, to, ev.line+1, c) {
					kp.horizon[to] = struct{}{}
				}
			}
			if kp.prune && ev.disposition == lineOverfull {
				kp.retirementScratch = append(kp.retirementScratch, origin{from, ev.line})
			} else {
				survivor = true
			}
		}
	}
	reached := len(kp.paths[to]) != 0
	needsFallback := !reached && (forced || !survivor)
	if needsFallback && kp.final && fallback.valid {
		reached = kp.evalNewSegment(fallback.from.from, to, fallback.from.line+1, fallback.local)
		if reached {
			kp.horizon[to] = struct{}{}
		}
	}
	for _, o := range kp.retirementScratch {
		kp.retireState(o)
	}
	if forced {
		// No discretionary lookahead or ordinary origin may cross a forced
		// boundary, even when width preconditions disabled early retirement.
		for from, lines := range kp.paths {
			if from != to {
				for line := range lines {
					kp.retireState(origin{from, line})
				}
			}
		}
		return reached
	}
	return !needsFallback || reached
}
