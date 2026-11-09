package khipu_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

func TestDefaultLayoutProfiles(t *testing.T) {
	shape := linebreak.RectangularParShape(100 * dimen.PT)
	for _, tc := range []struct {
		name    string
		profile func(khipu.LineShape) khipu.Params
		max     dimen.DU
	}{
		{"firstfit", firstfit.DefaultLayout, dimen.Fil},
		{"knuthplass", knuthplass.DefaultLayout, 50 * dimen.BP},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.profile(shape)
			if p.LineShape != shape || p.LeftSkip.W != 0 || p.LeftSkip.MinW != 0 || p.LeftSkip.MaxW != 0 ||
				p.RightSkip.W != 0 || p.RightSkip.MinW != 0 || p.RightSkip.MaxW != tc.max || p.RightSkip.Kind != khipu.KTGlue ||
				p.SpaceStretchPercent != 50 || p.SpaceShrinkPercent != 33.3 {
				t.Fatal("incomplete profile", p)
			}
			if err := p.ValidateGeometry(); err != nil {
				t.Fatal(err)
			}
			p.RightSkip = khipu.KnotCore{}
			if tc.profile(shape).RightSkip.MaxW != tc.max {
				t.Fatal("profile defaults share mutable storage")
			}
		})
	}
}

func layoutParagraph(t *testing.T, dir bidi.Direction) (*khipu.Khipukamayuq, *khipu.Khipu) {
	t.Helper()
	text := styled.TextFromString("aa bb\ncc")
	p, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	k, err := owner.EncodeParagraph(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	return owner, k
}

func TestFirstfitRaggedRightFreezing(t *testing.T) {
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		for _, target := range []dimen.DU{30 * dimen.PT, 24 * dimen.PT} {
			owner, k := layoutParagraph(t, dir)
			layout := firstfit.DefaultLayout(linebreak.RectangularParShape(target))
			before := slices.Clone(k.W)
			breaks, err := firstfit.BreakParagraph(k, layout, nil)
			if err != nil {
				t.Fatal(err)
			}
			visual, err := khipu.MaterializeVisualLinesForTest(owner, k, breaks)
			if err != nil {
				t.Fatal(err)
			}
			got, err := khipu.FreezeParagraphForTest(owner, k, breaks, layout)
			if err != nil || len(got) != 2 {
				t.Fatal(got, err)
			}
			wantSkip, wantSpace := target-25*dimen.PT, 5*dimen.PT
			if target < 25*dimen.PT {
				wantSkip, wantSpace = 0, target-20*dimen.PT
			}
			if got[0].LeftSkip != 0 || got[0].RightSkip != wantSkip {
				t.Fatal("skip is not on physical right edge", dir, got[0])
			}
			spaces := 0
			for j, core := range visual[0].Knots {
				if core.Kind == khipu.KTGlue && core.W == 5*dimen.PT {
					spaces++
					if got[0].Widths[j] != wantSpace {
						t.Fatal("wrong finite interword adjustment", got[0])
					}
				} else if got[0].Widths[j] != core.W {
					t.Fatal("changed non-space content")
				}
			}
			if spaces != 1 || !slices.Equal(k.W, before) {
				t.Fatal("lost space or mutated input")
			}
			fill := len(got[1].Widths) - 1
			if dir == bidi.RightToLeft {
				fill = 0
			}
			if got[1].RightSkip != 0 || !got[1].ParagraphFill[fill] || got[1].Widths[fill] != target-10*dimen.PT {
				t.Fatal("terminal fill did not dominate fil side skip", got[1])
			}
			for _, line := range got {
				sum := line.LeftSkip + line.RightSkip
				for _, w := range line.Widths {
					sum += w
				}
				if sum != target || line.Width != target {
					t.Fatal("non-conserving final width", line)
				}
			}
		}
	}
}

