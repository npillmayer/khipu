package khipu_test

import (
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/uax/bidi"
)

// Exercise ordinary checkpoint selection and carry, then freeze the very same
// lines. Synthetic shaping gives each letter and space a natural width of 5pt.
func TestFirstfitFittingAndFreezing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fixed  bool
		target dimen.DU // zero selects the exact two-word minimum; -1 goes one DU below it
		pairs  bool
	}{
		{"slack", false, 30 * dimen.PT, true},
		{"natural exact", false, 25 * dimen.PT, true},
		{"shrink", false, 24 * dimen.PT, true},
		{"minimum exact", false, 0, true},
		{"below minimum", false, -1, false},
		{"fixed slack", true, 30 * dimen.PT, true},
		{"fixed exact", true, 25 * dimen.PT, true},
		{"fixed cannot compress", true, 24 * dimen.PT, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := khipu.DefaultParams()
			if tc.fixed {
				params.SpaceStretchPercent, params.SpaceShrinkPercent = 0, 0
			}
			owner := khipu.NewKhipukamayuq(khipu.Config{Params: &params})
			text := styled.TextFromString("aa bb cc dd")
			para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
			if err != nil {
				t.Fatal(err)
			}
			k, err := owner.EncodeParagraph(t.Context(), para)
			if err != nil {
				t.Fatal(err)
			}
			target := tc.target
			if target <= 0 {
				target += 20*dimen.PT + k.MinW[1]
			}
			layout := firstfit.DefaultLayout(linebreak.RectangularParShape(target))
			w, min, max := slices.Clone(k.W), slices.Clone(k.MinW), slices.Clone(k.MaxW)
			breaks, err := firstfit.BreakParagraph(k, layout, nil)
			want := []int{1, 3, 5, 7}
			if tc.pairs {
				want = []int{3, 7}
			}
			if err != nil || !slices.Equal(breaks, want) {
				t.Fatalf("breaks=%v, err=%v; want %v", breaks, err, want)
			}
			visual, err := khipu.MaterializeVisualLinesForTest(owner, k, breaks)
			if err != nil {
				t.Fatal(err)
			}
			frozen, err := khipu.FreezeParagraphForTest(owner, k, breaks, layout)
			if err != nil {
				t.Fatal(err)
			}
			if len(frozen) != len(want) {
				t.Fatal("wrong line count", len(frozen))
			}
			for i, line := range frozen {
				sum := line.LeftSkip + line.RightSkip
				spaces := 0
				for j, core := range visual[i].Knots {
					sum += line.Widths[j]
					if line.ParagraphFill[j] {
						continue
					}
					wantWidth := core.W
					if core.Kind == khipu.KTGlue {
						spaces++
						if target < 25*dimen.PT {
							wantWidth = target - 20*dimen.PT
						}
					}
					if line.Widths[j] != wantWidth {
						t.Fatalf("line %d knot %d: width=%v want=%v", i, j, line.Widths[j], wantWidth)
					}
				}
				wantSpaces, natural := 0, 10*dimen.PT
				if tc.pairs {
					wantSpaces, natural = 1, 25*dimen.PT
				}
				wantSkip := target - natural
				if wantSkip < 0 || i == len(frozen)-1 {
					wantSkip = 0
				}
				if spaces != wantSpaces || line.RightSkip != wantSkip || sum != target || line.Width != target {
					t.Fatalf("line %d: spaces=%d skip=%v sum=%v target=%v", i, spaces, line.RightSkip, sum, target)
				}
			}
			if !slices.Equal(k.W, w) || !slices.Equal(k.MinW, min) || !slices.Equal(k.MaxW, max) {
				t.Fatal("breaking/freezing mutated width planes")
			}
		})
	}
}
