package khipu

import (
	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
	"slices"
)

type FixturePrepared struct {
	*preparedMaterial
	knots       []KnotCore
	owner       *Khipukamayuq
	atomicCount int
	hyphenation HyphenationMode
	material    *Khipu
	discovery   *Discovery
	explicit    map[int]explicitBatch
}

func fixturePrepared(q *Khipukamayuq, k *Khipu, d *Discovery) *FixturePrepared {
	p := &FixturePrepared{preparedMaterial: k.material, owner: q, atomicCount: len(k.atomicIDs), hyphenation: k.hyphenation, material: k, discovery: d}
	for i := 0; i < k.KnotCount(); i++ {
		v, _ := k.Knot(i)
		p.knots = append(p.knots, v.Core)
	}
	return p
}
func (q *Khipukamayuq) PrepareParagraphFixture(in ParagraphInput, o PrepareOptions) (*FixturePrepared, error) {
	k, d, e := q.PrepareParagraph(in, o)
	if e != nil {
		return nil, e
	}
	return fixturePrepared(q, k, d), nil
}
func fixtureBinding(q *Khipukamayuq, k *Khipu, d *Discovery, b *AtomicBinding) *FixtureKhipu {
	f := q.allocKhipu(k.KnotCount())
	f.material, f.discovery, f.binding = k, d, b
	f.paragraph = k.material.paragraph
	f.boxRefs = slices.Clone(k.material.boxRefs)
	f.shaping = k.material.shaping
	f.shaping.boxes = slices.Clone(f.shaping.boxes)
	f.shaping.glyphs = slices.Clone(f.shaping.glyphs)
	f.shaping.clusters = slices.Clone(f.shaping.clusters)
	f.shaping.contexts = slices.Clone(f.shaping.contexts)
	f.shaping.sources = slices.Clone(f.shaping.sources)
	f.boxRefs = nil
	src := f.shaping.sources
	f.shaping.sources = nil
	for i := 0; i < b.KnotCount(); i++ {
		c, _ := b.Knot(i)
		f.appendKnot(c)
	}
	f.boxRefs = slices.Clone(k.material.boxRefs)
	f.shaping.sources = src
	f.hyphenationPolicy = true
	f.hyphenation = k.hyphenation
	return f
}
func (q *Khipukamayuq) EncodeParagraphFixture(para *styled.Paragraph) (*FixtureKhipu, error) {
	k, d, e := q.EncodeParagraph(para)
	if e != nil {
		return nil, e
	}
	b, e := BindParagraph(k, nil)
	if e != nil {
		return nil, e
	}
	return fixtureBinding(q, k, d, b), nil
}
func (q *Khipukamayuq) BindParagraphFixture(p *FixturePrepared, a []AtomicAdvance) (*FixtureKhipu, error) {
	if q == nil {
		return nil, ErrVoidKhipukamayuq
	}
	if p == nil || p.owner != q || p.material == nil {
		return nil, ErrIllegalArguments
	}
	b, e := BindParagraph(p.material, a)
	if e != nil {
		return nil, e
	}
	f := fixtureBinding(q, p.material, p.discovery, b)
	f.prepared = p
	f.boundMaterial = p.preparedMaterial
	return f, nil
}
func (q *Khipukamayuq) MeasureIntrinsicFixture(p *FixturePrepared, r MeasureRequest) (IntrinsicWidths, error) {
	if q == nil {
		return IntrinsicWidths{}, ErrVoidKhipukamayuq
	}
	if p == nil || p.owner != q || p.material == nil {
		return IntrinsicWidths{}, ErrIllegalArguments
	}
	defer p.syncBatches()
	return MeasureIntrinsic(p.material, p.discovery, r)
}
func (q *Khipukamayuq) publishCandidateParagraph(c *candidateParagraph) (*FixtureKhipu, error) {
	m, e := compileCandidateParagraph(c)
	if e != nil {
		return nil, e
	}
	k := &Khipu{id: IncarnationID{new(incarnationToken)}, material: m, hyphenation: AutomaticHyphenation}
	d := &Discovery{material: k, recipe: q, batches: make(map[int]*AlternativeSet)}
	b, e := BindParagraph(k, nil)
	if e != nil {
		return nil, e
	}
	return fixtureBinding(q, k, d, b), nil
}
func (q *Khipukamayuq) DiscoverFixture(k *FixtureKhipu, i int) ([]DiscretionaryCandidate, error) {
	if q == nil {
		return nil, ErrVoidKhipukamayuq
	}
	if k == nil || i < 0 || i >= len(k.Kind) || k.owner != nil && k.owner != q {
		return nil, ErrIllegalArguments
	}
	if c := k.DiscretionariesAt(i); len(c) > 0 {
		return c, nil
	}
	if k.discovery == nil {
		return nil, nil
	}
	if k.prepared != nil {
		defer k.prepared.syncBatches()
	}
	a, e := k.discovery.Alternatives(i)
	if e != nil {
		return nil, e
	}
	for _, v := range a.alternatives {
		pre, post := k.shaping.appendReplacement(v.pre), k.shaping.appendReplacement(v.post)
		for j := range k.shaping.contexts {
			k.shaping.contexts[j].params.shaping.Features = slices.Clone(k.shaping.contexts[j].params.shaping.Features)
		}
		k.AddDiscretionaryCandidate(i, DiscretionaryCandidate{Variant: v.id.Variant, PreBreak: v.preCore, PostBreak: v.postCore, SourceSplit: v.split, preBox: pre, postBox: post})
	}
	return k.DiscretionariesAt(i), nil
}
func (q *Khipukamayuq) FreezeFixture(k *FixtureKhipu, path []int, params Params) (*FrozenKhipu, error) {
	if q == nil {
		return nil, ErrVoidKhipukamayuq
	}
	if q == nil {
		return nil, ErrVoidKhipukamayuq
	}
	if k == nil || k.owner != q {
		return nil, ErrIllegalArguments
	}
	p, e := materializeLineGeometry(k.freezeInput(), path, params)
	if e != nil {
		return nil, e
	}
	return p.freeze()
}
func (q *Khipukamayuq) FreezeParagraphFixture(k *FixtureKhipu, path []int, params Params) (*FrozenKhipu, error) {
	return q.FreezeFixture(k, path, params)
}

