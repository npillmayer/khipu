package khipu

import (
	"fmt"
	"github.com/npillmayer/khipu/dimen"
	"slices"
)

// GeometryRequest borrows a stable one-based shape only during search. Skip
// triples are physical geometry; all other fields of the cores are ignored.
type GeometryRequest struct {
	Shape               LineShape
	LeftSkip, RightSkip KnotCore
}

// BreakRequest pairs one immutable binding with its incarnation's discovery.
// Prefix locks previously accepted decisions for supported continuation searches.
type BreakRequest struct {
	Binding   *AtomicBinding
	Discovery *Discovery
	Geometry  GeometryRequest
	Prefix    *BreakCheckpoint
}

// Validate checks handles, incarnation, skip geometry and prefix compatibility.
// It never queries the shape or any service, and never scans prefix knots.
func (r BreakRequest) Validate() error {
	if r.Binding == nil || r.Binding.Incarnation().token == nil || r.Binding.KnotCount() == 0 || !r.Discovery.validFor(r.Binding.material) {
		return ErrIllegalArguments
	}
	if err := (Params{LineShape: r.Geometry.Shape, LeftSkip: r.Geometry.LeftSkip, RightSkip: r.Geometry.RightSkip}).ValidateGeometry(); err != nil {
		return err
	}
	if r.Prefix != nil {
		return r.Prefix.validate(r.Binding)
	}
	return nil
}

// BoundaryKind distinguishes an ordinary endpoint from a replacement split.
type BoundaryKind uint8

const (
	OrdinaryBoundary BoundaryKind = iota + 1
	DiscretionaryBoundary
)

// BreakBoundary is an inclusive base-knot endpoint with optional replacement ID.
type BreakBoundary struct {
	Kind        BoundaryKind
	Knot        int
	Alternative AlternativeID
}

// WidthTriple records natural, minimum and maximum widths in DU.
type WidthTriple struct{ Natural, Minimum, Maximum dimen.DU }

// LineGeometry captures the geometry used to search one line, without callbacks.
type LineGeometry struct {
	TargetWidth         dimen.DU
	LeftSkip, RightSkip WidthTriple
}

// LineDecision is one endpoint and its immutable physical geometry.
type LineDecision struct {
	End      BreakBoundary
	Geometry LineGeometry
}

// Algorithm identifies a resolved breaker policy.
type Algorithm uint8

const (
	FirstFitAlgorithm Algorithm = iota + 1
	KnuthPlassAlgorithm
)

// FirstFitSettings captures first-fit's resolved policy values.
type FirstFitSettings struct{ MinHyphenGain dimen.DU }

// KnuthPlassSettings captures scoring policy; emergency stretch is scoring-only.
type KnuthPlassSettings struct {
	PreTolerance, Tolerance, LinePenalty     int64
	FirstHyphenDemerits, FinalHyphenDemerits int64
	EmergencyStretch                         dimen.DU
}

// PolicyRecord has exactly one active arm, selected by Algorithm.
type PolicyRecord struct {
	Algorithm  Algorithm
	FirstFit   FirstFitSettings
	KnuthPlass KnuthPlassSettings
}

// BreakDiagnostics records available search information. For continuation it
// describes the suffix search, not a newly optimized score for the whole paragraph.
type BreakDiagnostics struct {
	HasScore bool
	Score    int64
	HasPass  bool
	Pass     int
}

// Tikray is an immutable complete paragraph trial. It retains its binding and
// selected replacements, never discovery, shape callbacks or mutable search state.
type Tikray struct {
	binding      *AtomicBinding
	lines        []LineDecision
	replacements map[AlternativeID]*alternative
	policies     []PolicyRecord
	linePolicy   []int
	diagnostics  BreakDiagnostics
}

// Incarnation returns zero for nil/uninitialized plans.
func (p *Tikray) Incarnation() IncarnationID {
	if p == nil {
		return IncarnationID{}
	}
	return p.binding.Incarnation()
}

// Lines returns copied complete decisions, including any locked prefix.
func (p *Tikray) Lines() []LineDecision {
	if p == nil {
		return nil
	}
	return slices.Clone(p.lines)
}

// Policies returns detached effective policy records; use PolicyForLine to map lines.
func (p *Tikray) Policies() []PolicyRecord {
	if p == nil {
		return nil
	}
	return slices.Clone(p.policies)
}

// Diagnostics returns a value copy, or zero for nil/uninitialized plans.
func (p *Tikray) Diagnostics() BreakDiagnostics {
	if p == nil || p.Incarnation().token == nil {
		return BreakDiagnostics{}
	}
	return p.diagnostics
}

