package firstfit

import (
	"context"
	"github.com/npillmayer/khipu/internal/testsupport"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/uax/bidi"
)

type continuationServices struct {
	sealed atomic.Bool
	calls  [4]atomic.Int64
}

func (s *continuationServices) hit(i int) {
	s.calls[i].Add(1)
	if s.sealed.Load() {
		panic("continuation called a preparation service")
	}
}

type continuationResolver struct{ services *continuationServices }

func (r continuationResolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	r.services.hit(0)
	return helpers.DefaultStyleResolver{}.ResolveStyle(s, d)
}

type continuationStream struct {
	*helpers.SyntheticStreamingShaper
	services *continuationServices
}

func (s continuationStream) CheckFace(f contracts.FontFace) error {
	s.services.hit(2)
	return s.SyntheticStreamingShaper.CheckFace(f)
}
func (s continuationStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	s.services.hit(3)
	return s.SyntheticStreamingShaper.Shape(ctx, p, r, o)
}

func continuationFixture(t *testing.T, source string, dir bidi.Direction, elastic bool) (*khipu.Khipukamayuq, *testsupport.Paragraph, *testsupport.Paragraph) {
	t.Helper()
	services := new(continuationServices)
	params := khipu.DefaultParams()
	if !elastic {
		params.SpaceStretchPercent, params.SpaceShrinkPercent = 0, 0
	}
	q := khipu.NewKhipukamayuq(khipu.Config{Params: &params, Streaming: &khipu.StreamingConfig{
		Resolver: continuationResolver{services},
		Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP},
		NewShaper: func() (contracts.StreamingShaper, error) {
			services.hit(1)
			s, err := helpers.NewSyntheticStreamingShaper(8)
			return continuationStream{s, services}, err
		},
	}})
	text := styled.TextFromString(source)
	paragraph, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	paragraph.Offset = 37
	material, discovery, err := q.PrepareParagraph(khipu.ParagraphInput{Paragraph: paragraph}, khipu.PrepareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if services.calls[1].Load() != 1 {
		t.Fatal("expected one preparation stream")
	}
	services.sealed.Store(true)
	binding, err := khipu.BindParagraph(material, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := testsupport.FromBinding(material, discovery, binding)
	return q, p, continuationBind(t, q, p)
}
func continuationBind(t *testing.T, q *khipu.Khipukamayuq, p *testsupport.Paragraph) *testsupport.Paragraph {
	t.Helper()
	binding, err := khipu.BindParagraph(p.Material, nil)
	if err != nil {
		t.Fatal(err)
	}
	k := testsupport.FromBinding(p.Material, p.Discovery, binding)
	k.Plans = p.Plans
	return k
}

type continuationShape struct {
	prefix []dimen.DU
	tail   dimen.DU
	visits []int32
}

func (s *continuationShape) LineLength(n int32) dimen.DU {
	s.visits = append(s.visits, n)
	if int(n) <= len(s.prefix) {
		return s.prefix[n-1]
	}
	return s.tail
}

// The constrained reference scans all prefix knots, obeying only the certified
// boundaries. It then invokes ordinary first-fit on detached metric slice headers
// for the suffix, translating line numbers and knot indexes. There is no production
// seed path here, and no mutation of the original binding or its provenance.
func continuationReference(t *testing.T, k *testsupport.Paragraph, layout khipu.Params, prefix []int) []int {
	t.Helper()
	start, next := 0, 0
	for i := 0; next < len(prefix); i++ {
		_ = k.KnotByIndex(i)
		if i == prefix[next] {
			start = i + 1
			next++
		}
	}
	out := slices.Clone(prefix)
	if start == len(k.Kind) {
		return out
	}
	suffix := &testsupport.Paragraph{W: k.W[start:], MinW: k.MinW[start:], MaxW: k.MaxW[start:],
		Penalty: k.Penalty[start:], Pos: k.Pos[start:], Len: k.Len[start:], Kind: k.Kind[start:], Flags: k.Flags[start:]}
	layout.LineShape = continuationOffsetShape{layout.LineShape, len(prefix)}
	breaks, err := breakFixture(suffix, layout, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range breaks {
		out = append(out, start+at)
	}
	return out
}

type continuationOffsetShape struct {
	khipu.LineShape
	offset int
}

func (s continuationOffsetShape) LineLength(n int32) dimen.DU {
	return s.LineShape.LineLength(n + int32(s.offset))
}

func checkContinuationFrozen(t *testing.T, q *khipu.Khipukamayuq, k *testsupport.Paragraph, path []int, layout khipu.Params) *khipu.FrozenKhipu {
	t.Helper()
	f, err := testsupport.Freeze(q, k, path, layout)
	if err != nil {
		t.Fatal(err)
	}
	var end uint64
	fills := 0
	for i := 0; i < f.LineCount(); i++ {
		l, _ := f.Line(i)
		if l.Source.Start != end {
			t.Fatal("source gap/overlap", l, end)
		}
		end = l.Source.End
		for j := l.FirstKnot; j < l.FirstKnot+l.KnotCount; j++ {
			knot, _ := f.Knot(j)
			if knot.Material == khipu.ParagraphFillMaterial {
				fills++
				if i != f.LineCount()-1 || knot.Source.Bytes != (khipu.ByteRange{Start: f.SourceLen(), End: f.SourceLen()}) {
					t.Fatal("misplaced final fill", i, knot)
				}
			}
		}
	}
	if end != f.SourceLen() || fills != 1 || f.SourceOffset() != 37 {
		t.Fatal("incomplete source/fill", end, fills)
	}
	return f
}
func checkContinuationPrefix(t *testing.T, before, after *khipu.FrozenKhipu, lines int) {
	t.Helper()
	for i := 0; i < lines; i++ {
		a, _ := before.Line(i)
		b, _ := after.Line(i)
		if !reflect.DeepEqual(a, b) {
			t.Fatal("prefix line changed", a, b)
		}
		for j := a.FirstKnot; j < a.FirstKnot+a.KnotCount; j++ {
			ak, _ := before.Knot(j)
			bk, _ := after.Knot(j)
			at, _ := before.CopyText(j)
			bt, _ := after.CopyText(j)
			if !reflect.DeepEqual(ak, bk) || !reflect.DeepEqual(at, bt) {
				t.Fatal("prefix content changed", j)
			}
		}
	}
}

func breakContinuationFixture(k *testsupport.Paragraph, l khipu.Params) ([]int, error) {
	p, e := BreakParagraph(testsupport.Request(k, l), nil)
	if e != nil {
		return nil, e
	}
	return k.Save(p), nil
}
func continueTrialFixture(k *testsupport.Paragraph, l khipu.Params, c *khipu.BreakCheckpoint) ([]int, error) {
	r := testsupport.Request(k, l)
	r.Prefix = c
	p, e := ContinueParagraph(r, nil)
	if e != nil {
		return nil, e
	}
	return k.Save(p), nil
}
func checkpointTrialFixture(p *testsupport.Paragraph, path []int, n int) (*khipu.BreakCheckpoint, error) {
	return khipu.Checkpoint(p.PlanFor(path), n)
}
func prefixTrialFixture(c *khipu.BreakCheckpoint, k *testsupport.Paragraph) ([]int, error) {
	s, e := c.SeedFor(k.Binding)
	if e != nil {
		return nil, e
	}
	r := make([]int, len(s.Prefix))
	for i, l := range s.Prefix {
		r[i] = l.End.Knot
	}
	return r, nil
}