func (q *Khipukamayuq) materializeLogicalLinesFixture(k *FixtureKhipu, path []int) (*logicalParagraph, error) {
	if q == nil {
		return nil, ErrVoidKhipukamayuq
	}
	if k == nil || k.owner != q {
		return nil, ErrIllegalArguments
	}
	return materializeLogicalLines(k.freezeInput(), path)
}
func (q *Khipukamayuq) materializeVisualLinesFixture(k *FixtureKhipu, path []int) (*visualParagraph, error) {
	if q == nil {
		return nil, ErrVoidKhipukamayuq
	}
	if k == nil || k.owner != q {
		return nil, ErrIllegalArguments
	}
	return materializeVisualLines(k.freezeInput(), path)
}
func (q *Khipukamayuq) materializeLineGeometryFixture(k *FixtureKhipu, path []int, params Params) (*geometryParagraph, error) {
	if q == nil {
		return nil, ErrVoidKhipukamayuq
	}
	if k == nil || k.owner != q {
		return nil, ErrIllegalArguments
	}
	return materializeLineGeometry(k.freezeInput(), path, params)
}

type explicitBatch struct {
	positions    int
	replacements []explicitReplacement
}
type explicitReplacement struct {
	variant uint16
	penalty Penalty
	pair    *shapedDiscretionaryInput
}

func (p *FixturePrepared) syncBatches() {
	if p.explicit == nil {
		p.explicit = make(map[int]explicitBatch)
	}
	for i, a := range p.discovery.batches {
		if _, ok := p.explicit[i]; ok {
			continue
		}
		b := explicitBatch{positions: a.positions}
		for _, v := range a.alternatives {
			cv := func(a shapingArena, c KnotCore) shapedReplacement {
				return shapedReplacement{input: *a.boxes[0].replacement, params: a.contexts[0].params, glyphs: a.glyphs, widths: candidateWidths{W: c.W, MinW: c.MinW, MaxW: c.MaxW}}
			}
			b.replacements = append(b.replacements, explicitReplacement{v.id.Variant, v.preCore.Penalty, &shapedDiscretionaryInput{source: p.shaping.sources[i].source, split: v.split, pre: cv(v.pre, v.preCore), post: cv(v.post, v.postCore)}})
		}
		p.explicit[i] = b
	}
}
func (p *FixturePrepared) explicitReplacements(i int, measurement bool) (explicitBatch, error) {
	_, e := p.discovery.alternatives(i, measurement)
	p.syncBatches()
	return p.explicit[i], e
}

type fixtureShape struct{}

func (fixtureShape) LineLength(int32) dimen.DU { return 100 * dimen.BP }

type FixtureCheckpoint struct{ checkpoint *BreakCheckpoint }

