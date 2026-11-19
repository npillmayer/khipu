package khipu

import "slices"

// Freeze realizes one immutable trial using its captured geometry and choices.
// It performs no discovery, shaping or geometry callbacks. Frozen output retains
// stable source/shaping storage, but no Khipu, binding, Tikray, checkpoint or cache.
func Freeze(p *Tikray) (*FrozenKhipu, error) {
	if p == nil || p.Incarnation().token == nil || len(p.lines) == 0 {
		return nil, ErrIllegalArguments
	}
	k := p.freezeInput()
	if k.paragraph == nil {
		return nil, ErrIllegalArguments
	}
	breaks := make([]int, len(p.lines))
	for i, l := range p.lines {
		breaks[i] = l.End.Knot
	}
	logical, e := materializeLogicalLines(k, breaks)
	if e != nil {
		return nil, e
	}
	visual, e := logical.visualLines()
	if e != nil {
		return nil, e
	}
	geom, e := visual.prepareCapturedGeometry(p.lines)
	if e != nil {
		return nil, e
	}
	return geom.freeze()
}
func (p *Tikray) freezeInput() *freezeInput {
	b := p.binding
	m := b.material.material
	v := m.planes
	k := &freezeInput{paragraph: m.paragraph, W: slices.Clone(v.w), MinW: slices.Clone(v.minW), MaxW: slices.Clone(v.maxW), Penalty: v.penalty, Pos: v.pos, Len: v.length, Kind: v.kind, Flags: v.flags, boxRefs: m.boxRefs, shaping: m.shaping, DiscretionaryCandidates: make(map[int][]DiscretionaryCandidate), SelectedDiscretionaries: make(map[int]DiscretionarySelection)}
	for i, s := range m.shaping.sources {
		if s.atomic != 0 {
			w := b.advances[b.material.atomicSlots[s.atomic]]
			k.W[i], k.MinW[i], k.MaxW[i] = w, w, w
		}
	}
	// Copy slice headers with capped capacity: appending replacements cannot mutate
	// any immutable backing array. Relocation copies the replacement cluster records.
	a := &k.shaping
	a.boxes = a.boxes[:len(a.boxes):len(a.boxes)]
	a.glyphs = a.glyphs[:len(a.glyphs):len(a.glyphs)]
	a.clusters = a.clusters[:len(a.clusters):len(a.clusters)]
	a.contexts = a.contexts[:len(a.contexts):len(a.contexts)]
	for _, line := range p.lines {
		if line.End.Kind == DiscretionaryBoundary {
			id := line.End.Alternative
			v := p.replacements[id]
			pre, post := a.appendReplacement(v.pre), a.appendReplacement(v.post)
			k.DiscretionaryCandidates[id.Source] = []DiscretionaryCandidate{{Variant: id.Variant, PreBreak: v.preCore, PostBreak: v.postCore, SourceSplit: v.split, preBox: pre, postBox: post}}
			k.SelectedDiscretionaries[id.Source] = DiscretionarySelection{Source: id.Source, Variant: id.Variant}
		}
	}
	return k
}

// BreakpointPositions returns the original paragraph-relative UTF-8 endpoints of
// this complete plan. It performs no discovery and returns detached storage.
func (p *Tikray) BreakpointPositions() ([]uint64, error) {
	if p == nil || p.Incarnation().token == nil {
		return nil, ErrIllegalArguments
	}
	k := p.freezeInput()
	ends := make([]int, len(p.lines))
	for i, l := range p.lines {
		ends[i] = l.End.Knot
	}
	return k.BreakpointPositions(ends)
}
