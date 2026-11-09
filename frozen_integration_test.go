package khipu_test

import (
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

type freezeIntegrationShape struct{ calls []int32 }

func (s *freezeIntegrationShape) LineLength(n int32) dimen.DU {
	s.calls = append(s.calls, n)
	if n == 1 {
		return 15 * dimen.PT
	}
	return 30 * dimen.PT
}

func TestFreezeFromBothBreakers(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
			t.Run(engine+dir.String(), func(t *testing.T) {
				text := styled.TextFromString("aa\nbbbb")
				para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), dir, nil)
				if err != nil {
					t.Fatal(err)
				}
				owner := khipu.NewKhipukamayuq(khipu.Config{})
				k, err := owner.EncodeParagraph(t.Context(), para)
				if err != nil {
					t.Fatal(err)
				}
				left := khipu.KnotCore{W: 2 * dimen.PT, MinW: 2 * dimen.PT, MaxW: 2 * dimen.PT}
				right := khipu.KnotCore{W: 3 * dimen.PT, MinW: 3 * dimen.PT, MaxW: 3 * dimen.PT}
				shape := &freezeIntegrationShape{}
				layout := khipu.Params{LineShape: shape, LeftSkip: left, RightSkip: right}
				var breaks []int
				if engine == "firstfit" {
					params := *firstfit.NewDefaultParameters()
					breaks, err = firstfit.BreakParagraph(k, layout, &params)
				} else {
					params := knuthplass.NewDefaultParameters()
					breaks, err = knuthplass.BreakParagraph(k, layout, params)
				}
				if err != nil {
					t.Fatal(err)
				}
				before := slices.Clone(k.W)
				shape.calls = nil
				got, err := owner.Freeze(t.Context(), k, breaks, layout)
				if err != nil {
					t.Fatal(err)
				}
				if got.LineCount() != 2 || !slices.Equal(shape.calls, []int32{1, 2}) || !slices.Equal(before, k.W) {
					t.Fatal(got, shape.calls)
				}
				for i := 0; i < got.LineCount(); i++ {
					line, _ := got.Line(i)
					sum := line.LeftSkip + line.RightSkip
					for j := 0; j < line.KnotCount; j++ {
						knot, _ := got.Knot(line.FirstKnot + j)
						sum += knot.W
					}
					want := dimen.DU(15+15*i) * dimen.PT
					if sum != want || line.Width != want || line.LeftSkip != left.W || line.RightSkip != right.W {
						t.Fatal("incorrect frozen width", line)
					}
				}
				last, _ := got.Line(1)
				first, _ := got.Line(0)
				fill := last.KnotCount - 1
				if dir == bidi.RightToLeft {
					fill = 0
				}
				fillKnot, _ := got.Knot(last.FirstKnot + fill)
				if fillKnot.Material != khipu.ParagraphFillMaterial || fillKnot.W != 5*dimen.PT || first.Source.End != 3 || last.Source.Start != 3 || last.Source.End != 7 {
					t.Fatal("lost visual fill or logical coverage", got)
				}
			})
		}
	}
}