func (q *Khipukamayuq) CheckpointFixture(p *FixturePrepared, path []int, n int) (*FixtureCheckpoint, error) {
	if q == nil || p == nil || p.owner != q || p.material == nil {
		return nil, ErrIllegalArguments
	}
	if p.hyphenation != NoHyphenation {
		return nil, ErrUnsupportedPolicy
	}
	b, e := BindParagraph(p.material, nil)
	if e != nil {
		return nil, e
	}
	r := BreakRequest{Binding: b, Discovery: p.discovery, Geometry: GeometryRequest{Shape: fixtureShape{}}}
	lines := make([]LineDecision, len(path))
	for i, k := range path {
		lines[i] = LineDecision{End: BreakBoundary{Kind: OrdinaryBoundary, Knot: k}, Geometry: LineGeometry{TargetWidth: 100 * dimen.BP}}
	}
	plan, e := NewTikray(r, lines, PolicyRecord{Algorithm: FirstFitAlgorithm}, BreakDiagnostics{})
	if e != nil {
		return nil, e
	}
	c, e := Checkpoint(plan, n)
	if e != nil {
		return nil, e
	}
	return &FixtureCheckpoint{c}, nil
}
func (c *FixtureCheckpoint) PrefixForFixture(k *FixtureKhipu) ([]int, error) {
	if c == nil || c.checkpoint == nil || k == nil || k.binding == nil || len(k.W) != k.binding.KnotCount() {
		return nil, ErrIllegalArguments
	}
	if k.hyphenation != NoHyphenation {
		return nil, ErrUnsupportedPolicy
	}
	s, e := c.checkpoint.SeedFor(k.binding)
	if e != nil {
		return nil, e
	}
	ends := make([]int, len(s.Prefix))
	for i, l := range s.Prefix {
		ends[i] = l.End.Knot
	}
	return ends, nil
}

// RequestForTest exposes only the new immutable request for integration helpers.
func (k *FixtureKhipu) RequestForTest(params Params) BreakRequest {
	return BreakRequest{Binding: k.binding, Discovery: k.discovery, Geometry: GeometryRequest{Shape: params.LineShape, LeftSkip: params.LeftSkip, RightSkip: params.RightSkip}}
}
func (k *FixtureKhipu) ApplyPlanForTest(p *Tikray) []int {
	clear(k.SelectedDiscretionaries)
	for at := range k.discovery.batches {
		_, _ = k.owner.DiscoverFixture(k, at)
	}
	lines := p.Lines()
	ends := make([]int, len(lines))
	for i, l := range lines {
		ends[i] = l.End.Knot
		if l.End.Kind == DiscretionaryBoundary {
			_, _ = k.owner.DiscoverFixture(k, l.End.Knot)
			k.SelectDiscretionary(l.End.Knot, DiscretionarySelection{Source: l.End.Knot, Variant: l.End.Alternative.Variant})
		}
	}
	return ends
}

// materializeLineGeometry uses the SAME shape and skip values as the completed
// breaker run. Callers must keep them stable: breakpoint indices do not encode
// geometry, so we cannot detect a different but structurally valid parameter set.
// No breaker defaults are substituted. The result is private until every stage
// succeeds; failure does not change the elastic khipu or selected discretionaries.
func materializeLineGeometry(k *freezeInput, breaks []int, params Params) (*geometryParagraph, error) {
	p, err := materializeVisualLines(k, breaks)
	if err != nil {
		return nil, err
	}
	return p.prepareGeometry(params)
}

// prepareGeometry snapshots one target per line in logical paragraph order,
// independent of each line's visual direction. A caller-supplied shape must be
// stable and side-effect free; calls already made cannot be undone on failure.
// The result never retains the shape or aliases mutable line/knot slices.
//
// Finite natural widths and targets must stay below dimen.Fil. Fill orders in
// MaxW are retained per item, never numerically summed as capacities. Oversized
// totals and inconsistent width triples fail rather than saturate as breaker
// summaries do. Negative finite skip widths are allowed if the total remains
// nonnegative. Fit checks and allocation belong to the next stage.
func (p *visualParagraph) prepareGeometry(params Params) (*geometryParagraph, error) {
	if err := params.ValidateGeometry(); err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrIllegalArguments
	}
	lines := make([]LineDecision, len(p.lines))
	for i := range lines {
		lines[i].Geometry = LineGeometry{params.LineShape.LineLength(int32(i + 1)), WidthTriple{params.LeftSkip.W, params.LeftSkip.MinW, params.LeftSkip.MaxW}, WidthTriple{params.RightSkip.W, params.RightSkip.MinW, params.RightSkip.MaxW}}
	}
	g, err := p.prepareCapturedGeometry(lines)
	if err != nil {
		return nil, err
	}
	for i := range g.lines {
		g.lines[i].leftSkip = params.LeftSkip
		g.lines[i].rightSkip = params.RightSkip
	}
	return g, nil
}

// materializeVisualLines connects a complete breaker decision to visual content.
// It inherits materializeLogicalLines' ownership and selection-validity contract.
// No shaping, discretionary discovery, side-skip insertion or width allocation
// occurs here. Errors return no partial paragraph and do not change the input.
func materializeVisualLines(k *freezeInput, breakpoints []int) (*visualParagraph, error) {
	p, err := materializeLogicalLines(k, breakpoints)
	if err != nil {
		return nil, err
	}
	return p.visualLines()
}
