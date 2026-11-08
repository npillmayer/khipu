package knuthplass

import "github.com/npillmayer/khipu"

// boundaryEvaluations bridges discovery and commit at ONE logical boundary.
// Discovery needs ordinary badness to decide whether to request discretionaries;
// commit needs the same local result to admit, retire or rank fallback origins.
// Costs accumulated before the line are deliberately not cached: commit reads
// those from the current working state.
// Layout, tolerance and emergency stretch must remain constant for the pass.
//
// Successful writes invalidate the entire target bucket, including unchanged
// sibling line counts. This conservative rule avoids per-state revisions and
// handles both new lines and changed discretionary seeds. Losing proposals do
// not invalidate. All production writes go through updatePath,
// evalNewSegmentSeed or retireState; new mutation sites must do the same.
//
// Results are copied out of ordinary scratch, since subsequent origins reuse
// that scratch. Values and index capacity are reused across boundaries, never
// across breakers. No entries survive as valid evaluations of the next knot.
type boundaryEvaluations struct {
	active   bool
	disabled bool // internal diagnostic switch for uncached equivalence tests
	index    map[BreakRef]evaluationSpan
	values   []lineEvaluation
}

type evaluationSpan struct {
	start, end int
	penalty    khipu.Penalty
}

func (c *boundaryEvaluations) begin() {
	if c.disabled {
		return
	}
	if c.index == nil {
		c.index = make(map[BreakRef]evaluationSpan)
	}
	clear(c.index)
	c.values = c.values[:0]
	c.active = true
}

func (c *boundaryEvaluations) end() { c.active = false }

func (c *boundaryEvaluations) invalidate(bp BreakRef) {
	if c.active {
		delete(c.index, bp)
	}
}

func (c *boundaryEvaluations) get(bp BreakRef, penalty khipu.Penalty) ([]lineEvaluation, bool) {
	if !c.active {
		return nil, false
	}
	span, ok := c.index[bp]
	if !ok || span.penalty != penalty {
		return nil, false
	}
	return c.values[span.start:span.end], true
}

func (c *boundaryEvaluations) put(bp BreakRef, penalty khipu.Penalty, evals []lineEvaluation) {
	if c.active {
		start := len(c.values)
		c.values = append(c.values, evals...)
		c.index[bp] = evaluationSpan{start, len(c.values), penalty}
	}
}
