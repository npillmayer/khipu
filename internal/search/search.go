// Package search contains invocation-local breaker storage, never published material.
package search

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

// Material combines read-only binding scans with a private candidate working set.
// Its maps are private to a single search, including all KP passes.
type Material struct {
	Binding                 *khipu.AtomicBinding
	Discovery               *khipu.Discovery
	DiscretionaryCandidates map[int][]khipu.DiscretionaryCandidate
	SelectedDiscretionaries map[int]khipu.DiscretionarySelection
}

// New allocates search-local maps, without copying base knot planes.
func New(b *khipu.AtomicBinding, d *khipu.Discovery) *Material {
	return &Material{b, d, make(map[int][]khipu.DiscretionaryCandidate), make(map[int]khipu.DiscretionarySelection)}
}

// KnotCount reports the number of immutable base knots.
func (m *Material) KnotCount() int { return m.Binding.KnotCount() }

// KnotByIndex is an internal scan convenience for validated indexes.
func (m *Material) KnotByIndex(i int) khipu.KnotCore { k, _ := m.Binding.Knot(i); return k }

// HasExplicitDiscretionary performs only the retained metadata check.
func (m *Material) HasExplicitDiscretionary(i int) bool { return m.Binding.HasExplicitDiscretionary(i) }

// DiscretionariesAt reads this search's working set.
func (m *Material) DiscretionariesAt(i int) []khipu.DiscretionaryCandidate {
	return m.DiscretionaryCandidates[i]
}

// AddDiscretionaryCandidate appends a value only to the local working set.
func (m *Material) AddDiscretionaryCandidate(i int, c khipu.DiscretionaryCandidate) bool {
	m.DiscretionaryCandidates[i] = append(m.DiscretionaryCandidates[i], c)
	return true
}

// SelectDiscretionary records a provisional choice for plan publication.
func (m *Material) SelectDiscretionary(i int, c khipu.DiscretionarySelection) bool {
	m.SelectedDiscretionaries[i] = c
	return true
}

// Provider adapts immutable discovery to the invocation-local evaluator.
type Provider struct{ Discovery *khipu.Discovery }

// DiscretionaryCandidates copies only metrics into the private working set.
func (p Provider) DiscretionaryCandidates(m Paragraph, i int) ([]khipu.DiscretionaryCandidate, error) {
	a, e := p.Discovery.Alternatives(i)
	if e != nil {
		return nil, e
	}
	values := a.Candidates()
	r := make([]khipu.DiscretionaryCandidate, len(values))
	for i, v := range values {
		r[i] = khipu.DiscretionaryCandidate{Variant: v.ID.Variant, PreBreak: v.PreBreak, PostBreak: v.PostBreak, SourceSplit: v.SourceSplit}
	}
	return r, nil
}

// Widths captures each line's first validated target, shared across search passes.
// Prefix widths are imported from certified decisions and never queried again.
type Widths struct {
	Original khipu.LineShape
	values   map[int32]dimen.DU
	Err      error
}

// NewWidths initializes invocation-local memoization from an optional prefix.
func NewWidths(shape khipu.LineShape, prefix []khipu.LineDecision) *Widths {
	w := &Widths{Original: shape, values: make(map[int32]dimen.DU)}
	for i, v := range prefix {
		w.values[int32(i+1)] = v.Geometry.TargetWidth
	}
	return w
}

// LineLength memoizes a valid width; invalid inputs latch an error and return zero
// solely to unwind existing numeric search code. The caller must check Err.
func (w *Widths) LineLength(i int32) dimen.DU {
	if w.Err != nil {
		return 0
	}
	if n, ok := w.values[i]; ok {
		return n
	}
	if i < 1 {
		w.Err = khipu.ErrIllegalArguments
		return 0
	}
	n := w.Original.LineLength(i)
	if n < 0 || n >= dimen.Fil {
		w.Err = khipu.ErrIllegalArguments
		return 0
	}
	w.values[i] = n
	return n
}

// Layout provides the existing private arithmetic with memoized geometry only.
func (w *Widths) Layout(g khipu.GeometryRequest) khipu.Params {
	return khipu.Params{LineShape: w, LeftSkip: g.LeftSkip, RightSkip: g.RightSkip}
}

// Decisions captures selected endpoints and geometry without exposing work maps.
func (w *Widths) Decisions(r khipu.BreakRequest, ends []int, selections map[int]khipu.DiscretionarySelection, prefix []khipu.LineDecision) ([]khipu.LineDecision, error) {
	out := make([]khipu.LineDecision, len(ends))
	copy(out, prefix)
	for i := len(prefix); i < len(ends); i++ {
		at := ends[i]
		end := khipu.BreakBoundary{Kind: khipu.OrdinaryBoundary, Knot: at}
		if c, ok := selections[at]; ok {
			end.Kind = khipu.DiscretionaryBoundary
			end.Alternative = khipu.AlternativeID{Incarnation: r.Binding.Incarnation(), Source: at, Variant: c.Variant}
		}
		g := r.Geometry
		out[i] = khipu.LineDecision{End: end, Geometry: khipu.LineGeometry{TargetWidth: w.LineLength(int32(i + 1)), LeftSkip: khipu.WidthTriple{Natural: g.LeftSkip.W, Minimum: g.LeftSkip.MinW, Maximum: g.LeftSkip.MaxW}, RightSkip: khipu.WidthTriple{Natural: g.RightSkip.W, Minimum: g.RightSkip.MinW, Maximum: g.RightSkip.MaxW}}}
	}
	if w.Err != nil {
		return nil, w.Err
	}
	return out, nil
}

// Paragraph is the internal read-only scan/local workspace seam. Test fixtures
// implement it independently; no mutable base planes cross the public API.
type Paragraph interface {
	KnotCount() int
	KnotByIndex(int) khipu.KnotCore
	HasExplicitDiscretionary(int) bool
	DiscretionariesAt(int) []khipu.DiscretionaryCandidate
	AddDiscretionaryCandidate(int, khipu.DiscretionaryCandidate) bool
	SelectDiscretionary(int, khipu.DiscretionarySelection) bool
	Selections() map[int]khipu.DiscretionarySelection
	Candidates() map[int][]khipu.DiscretionaryCandidate
}

func (m *Material) Selections() map[int]khipu.DiscretionarySelection {
	return m.SelectedDiscretionaries
}
func (m *Material) Candidates() map[int][]khipu.DiscretionaryCandidate {
	return m.DiscretionaryCandidates
}

// WidthError reads a latched validation failure without another shape query.
func WidthError(shape khipu.LineShape) error {
	if w, ok := shape.(*Widths); ok {
		return w.Err
	}
	return nil
}