// PolicyForLine uses a zero-based complete-paragraph line index. Invalid indexes
// and nil/zero plans return a zero record and false, without a default policy.
func (p *Tikray) PolicyForLine(i int) (PolicyRecord, bool) {
	if p == nil || p.Incarnation().token == nil || i < 0 || i >= len(p.linePolicy) {
		return PolicyRecord{}, false
	}
	return p.policies[p.linePolicy[i]], true
}
func widthCore(w WidthTriple) KnotCore {
	return KnotCore{W: w.Natural, MinW: w.Minimum, MaxW: w.Maximum}
}
func validLineGeometry(g LineGeometry) bool {
	return g.TargetWidth >= 0 && g.TargetWidth < dimen.Fil && validGeometryWidths(widthCore(g.LeftSkip)) && validGeometryWidths(widthCore(g.RightSkip))
}
func validPolicy(p PolicyRecord) bool {
	switch p.Algorithm {
	case FirstFitAlgorithm:
		return p.KnuthPlass == (KnuthPlassSettings{})
	case KnuthPlassAlgorithm:
		return p.FirstFit == (FirstFitSettings{}) && p.KnuthPlass.EmergencyStretch >= 0 && p.KnuthPlass.EmergencyStretch < dimen.Fil
	}
	return false
}

// NewTikray validates and copies a complete legal path. It resolves only cached
// alternatives and does not query geometry, discover, shape or certify optimality.
// Suffix side skips must match the request; checkpoint-prefix skips remain locked.
// Errors return nil; completed discoveries and previous plans stay reusable.
func NewTikray(r BreakRequest, decisions []LineDecision, policy PolicyRecord, diagnostics BreakDiagnostics) (*Tikray, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if !validPolicy(policy) || len(decisions) == 0 {
		return nil, ErrIllegalArguments
	}
	p := &Tikray{binding: r.Binding, lines: slices.Clone(decisions), replacements: make(map[AlternativeID]*alternative), diagnostics: diagnostics}
	prefix := 0
	if c := r.Prefix; c != nil {
		prefix = len(c.prefix)
		if len(decisions) < prefix || !slices.Equal(decisions[:prefix], c.prefix) {
			return nil, fmt.Errorf("%w: changed locked prefix", ErrIllegalArguments)
		}
		p.policies = slices.Clone(c.policies)
		p.linePolicy = slices.Clone(c.linePolicy)
	}
	suffixPolicy := len(p.policies)
	if len(decisions) > prefix {
		p.policies = append(p.policies, policy)
	}
	leftSkip := WidthTriple{Natural: r.Geometry.LeftSkip.W, Minimum: r.Geometry.LeftSkip.MinW, Maximum: r.Geometry.LeftSkip.MaxW}
	rightSkip := WidthTriple{Natural: r.Geometry.RightSkip.W, Minimum: r.Geometry.RightSkip.MinW, Maximum: r.Geometry.RightSkip.MaxW}
	previous := -1
	for i, line := range p.lines {
		end := line.End
		if !validLineGeometry(line.Geometry) || end.Knot <= previous || end.Knot >= r.Binding.KnotCount() {
			return nil, ErrIllegalArguments
		}
		// Prefix geometry is certified by the checkpoint and may use older skips.
		if i >= prefix && (line.Geometry.LeftSkip != leftSkip || line.Geometry.RightSkip != rightSkip) {
			return nil, fmt.Errorf("%w: suffix side skips differ from request", ErrIllegalArguments)
		}
		for j := previous + 1; j < end.Knot; j++ {
			k, _ := r.Binding.Knot(j)
			if k.Penalty <= MaxMerit {
				return nil, fmt.Errorf("%w: skipped mandatory boundary", ErrIllegalArguments)
			}
		}
		knot, _ := r.Binding.Knot(end.Knot)
		switch end.Kind {
		case OrdinaryBoundary:
			if end.Alternative != (AlternativeID{}) || knot.Penalty >= MaxPenalty {
				return nil, ErrIllegalArguments
			}
		case DiscretionaryBoundary:
			id := end.Alternative
			if id.Incarnation != r.Binding.Incarnation() || id.Source != end.Knot || id.Variant == 0 || knot.Kind != KTTextBox || knot.Penalty <= MaxMerit {
				return nil, ErrIllegalArguments
			}
			var v *alternative
			if i < prefix {
				v = r.Prefix.replacements[id]
			} else {
				v = r.Discovery.lookup(id)
			}
			if v == nil || v.preCore.Penalty >= MaxPenalty {
				return nil, fmt.Errorf("%w: unpublished or prohibited alternative", ErrIllegalArguments)
			}
			p.replacements[id] = v
		default:
			return nil, ErrIllegalArguments
		}
		if i >= prefix {
			p.linePolicy = append(p.linePolicy, suffixPolicy)
		}
		previous = end.Knot
	}
	final := p.lines[len(p.lines)-1].End
	last, _ := r.Binding.Knot(r.Binding.KnotCount() - 1)
	if final.Knot != r.Binding.KnotCount()-1 || final.Kind != OrdinaryBoundary || last.Penalty > MaxMerit {
		return nil, fmt.Errorf("%w: incomplete paragraph path", ErrIllegalArguments)
	}
	return p, nil
}
