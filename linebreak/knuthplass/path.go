package knuthplass

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type kinx = int // khipu knot index
type lineNo int // line number

const noinx = -1

func compareBreakRef(a, b BreakRef) int {
	d := cmp.Compare(a.At, b.At)
	if d != 0 {
		return d
	}
	return cmp.Compare(a.Variant, b.Variant)
}

type origin struct {
	from BreakRef
	line lineNo
}

type pathState struct {
	to   BreakRef
	line lineNo
}

type predecessor struct {
	from  BreakRef
	cost  merits
	total merits
}

// pathTable stores the current best predecessor for each reachable (to,line) state.
// It also keeps the set of feasible breakpoints for convenience and debugging.
type pathTable struct {
	feasBP map[BreakRef]struct{}
	preds  map[pathState]predecessor
}

func newPathTable() *pathTable {
	return &pathTable{
		feasBP: make(map[BreakRef]struct{}),
		preds:  make(map[pathState]predecessor),
	}
}

func (pt *pathTable) String() string {
	var sb strings.Builder
	states := slices.Collect(maps.Keys(pt.preds))
	slices.SortFunc(states, func(a, b pathState) int {
		d := compareBreakRef(a.to, b.to)
		if d != 0 {
			return d
		}
		return cmp.Compare(a.line, b.line)
	})
	for _, st := range states {
		pred := pt.preds[st]
		sb.WriteString(fmt.Sprintf("(%s,L:%d) <= %s [c:%d tc:%d]\n",
			st.to.String(), st.line, pred.from.String(), pred.cost, pred.total))
	}
	return sb.String()
}

// AddBP registers a feasible breakpoint index.
func (pt *pathTable) AddBP(bp BreakRef) error {
	if _, ok := pt.feasBP[bp]; ok {
		tracer().Errorf("Breakpoint at position %s already known", bp.String())
	}
	pt.feasBP[bp] = struct{}{}
	return nil
}

// Breakpoint returns a registered reference, or a fresh {At: noinx} value.
// Missing lookups never depend on the exported legacy NoBreakRef variable.
func (pt *pathTable) Breakpoint(bp BreakRef) BreakRef {
	if _, ok := pt.feasBP[bp]; ok {
		return bp
	}
	return BreakRef{At: noinx}
}

// Pred returns the currently stored predecessor for a reachable (to,line) state.
func (pt *pathTable) Pred(to BreakRef, line lineNo) (predecessor, bool) {
	pred, ok := pt.preds[pathState{to: to, line: line}]
	return pred, ok
}

// SetPred stores the best known predecessor for a reachable (to,line) state.
// Equal total costs prefer the lower predecessor knot index, then variant.
// This canonical ordering is independent of map iteration and arrival order.
func (pt *pathTable) SetPred(from, to BreakRef, cost, total merits, line lineNo) bool {
	if pt.Breakpoint(to) == (BreakRef{At: noinx}) {
		pt.AddBP(to)
	}
	st := pathState{to: to, line: line}
	if pred, ok := pt.preds[st]; ok && !betterPredecessor(from, total, pred) {
		return false
	}
	pt.preds[st] = predecessor{
		from:  from,
		cost:  cost,
		total: total,
	}
	return true
}

// betterPredecessor is shared by table updates and live path bookkeeping so
// they retain the same winner, including equal-cost discretionary alternatives.
func betterPredecessor(from BreakRef, total merits, old predecessor) bool {
	return total < old.total || (total == old.total && compareBreakRef(from, old.from) < 0)
}
