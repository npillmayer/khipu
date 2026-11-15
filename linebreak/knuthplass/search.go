package knuthplass

import (
	"slices"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/schuko/tracing"
)

// buildBreakpointPaths scans SOA knot indices and maintains a horizon of
// active BreakRefs. For each permitted boundary, it evaluates transitions from
// active origins and records the cheapest predecessor for each line count.
// Forced boundaries retire earlier origins; discretionary variants seed the
// following line with their post-break fragment. Collection follows the
// predecessor table to recover the single cheapest complete path.
func (kp *linebreaker) buildBreakpointPaths(khp *khipu.Khipu) error {
	if khp == nil || len(khp.W) == 0 {
		return ErrNoBreakpoints
	}
	kp.end = PlainBreak(len(khp.W) - 1)
	kp.prepareRetirement(khp)
	trace := tracer()
	for last := range len(khp.W) {
		live, err := kp.processKnot(khp, last)
		if kp.arithmeticErr != nil {
			return kp.arithmeticErr
		}
		if err != nil || !live {
			return err
		}
	}
	if trace.GetTraceLevel() >= tracing.LevelInfo {
		trace.Infof("Collected %d potential breakpoints for paragraph", len(kp.feasBP))
		trace.Infof("          %v", kp.feasBP)
		trace.Infof("          predecessors:\n%v", kp.String())
		trace.Infof("          paths:\n%s", pathsAsString(kp.paths))
	}
	return nil
}

// snapshotHorizon returns a sorted snapshot, valid until its next call. The
// live horizon can change while one phase consumes it. Only after that phase
// finishes may the next phase refresh it, including boundary commit at the same
// knot. Newly discovered successors then participate without advancing twice.
func (kp *linebreaker) snapshotHorizon() []BreakRef {
	kp.horizonScratch = kp.horizonScratch[:0]
	for bp := range kp.horizon {
		kp.horizonScratch = append(kp.horizonScratch, bp)
	}
	slices.SortFunc(kp.horizonScratch, compareBreakRef)
	return kp.horizonScratch
}

func (kp *linebreaker) evalNewSegment(from, to BreakRef, line lineNo, c cost) bool {
	return kp.evalNewSegmentSeed(from, to, line, c, bookkeeping{})
}

// evalNewSegmentSeed reports whether the target has a live working state,
// including when an existing predecessor wins. Callers may activate only live
// targets. Retired targets keep their historical predecessor unchanged.
func (kp *linebreaker) evalNewSegmentSeed(from, to BreakRef, line lineNo, c cost, seed bookkeeping) bool {
	if _, retired := kp.retired[origin{to, line}]; retired {
		return false
	}
	trace := tracer()
	debug := trace.GetTraceLevel() >= tracing.LevelDebug
	if kp.pathTable.Breakpoint(to) == (BreakRef{At: noinx}) {
		kp.pathTable.AddBP(to)
	}
	// now sure that `to` is a breakpoint
	preSeg, ok := kp.paths[from][line-1]
	if debug {
		trace.Debugf("K&P prev seg = %v", preSeg)
	}
	assert(ok, "K&P internal error: cannot append segment to non-existent sub-path")
	evalCost := preSeg.totalcost + c.demerits
	pred, hasPred := kp.pathTable.Pred(to, line)
	lines := kp.paths[to]
	_, ok = lines[line]
	path := seed.segmentState() // `path` will be the new path, if cheaper than `seg`
	if !hasPred {
		assert(!ok, "K&P internal error: path with non-existent predecessor state exists")
	} else if !betterPredecessor(from, evalCost, pred) {
		assert(ok, "K&P internal error: path for predecessor state does not exist")
		return true // existing path is cheaper or wins the canonical tie
	} else {
		if debug {
			trace.Debugf("K&P remove sub-optimal seg %v", origin{to, line})
		}
		delete(lines, line) // remove the superseded working state
		delete(kp.seeds, origin{to, line})
	}
	kp.pathTable.SetPred(from, to, c.demerits, evalCost, line)
	path.totalcost = evalCost
	path.worstbadness = max(preSeg.worstbadness, c.badness)
	if lines == nil {
		lines = make(map[lineNo]bookkeeping)
		kp.paths[to] = lines
	}
	lines[line] = path
	kp.boundaryEvals.invalidate(to)
	if kp.metrics != nil && !ok {
		kp.metrics.live++
		kp.metrics.peak = max(kp.metrics.peak, kp.metrics.live)
	}
	kp.seeds[origin{to, line}] = seed.segmentState()
	// Debugf cannot defer argument evaluation. In particular, sorting and
	// formatting the entire state table must never happen with debug disabled.
	if debug {
		trace.Debugf(" paths:\n%s", pathsAsString(kp.paths))
		trace.Debugf("new segment %s ---(C:%d|L:%d)---> %s", from.String(), c.demerits,
			line, to.String())
	}
	return true
}

func assert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}
