package knuthplass

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type kinx = int   // khipu knot index
type merits int16 // (de-)merits for a line break
type lineNo int   // line number

const noinx = -1

type origin struct {
	from kinx
	line lineNo
}

type pathState struct {
	to   kinx
	line lineNo
}

type predecessor struct {
	from  kinx
	cost  merits
	total merits
}

// pathTable stores the current best predecessor for each reachable (to,line) state.
// It also keeps the set of feasible breakpoints for convenience and debugging.
type pathTable struct {
	feasBP map[kinx]struct{}
	preds  map[pathState]predecessor
}

func newPathTable() *pathTable {
	return &pathTable{
		feasBP: make(map[kinx]struct{}),
		preds:  make(map[pathState]predecessor),
	}
}

func (pt *pathTable) String() string {
	var sb strings.Builder
	states := slices.Collect(maps.Keys(pt.preds))
	slices.SortFunc(states, func(a, b pathState) int {
		d := cmp.Compare(a.to, b.to)
		if d != 0 {
			return d
		}
		return cmp.Compare(a.line, b.line)
	})
	for _, st := range states {
		pred := pt.preds[st]
		sb.WriteString(fmt.Sprintf("(%s,L:%d) <= %s [c:%d tc:%d]\n",
			knotInxStr(st.to), st.line, knotInxStr(pred.from), pred.cost, pred.total))
	}
	return sb.String()
}

// AddBP registers a feasible breakpoint index.
func (pt *pathTable) AddBP(bp kinx) error {
	if _, ok := pt.feasBP[bp]; ok {
		tracer().Errorf("Breakpoint at position %d already known", bp)
	}
	pt.feasBP[bp] = struct{}{}
	return nil
}

// Breakpoint returns the index if it is a registered breakpoint, and noinx otherwise.
func (pt *pathTable) Breakpoint(bp kinx) kinx {
	if _, ok := pt.feasBP[bp]; ok {
		return bp
	}
	return noinx
}

// Pred returns the currently stored predecessor for a reachable (to,line) state.
func (pt *pathTable) Pred(to kinx, line lineNo) (predecessor, bool) {
	pred, ok := pt.preds[pathState{to: to, line: line}]
	return pred, ok
}

// SetPred stores the best known predecessor for a reachable (to,line) state.
// If a predecessor already exists, it is only replaced when the new total cost is cheaper.
// Ties are left stable because choosing either predecessor is acceptable.
func (pt *pathTable) SetPred(from, to kinx, cost, total merits, line lineNo) bool {
	if pt.Breakpoint(to) == noinx {
		pt.AddBP(to)
	}
	st := pathState{to: to, line: line}
	if pred, ok := pt.preds[st]; ok && pred.total <= total {
		return false
	}
	pt.preds[st] = predecessor{
		from:  from,
		cost:  cost,
		total: total,
	}
	return true
}
