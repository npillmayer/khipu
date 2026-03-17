package knuthplass

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type kinx = int   // khipu knot index
type merits int32 // (de-)merits for a line break
type lineNo int   // line number

const noinx = -1

// BreakRef identifies a logical breakpoint for linebreaking.
// Variant == 0 denotes the ordinary breakpoint at knot index At.
// Variant > 0 denotes a discretionary breakpoint inside knot At.
type BreakRef struct {
	At      kinx
	Variant uint16
}

var NoBreakRef = BreakRef{At: noinx}

func PlainBreak(at kinx) BreakRef {
	return BreakRef{At: at}
}

func (br BreakRef) IsDiscretionary() bool {
	return br.Variant > 0
}

func (br BreakRef) Kinx() kinx {
	return br.At
}

func (br BreakRef) String() string {
	if br.At < 0 {
		return "START"
	}
	if br.Variant == 0 {
		return fmt.Sprintf("k-%d", br.At)
	}
	return fmt.Sprintf("k-%d[v:%d]", br.At, br.Variant)
}

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

// Breakpoint returns the index if it is a registered breakpoint, and noinx otherwise.
func (pt *pathTable) Breakpoint(bp BreakRef) BreakRef {
	if _, ok := pt.feasBP[bp]; ok {
		return bp
	}
	return NoBreakRef
}

// Pred returns the currently stored predecessor for a reachable (to,line) state.
func (pt *pathTable) Pred(to BreakRef, line lineNo) (predecessor, bool) {
	pred, ok := pt.preds[pathState{to: to, line: line}]
	return pred, ok
}

// SetPred stores the best known predecessor for a reachable (to,line) state.
// If a predecessor already exists, it is only replaced when the new total cost is cheaper.
// Ties are left stable because choosing either predecessor is acceptable.
func (pt *pathTable) SetPred(from, to BreakRef, cost, total merits, line lineNo) bool {
	if pt.Breakpoint(to) == NoBreakRef {
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
