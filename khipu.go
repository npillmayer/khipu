package khipu

import (
	"fmt"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"slices"
	"strings"
)

// IncarnationID is a comparable process-local material identity. Its zero value
// identifies no paragraph. The token retains no paragraph storage.
type IncarnationID struct{ token *incarnationToken }
type incarnationToken struct{ marker byte }

// Khipu is immutable paragraph material in private aligned SoA planes. Atomic
// slots have no advance until bound. Source, fonts and style services borrowed
// during preparation must stay stable for this incarnation's lifetime.
// A zero Khipu is uninitialized; copy handles, not their implementation storage.
type Khipu struct {
	id          IncarnationID
	material    *preparedMaterial
	hyphenation HyphenationMode
	atomicIDs   []contracts.AtomicID
	atomicSlots map[contracts.AtomicID]int
}

// KnotTemplate describes one base knot. Atomic slots have zero placeholder
// widths and MetricsKnown false, even when a later binding supplies zero.
type KnotTemplate struct {
	Core         KnotCore
	Atomic       contracts.AtomicID
	MetricsKnown bool
}

// ID returns the incarnation, or zero for nil/uninitialized material.
func (k *Khipu) ID() IncarnationID {
	if k == nil {
		return IncarnationID{}
	}
	return k.id
}

// KnotCount returns the number of base knots, including the final trailer.
func (k *Khipu) KnotCount() int {
	if k == nil || k.material == nil {
		return 0
	}
	return len(k.material.planes.kind)
}

// Knot returns a detached template; invalid indexes return zero and false.
func (k *Khipu) Knot(i int) (KnotTemplate, bool) {
	if i < 0 || i >= k.KnotCount() {
		return KnotTemplate{}, false
	}
	core := k.material.planes.knot(i)
	var id contracts.AtomicID
	if i < len(k.material.shaping.sources) {
		id = k.material.shaping.sources[i].atomic
	}
	return KnotTemplate{core, id, id == 0}, true
}

// AtomicIDs returns occurrence IDs in canonical source order in detached storage.
func (k *Khipu) AtomicIDs() []contracts.AtomicID {
	if k == nil {
		return nil
	}
	return slices.Clone(k.atomicIDs)
}

// HasExplicitDiscretionary inspects retained soft-hyphen metadata, without
// discovery or shaping. False includes invalid indexes and NoHyphenation.
func (k *Khipu) HasExplicitDiscretionary(i int) bool {
	t, ok := k.Knot(i)
	if !ok || t.Core.Kind != KTTextBox || k.hyphenation == NoHyphenation {
		return false
	}
	a := k.material.shaping
	if i >= len(a.sources) {
		return false
	}
	s := a.sources[i].source
	if s.scalarEnd > uint64(len(a.boundaries)) {
		return false
	}
	for j := s.scalarStart; j < s.scalarEnd; j++ {
		if a.boundaries[j].softHyphen {
			return true
		}
	}
	return false
}

// String returns diagnostic knot descriptions in logical source order.
func (k *Khipu) String() string {
	var b strings.Builder
	for i := 0; i < k.KnotCount(); i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		t, _ := k.Knot(i)
		b.WriteString(t.Core.String())
	}
	return b.String()
}

// AtomicBinding combines shared immutable base material with copied advances.
// It owns no frame width or search decisions and performs no service calls.
type AtomicBinding struct {
	material *Khipu
	advances []dimen.DU
}

// BindParagraph supplies exactly one finite signed advance per atomic occurrence
// in any order. Failure returns nil; neither inputs nor material are changed.
func BindParagraph(k *Khipu, input []AtomicAdvance) (*AtomicBinding, error) {
	if k == nil || k.ID().token == nil || k.KnotCount() == 0 || len(input) != len(k.atomicIDs) {
		return nil, ErrIllegalArguments
	}
	values := make(map[contracts.AtomicID]dimen.DU, len(input))
	for _, v := range input {
		if _, ok := values[v.ID]; ok || v.ID == 0 || v.Advance <= -dimen.Fil || v.Advance >= dimen.Fil {
			return nil, ErrIllegalArguments
		}
		values[v.ID] = v.Advance
	}
	b := &AtomicBinding{material: k, advances: make([]dimen.DU, len(k.atomicIDs))}
	for i, id := range k.atomicIDs {
		v, ok := values[id]
		if !ok {
			return nil, fmt.Errorf("%w: missing atomic %d", ErrIllegalArguments, id)
		}
		b.advances[i] = v
	}
	return b, nil
}

// Incarnation returns the binding's material identity, or zero if uninitialized.
func (b *AtomicBinding) Incarnation() IncarnationID {
	if b == nil {
		return IncarnationID{}
	}
	return b.material.ID()
}

// KnotCount includes the paragraph-final trailer; uninitialized bindings return zero.
func (b *AtomicBinding) KnotCount() int {
	if b == nil {
		return 0
	}
	return b.material.KnotCount()
}

// Knot overlays atomic metrics on a detached base snapshot. Invalid indexes
// return zero and false; valid bound atomics always have MinW == W == MaxW.
func (b *AtomicBinding) Knot(i int) (KnotCore, bool) {
	if b == nil {
		return KnotCore{}, false
	}
	t, ok := b.material.Knot(i)
	if !ok {
		return KnotCore{}, false
	}
	if t.Atomic != 0 {
		w := b.advances[b.material.atomicSlots[t.Atomic]]
		t.Core.W, t.Core.MinW, t.Core.MaxW = w, w, w
	}
	return t.Core, true
}

// AtomicAdvances returns canonical copied ID/advance pairs.
func (b *AtomicBinding) AtomicAdvances() []AtomicAdvance {
	if b == nil || b.material == nil {
		return nil
	}
	r := make([]AtomicAdvance, len(b.advances))
	for i, w := range b.advances {
		r[i] = AtomicAdvance{ID: b.material.atomicIDs[i], Advance: w}
	}
	return r
}

// HasExplicitDiscretionary delegates to immutable source metadata only.
func (b *AtomicBinding) HasExplicitDiscretionary(i int) bool {
	return b != nil && b.material.HasExplicitDiscretionary(i)
}

// knotPlanes is the immutable base SoA after compiler publication.
type knotPlanes struct {
	w, minW, maxW []dimen.DU
	penalty       []Penalty
	pos           []uint64
	length        []uint16
	kind          []KnotType
	flags         []KnotFlags
}

func (p *knotPlanes) append(k KnotCore) {
	p.w = append(p.w, k.W)
	p.minW = append(p.minW, k.MinW)
	p.maxW = append(p.maxW, k.MaxW)
	p.penalty = append(p.penalty, k.Penalty)
	p.pos = append(p.pos, k.Pos)
	p.length = append(p.length, k.Len)
	p.kind = append(p.kind, k.Kind)
	p.flags = append(p.flags, k.Flags)
}
func (p *knotPlanes) knot(i int) KnotCore {
	return KnotCore{W: p.w[i], MinW: p.minW[i], MaxW: p.maxW[i], Penalty: p.penalty[i], Pos: p.pos[i], Len: p.length[i], Kind: p.kind[i], Flags: p.flags[i]}
}
func assert(ok bool, msg string) {
	if !ok {
		panic(msg)
	}
}
