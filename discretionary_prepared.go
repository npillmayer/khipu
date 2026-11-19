package khipu

import (
	"context"
	"fmt"
	"github.com/npillmayer/uax/bidi"
	textbidi "golang.org/x/text/unicode/bidi"
	"io"
	"sync"
)

// AlternativeID is stable within one incarnation and original textbox. Variant
// zero is reserved for absence and is never published by discovery.
type AlternativeID struct {
	Incarnation IncarnationID
	Source      int
	Variant     uint16
}

// Alternative is a detached metrics/provenance view of a complete replacement pair.
type Alternative struct {
	ID                  AlternativeID
	PreBreak, PostBreak KnotCore
	SourceSplit         SourceSplit
}
type alternative struct {
	id                AlternativeID
	preCore, postCore KnotCore
	split             SourceSplit
	pre, post         shapingArena
}

// AlternativeSet is an immutable, complete batch, including successful emptiness.
type AlternativeSet struct {
	incarnation       IncarnationID
	source, positions int
	alternatives      []*alternative
}

// IDs returns a detached ordered list, preserving provider tie order.
func (a *AlternativeSet) IDs() []AlternativeID {
	if a == nil {
		return nil
	}
	r := make([]AlternativeID, len(a.alternatives))
	for i, v := range a.alternatives {
		r[i] = v.id
	}
	return r
}

// Candidates returns copied metrics and provenance, without mutable glyph storage.
func (a *AlternativeSet) Candidates() []Alternative {
	if a == nil {
		return nil
	}
	r := make([]Alternative, len(a.alternatives))
	for i, v := range a.alternatives {
		r[i] = Alternative{v.id, v.preCore, v.postCore, v.split}
	}
	return r
}

// Discovery memoizes complete immutable alternatives for one incarnation. Misses
// are serialized; failures publish nothing and remain retryable. Borrowed shaping
// and hyphenation collaborators must remain stable and obey their own concurrency
// contracts across different discovery facilities. Do not copy Discovery.
type Discovery struct {
	material *Khipu
	recipe   *Khipukamayuq
	gate     sync.Mutex
	mu       sync.Mutex
	batches  map[int]*AlternativeSet
}

