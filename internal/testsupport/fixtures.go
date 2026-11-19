// Package testsupport provides detached mutable metric fixtures for algorithm
// regressions. It is imported only by tests; fixtures never become production Khipus.
package testsupport

import (
	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/internal/search"
	"strings"
)

type Paragraph struct {
	Plans                   map[*int]*khipu.Tikray
	W, MinW, MaxW           []dimen.DU
	Penalty                 []khipu.Penalty
	Pos                     []uint64
	Len                     []uint16
	Kind                    []khipu.KnotType
	Flags                   []khipu.KnotFlags
	DiscretionaryCandidates map[int][]khipu.DiscretionaryCandidate
	SelectedDiscretionaries map[int]khipu.DiscretionarySelection
	Binding                 *khipu.AtomicBinding
	Discovery               *khipu.Discovery
	Material                *khipu.Khipu
	Plan                    *khipu.Tikray
}

func (p *Paragraph) KnotCount() int { return len(p.Kind) }
func (p *Paragraph) KnotByIndex(i int) khipu.KnotCore {
	if i < 0 || i >= len(p.Kind) {
		return khipu.KnotCore{}
	}
	return khipu.KnotCore{W: p.W[i], MinW: p.MinW[i], MaxW: p.MaxW[i], Penalty: p.Penalty[i], Pos: p.Pos[i], Len: p.Len[i], Kind: p.Kind[i], Flags: p.flagsAt(i)}
}
func (p *Paragraph) flagsAt(i int) khipu.KnotFlags {
	if i < 0 || i >= len(p.Flags) {
		return 0
	}
	return p.Flags[i]
}
func (p *Paragraph) SetKnotFlags(i int, f khipu.KnotFlags) bool {
	if i < 0 || i >= len(p.Kind) {
		return false
	}
	for len(p.Flags) < len(p.Kind) {
		p.Flags = append(p.Flags, 0)
	}
	p.Flags[i] = f
	return true
}
func (p *Paragraph) Discardable(i int) bool { return p.flagsAt(i)&khipu.KFDiscardable != 0 }
func (p *Paragraph) HasExplicitDiscretionary(i int) bool {
	return p.Material.HasExplicitDiscretionary(i)
}
func (p *Paragraph) DiscretionariesAt(i int) []khipu.DiscretionaryCandidate {
	return p.DiscretionaryCandidates[i]
}
func (p *Paragraph) AddDiscretionaryCandidate(i int, c khipu.DiscretionaryCandidate) bool {
	if p.DiscretionaryCandidates == nil {
		p.DiscretionaryCandidates = make(map[int][]khipu.DiscretionaryCandidate)
	}
	p.DiscretionaryCandidates[i] = append(p.DiscretionaryCandidates[i], c)
	return true
}
func (p *Paragraph) SelectDiscretionary(i int, c khipu.DiscretionarySelection) bool {
	if p.SelectedDiscretionaries == nil {
		p.SelectedDiscretionaries = make(map[int]khipu.DiscretionarySelection)
	}
	p.SelectedDiscretionaries[i] = c
	return true
}
func (p *Paragraph) SelectedDiscretionaryAt(i int) (khipu.DiscretionarySelection, bool) {
	v, ok := p.SelectedDiscretionaries[i]
	return v, ok
}
func (p *Paragraph) Selections() map[int]khipu.DiscretionarySelection {
	return p.SelectedDiscretionaries
}
func (p *Paragraph) Candidates() map[int][]khipu.DiscretionaryCandidate {
	return p.DiscretionaryCandidates
}
func (p *Paragraph) String() string {
	var b strings.Builder
	for i := range p.Kind {
		b.WriteString(p.KnotByIndex(i).String())
	}
	return b.String()
}
func FromBinding(k *khipu.Khipu, d *khipu.Discovery, b *khipu.AtomicBinding) *Paragraph {
	p := &Paragraph{Plans: make(map[*int]*khipu.Tikray), Material: k, Discovery: d, Binding: b, SelectedDiscretionaries: make(map[int]khipu.DiscretionarySelection), DiscretionaryCandidates: make(map[int][]khipu.DiscretionaryCandidate)}
	for i := 0; i < b.KnotCount(); i++ {
		v, _ := b.Knot(i)
		p.W = append(p.W, v.W)
		p.MinW = append(p.MinW, v.MinW)
		p.MaxW = append(p.MaxW, v.MaxW)
		p.Penalty = append(p.Penalty, v.Penalty)
		p.Pos = append(p.Pos, v.Pos)
		p.Len = append(p.Len, v.Len)
		p.Kind = append(p.Kind, v.Kind)
		p.Flags = append(p.Flags, v.Flags)
	}
	return p
}
func Encode(q *khipu.Khipukamayuq, para *styled.Paragraph) (*Paragraph, error) {
	k, d, e := q.EncodeParagraph(para)
	if e != nil {
		return nil, e
	}
	b, e := khipu.BindParagraph(k, nil)
	if e != nil {
		return nil, e
	}
	return FromBinding(k, d, b), nil
}
func Discover(q *khipu.Khipukamayuq, m search.Paragraph, i int) ([]khipu.DiscretionaryCandidate, error) {
	p := m.(*Paragraph)
	return (search.Provider{Discovery: p.Discovery}).DiscretionaryCandidates(m, i)
}
func Request(p *Paragraph, layout khipu.Params) khipu.BreakRequest {
	return khipu.BreakRequest{Binding: p.Binding, Discovery: p.Discovery, Geometry: khipu.GeometryRequest{Shape: layout.LineShape, LeftSkip: layout.LeftSkip, RightSkip: layout.RightSkip}}
}
func Publish(p *Paragraph, path []int, layout khipu.Params) (*khipu.Tikray, error) {
	r := Request(p, layout)
	w := search.NewWidths(layout.LineShape, nil)
	lines, e := w.Decisions(r, path, p.SelectedDiscretionaries, nil)
	if e != nil {
		return nil, e
	}
	return khipu.NewTikray(r, lines, khipu.PolicyRecord{Algorithm: khipu.FirstFitAlgorithm}, khipu.BreakDiagnostics{})
}
func Freeze(q *khipu.Khipukamayuq, p *Paragraph, path []int, layout khipu.Params) (*khipu.FrozenKhipu, error) {
	plan := p.PlanFor(path)
	var e error
	if plan == nil {
		plan, e = Publish(p, path, layout)
	}
	if e != nil {
		return nil, e
	}
	return khipu.Freeze(plan)
}

func (p *Paragraph) Save(plan *khipu.Tikray) []int {
	lines := plan.Lines()
	ends := make([]int, len(lines))
	clear(p.SelectedDiscretionaries)
	for i, l := range lines {
		ends[i] = l.End.Knot
		if l.End.Kind == khipu.DiscretionaryBoundary {
			p.SelectedDiscretionaries[l.End.Knot] = khipu.DiscretionarySelection{Source: l.End.Knot, Variant: l.End.Alternative.Variant}
		}
	}
	p.Plan = plan
	if p.Plans == nil {
		p.Plans = make(map[*int]*khipu.Tikray)
	}
	if len(ends) > 0 {
		p.Plans[&ends[0]] = plan
	}
	return ends
}
func (p *Paragraph) PlanFor(path []int) *khipu.Tikray {
	if len(path) == 0 {
		return nil
	}
	return p.Plans[&path[0]]
}
func (p *Paragraph) BreakpointPositions(path []int) ([]uint64, error) {
	plan := p.PlanFor(path)
	if plan == nil {
		return nil, khipu.ErrIllegalArguments
	}
	return plan.BreakpointPositions()
}
