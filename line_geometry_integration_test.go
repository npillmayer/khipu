package khipu_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

type geometryIntegrationShape struct{ calls []int32 }

func (s *geometryIntegrationShape) LineLength(n int32) dimen.DU {
	s.calls = append(s.calls, n)
	if n == 1 {
		return 15 * dimen.PT
	}
	return 25 * dimen.PT
}

func TestLineGeometryFromBothBreakers(t *testing.T) {
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
				left := khipu.KnotCore{W: 2 * dimen.PT, MinW: 2 * dimen.PT, MaxW: 2 * dimen.PT, Flags: khipu.KFDiscardable}
				right := khipu.KnotCore{W: 3 * dimen.PT, MinW: 3 * dimen.PT, MaxW: 3 * dimen.PT}
				shape := &geometryIntegrationShape{}
				layout := khipu.Params{LineShape: shape, LeftSkip: left, RightSkip: right}
				var breaks []int
				if engine == "firstfit" {
					params := *firstfit.DefaultParameters
					breaks, err = firstfit.BreakParagraph(k, layout, &params)
				} else {
					params := knuthplass.NewKPDefaultParameters()
					breaks, err = knuthplass.BreakParagraph(k, layout, params)
				}
				if err != nil {
					t.Fatal(err)
				}
				shape.calls = nil
				got, err := khipu.MaterializeLineGeometryForTest(owner, k, breaks, layout)
				if err != nil {
					t.Fatal(err)
				}
				if len(got) != 2 || !slices.Equal(shape.calls, []int32{1, 2}) {
					t.Fatal(got, shape.calls)
				}
				visual, err := khipu.MaterializeVisualLinesForTest(owner, k, breaks)
				if err != nil {
					t.Fatal(err)
				}
				for i, line := range got {
					target := dimen.DU(15+10*i) * dimen.PT
					if line.TargetWidth != target || line.NaturalWidth != target || line.Delta != 0 ||
						line.LeftSkip != left || line.RightSkip != right || !reflect.DeepEqual(line.Content, visual[i]) {
						t.Fatal("target/skip contract differs from breaker", i, line)
					}
				}
			})
		}
	}
}