func TestSharedLayoutExplicitZeroSkips(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		t.Run(engine, func(t *testing.T) {
			owner, k := layoutParagraph(t, bidi.LeftToRight)
			shape := linebreak.RectangularParShape(30 * dimen.PT)
			layout := firstfit.DefaultLayout(shape)
			breakLine := func(p khipu.Params) ([]int, error) { return firstfit.BreakParagraph(k, p, nil) }
			if engine == "knuthplass" {
				layout = knuthplass.DefaultLayout(shape)
				breakLine = func(p khipu.Params) ([]int, error) { return knuthplass.BreakParagraph(k, p, nil) }
			}
			layout.LeftSkip, layout.RightSkip = khipu.KnotCore{}, khipu.KnotCore{}
			breaks, err := breakLine(layout)
			if err != nil {
				t.Fatal(err)
			}
			g, err := khipu.MaterializeLineGeometryForTest(owner, k, breaks, layout)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range g {
				if line.LeftSkip != (khipu.KnotCore{}) || line.RightSkip != (khipu.KnotCore{}) {
					t.Fatal("substituted skip defaults")
				}
			}
			// Finite glue stretches beyond its nominal maximum without restoring
			// default side skips; the allocation is reported as underfull.
			if got, err := khipu.FreezeParagraphForTest(owner, k, breaks, layout); err != nil || got[0].LeftSkip != 0 || got[0].RightSkip != 0 || got[0].Width != 30*dimen.PT || !got[0].Underfull || got[0].Residual != 0 || got[0].Badness != 800 {
				t.Fatal("zero override lost during allocation", got, err)
			}
		})
	}
}

func TestFirstfitFillPreservesGreedyBreakpoints(t *testing.T) {
	text := styled.TextFromString("aa bb cc dd")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	k, err := owner.EncodeParagraph(t.Context(), para)
	if err != nil {
		t.Fatal(err)
	}
	layout := firstfit.DefaultLayout(linebreak.RectangularParShape(30 * dimen.PT))
	breaks, err := firstfit.BreakParagraph(k, layout, nil)
	if err != nil || len(breaks) != 2 {
		t.Fatal(breaks, err)
	}
	got, err := khipu.FreezeParagraphForTest(owner, k, breaks, layout)
	if err != nil || got[0].RightSkip != 5*dimen.PT {
		t.Fatal(got, err)
	}
	layout.RightSkip = khipu.KnotCore{}
	zeroBreaks, err := firstfit.BreakParagraph(k, layout, nil)
	if err != nil || !slices.Equal(breaks, zeroBreaks) {
		t.Fatal("fill changed greedy choices", breaks, zeroBreaks, err)
	}
}

type nilLayoutShape struct{}

func (*nilLayoutShape) LineLength(int32) dimen.DU { panic("invalid layout queried the shape") }

func TestLayoutValidationBeforeBreaking(t *testing.T) {
	for _, layout := range []khipu.Params{
		{},
		{LineShape: (*nilLayoutShape)(nil)},
		{LineShape: &nilLayoutShape{}, RightSkip: khipu.KnotCore{W: 1}},
		{LineShape: &nilLayoutShape{}, LeftSkip: khipu.KnotCore{MaxW: dimen.Infinity}},
	} {
		if err := layout.ValidateGeometry(); !errors.Is(err, khipu.ErrIllegalArguments) {
			t.Fatal(err)
		}
		owner, k := layoutParagraph(t, bidi.LeftToRight)
		k.SelectedDiscretionaries = map[int]khipu.DiscretionarySelection{0: {Source: 0, Variant: 17}}
		want := map[int]khipu.DiscretionarySelection{0: {Source: 0, Variant: 17}}
		if got, err := firstfit.BreakParagraph(k, layout, nil); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got, err := knuthplass.BreakParagraph(k, layout, nil); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if !reflect.DeepEqual(k.SelectedDiscretionaries, want) {
			t.Fatal("invalid geometry changed selections")
		}
		clear(k.SelectedDiscretionaries)
		if got, err := khipu.FreezeParagraphForTest(owner, k, []int{len(k.Kind) - 1}, layout); got != nil || err == nil {
			t.Fatal(got, err)
		}
	}
}
