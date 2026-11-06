package khipu_test

import (
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

func TestStreamingLinebreakContractAtNarrowWidths(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		width      dimen.DU
	}{
		{"exact pairs", "aa aa aa aa", 25 * dimen.PT},
		{"one word per line", "aa aa aa aa", 15 * dimen.PT},
		{"multiple spaces", "aa   aa   aa", 25 * dimen.PT},
		{"bidi", "ab אב cd גד", 25 * dimen.PT},
		{"forced", "aa aa\nbb bb", 25 * dimen.PT},
		{"NBSP", "a\u00a0b cd ef", 25 * dimen.PT},
		{"fixed space", "aa\u2003aa bb", 25 * dimen.PT},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := styled.TextFromString(tc.text)
			para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
			if err != nil {
				t.Fatal(err)
			}
			owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: &khipu.StreamingConfig{
				Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT, Spacing: contracts.SpacingElastic},
				NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(1) },
			}})
			k, err := owner.EncodeParagraph(t.Context(), para)
			if err != nil {
				t.Fatal(err)
			}
			before := make([]khipu.KnotCore, len(k.Kind))
			for i := range before {
				before[i] = k.KnotByIndex(i)
			}
			shape := linebreak.RectangularParShape(tc.width)
			f, err := firstfit.BreakParagraph(k, shape, nil)
			if err != nil {
				t.Fatal(err)
			}
			kp, err := knuthplass.BreakParagraph(k, shape, nil)
			if err != nil {
				t.Fatal(err)
			}
			for name, breaks := range map[string][]int{"firstfit": f, "knuthplass": kp} {
				if len(breaks) < 2 || breaks[len(breaks)-1] != len(k.Kind)-1 {
					t.Fatalf("%s: missing wrapped path: %v", name, breaks)
				}
				positions, err := k.BreakpointPositions(breaks)
				if err != nil || len(positions) != len(breaks) || positions[len(positions)-1] != uint64(len(tc.text)) {
					t.Fatalf("%s: invalid source positions %v: %v", name, positions, err)
				}
				for i, at := range breaks {
					// These ordinary fixtures have no omitted glyphs or marker
					// suffixes: source coverage includes the trailing whitespace.
					if positions[i] != k.Pos[at]-para.Offset+uint64(k.Len[at]) {
						t.Fatalf("%s: wrong source endpoint at knot %d: %d", name, at, positions[i])
					}
				}
				prev := -1
				for _, end := range breaks {
					if end <= prev || end >= len(k.Kind) || k.Penalty[end] >= 10000 {
						t.Fatalf("%s: invalid breakpoint %d", name, end)
					}
					for i := prev + 1; i < end; i++ {
						if k.Penalty[i] <= -10000 {
							t.Fatalf("%s skipped forced break %d", name, i)
						}
					}
					// The known fixtures fit without overfull fallback. Trim by flags,
					// not by Unicode whitespace or by the glyph storage representation.
					left, right := prev+1, end
					for left <= right && k.Flags[left]&khipu.KFDiscardable != 0 {
						left++
					}
					for right >= left && k.Flags[right]&khipu.KFDiscardable != 0 {
						right--
					}
					var minimum int64
					for i := left; i <= right; i++ {
						minimum += int64(k.MinW[i])
					}
					if minimum > int64(tc.width) {
						t.Fatalf("%s: line cannot fit: min=%d target=%d path=%v", name, minimum, tc.width, breaks)
					}
					prev = end
				}
			}
			if len(k.SelectedDiscretionaries) != 0 {
				t.Fatal("unexpected discretionary choice")
			}
			after := make([]khipu.KnotCore, len(k.Kind))
			for i := range after {
				after[i] = k.KnotByIndex(i)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("linebreaking mutated base knots")
			}
		})
	}
}