// Alternatives discovers the complete batch for one source knot, synchronously.
// NoHyphenation and ineligible knots yield cached empty sets. Invalid handles or
// indexes fail. Successful empty batches are cached; errors are not.
func (d *Discovery) Alternatives(at int) (*AlternativeSet, error) { return d.alternatives(at, false) }
func (d *Discovery) validFor(k *Khipu) bool {
	return d != nil && k != nil && k.ID().token != nil && d.material.ID() == k.ID() && d.batches != nil
}
func (d *Discovery) cached(at int) *AlternativeSet {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.batches[at]
}
func (a *AlternativeSet) measurable() error {
	if a.positions > 1 {
		return fmt.Errorf("%w: multiple explicit positions", ErrUnsupportedPolicy)
	}
	n := 0
	for _, v := range a.alternatives {
		if v.preCore.Penalty < MaxPenalty {
			n++
		}
	}
	if n > 1 {
		return fmt.Errorf("%w: multiple permitting explicit variants", ErrUnsupportedPolicy)
	}
	return nil
}
func (d *Discovery) alternatives(at int, measurement bool) (*AlternativeSet, error) {
	if d == nil || !d.validFor(d.material) || at < 0 || at >= d.material.KnotCount() {
		return nil, ErrIllegalArguments
	}
	check := func(a *AlternativeSet) (*AlternativeSet, error) {
		if measurement {
			if e := a.measurable(); e != nil {
				return nil, e
			}
		}
		return a, nil
	}
	if a := d.cached(at); a != nil {
		return check(a)
	}
	d.gate.Lock()
	defer d.gate.Unlock()
	if a := d.cached(at); a != nil {
		return check(a)
	}
	a, e := d.discover(at, measurement)
	if e != nil {
		return nil, e
	}
	d.mu.Lock()
	d.batches[at] = a
	d.mu.Unlock()
	return a, nil
}
func (d *Discovery) discover(at int, measurement bool) (*AlternativeSet, error) {
	k := d.material
	m := k.material
	a := &AlternativeSet{incarnation: k.ID(), source: at}
	t, _ := k.Knot(at)
	if k.hyphenation == NoHyphenation || t.Core.Kind != KTTextBox {
		return a, nil
	}
	explicit := k.HasExplicitDiscretionary(at)
	if !explicit && (k.hyphenation != AutomaticHyphenation || !k.automaticHyphenationEligible(at)) {
		return a, nil
	}
	if at >= len(m.boxRefs) || m.boxRefs[at] <= 0 || int(m.boxRefs[at]) > len(m.shaping.boxes) {
		return nil, ErrIllegalArguments
	}
	box := m.shaping.boxes[m.boxRefs[at]-1]
	if box.replacement != nil || box.context < 0 || box.context >= len(m.shaping.contexts) || box.source != m.shaping.sources[at].source {
		return nil, ErrIllegalArguments
	}
	run, s := m.shaping.contexts[box.context], box.source
	if s.scalarStart < run.source.scalarStart || s.scalarEnd > run.source.scalarEnd {
		return nil, ErrIllegalArguments
	}
	raw, e := io.ReadAll(m.paragraph.Raw().BoundedReader(s.byteStart, s.byteEnd))
	if e != nil {
		return nil, e
	}
	if explicit {
		positions, e := explicitSplitPositions(string(raw))
		if e != nil {
			return nil, e
		}
		a.positions = len(positions)
		if measurement && a.positions > 1 {
			return nil, a.measurable()
		}
		if a.positions == 0 {
			return a, nil
		}
	}
	q := d.recipe
	if q == nil || q.env.hyphenator == nil {
		return a, nil
	}
	var inputs []preparedDiscretionary
	if explicit {
		inputs, e = q.prepareExplicitDiscretionaries(string(raw), s)
	} else {
		policy := *q.env.params
		policy.Language, policy.Script = run.params.shaping.Language, run.params.shaping.Script
		policy.BidiDir = bidi.LeftToRight
		if run.params.shaping.Direction == textbidi.RightToLeft {
			policy.BidiDir = bidi.RightToLeft
		}
		inputs, e = q.prepareAutomaticDiscretionaries(string(raw), s, policy)
	}
	if e != nil {
		return nil, e
	}
	if len(inputs) == 0 {
		return a, nil
	}
	params, e := replacementParameters(run, s)
	if e != nil {
		return nil, e
	}
	if measurement {
		n := 0
		for _, in := range inputs {
			if in.penalty < MaxPenalty {
				n++
			}
		}
		if n > 1 {
			return nil, fmt.Errorf("%w: multiple permitting explicit variants", ErrUnsupportedPolicy)
		}
	}
	cfg := q.env.streaming
	if cfg == nil || cfg.NewShaper == nil {
		return nil, ErrStreamingDiscretionariesUnsupported
	}
	for _, in := range inputs {
		pair, e := shapeDiscretionaryInput(context.Background(), in.input, params, params, cfg.NewShaper, cfg.BufferOptions)
		if e != nil {
			return nil, e
		}
		v, e := makeAlternative(k, at, in.variant, in.penalty, pair)
		if e != nil {
			return nil, e
		}
		a.alternatives = append(a.alternatives, v)
	}
	return a, nil
}
func makeAlternative(k *Khipu, at int, variant uint16, penalty Penalty, pair *shapedDiscretionaryInput) (*alternative, error) {
	if pair == nil || variant == 0 {
		return nil, ErrIllegalArguments
	}
	s, a, b := pair.source, pair.pre.input.provenance, pair.post.input.provenance
	if s != k.material.shaping.sources[at].source || !pair.split.Valid || a.byteStart != s.byteStart || a.scalarStart != s.scalarStart || b.byteEnd != s.byteEnd || b.scalarEnd != s.scalarEnd || a.byteEnd != pair.split.ByteOffset || b.byteStart != pair.split.ByteOffset || a.scalarEnd != b.scalarStart {
		return nil, ErrIllegalArguments
	}
	pre, pc, e := prepareStoredReplacement(pair.pre, k.material.paragraph.Offset)
	if e != nil {
		return nil, e
	}
	post, qc, e := prepareStoredReplacement(pair.post, k.material.paragraph.Offset)
	if e != nil {
		return nil, e
	}
	pc.Penalty = penalty
	return &alternative{AlternativeID{k.ID(), at, variant}, pc, qc, pair.split, pre, post}, nil
}

// lookup resolves only already-published alternatives; publication never discovers.
func (d *Discovery) lookup(id AlternativeID) *alternative {
	if d == nil || id.Incarnation != d.material.ID() {
		return nil
	}
	a := d.cached(id.Source)
	if a != nil {
		for _, v := range a.alternatives {
			if v.id == id {
				return v
			}
		}
	}
	return nil
}
