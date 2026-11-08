package khipu_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
)

// Preserve the text cases from the AOS suite while checking source spans and
// legal boundaries instead of counts of separate legacy penalty items.
func TestEncodedParagraphTextRegressions(t *testing.T) {
	for _, tc := range []struct {
		source     string
		breakAfter []uint64
	}{
		{"Hello World ", []uint64{6}},
		{"The quick !", []uint64{4}},
		{"The quick brown fox jumps over the lazy dog!", []uint64{4, 10, 16, 20, 26, 31, 35, 40}},
		{"lime-tree", []uint64{5}},
	} {
		t.Run(tc.source, func(t *testing.T) {
			owner := khipu.NewKhipukamayuq(khipu.Config{})
			khp, err := owner.EncodeParagraph(t.Context(), encodingParagraph(t, tc.source))
			if err != nil {
				t.Fatal(err)
			}
			var reconstructed strings.Builder
			var boundaries []uint64
			var end uint64
			for i := 0; i < len(khp.Kind)-1; i++ {
				k := khp.KnotByIndex(i)
				if k.Pos != end || k.Len == 0 || k.Pos+uint64(k.Len) > uint64(len(tc.source)) {
					t.Fatalf("invalid source span at %d: %+v", i, k)
				}
				end = k.Pos + uint64(k.Len)
				text := tc.source[k.Pos:end]
				reconstructed.WriteString(text)
				if k.Kind == khipu.KTGlue {
					if strings.TrimSpace(text) != "" || !k.IsDiscardable() {
						t.Fatalf("invalid space knot: %+v (%q)", k, text)
					}
				} else if k.Kind != khipu.KTTextBox || k.IsDiscardable() {
					t.Fatalf("invalid content knot: %+v", k)
				}
				if k.Penalty < khipu.MaxPenalty {
					boundaries = append(boundaries, end)
				}
			}
			if reconstructed.String() != tc.source {
				t.Fatalf("source changed: %q", reconstructed.String())
			}
			if !slices.Equal(boundaries, tc.breakAfter) {
				t.Fatalf("breakable source offsets=%v, want %v", boundaries, tc.breakAfter)
			}
			if len(khp.SelectedDiscretionaries) != 0 || len(khp.DiscretionaryCandidates) != 0 {
				t.Fatal("literal source hyphens must not create discretionary replacements")
			}
		})
	}
}

// Styles now resolve shaping parameters before knots exist, not arbitrary
// post-shaping MinW/MaxW adjustments through the retired KnotStyler hook.
func TestEncodedParagraphStyleResolution(t *testing.T) {
	para := encodingParagraph(t, "The quick brown fox jumps over the lazy dog")
	if _, err := para.Style(encodingStyle("bold"), 4, 15); err != nil {
		t.Fatal(err)
	}
	var seen []styled.Style
	cfg := encodingStreamingConfig()
	cfg.Resolver = encodingResolver(func(style styled.Style, defaults contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
		seen = append(seen, style)
		defaults.Spacing = contracts.SpacingFixed
		if style == encodingStyle("bold") {
			defaults.EmSize = 14 * dimen.PT
		}
		return defaults, nil
	})
	khp, err := khipu.NewKhipukamayuq(khipu.Config{Streaming: cfg}).EncodeParagraph(t.Context(), para)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(seen, []styled.Style{nil, encodingStyle("bold"), nil}) {
		t.Fatalf("resolved styles: %v", seen)
	}
	var boldBytes uint64
	for i := 0; i < len(khp.Kind)-1; i++ {
		k := khp.KnotByIndex(i)
		end := k.Pos + uint64(k.Len)
		if k.Pos >= 4 && end <= 15 {
			boldBytes += uint64(k.Len)
			if k.W != dimen.DU(k.Len)*7*dimen.PT || k.MinW != k.W || k.MaxW != k.W {
				t.Fatalf("bold metrics lost at %d: %+v", i, k)
			}
		} else if (k.Pos < 15 && end > 4) || k.W != dimen.DU(k.Len)*5*dimen.PT || k.MinW != k.W || k.MaxW != k.W {
			t.Fatalf("incorrect unstyled range at %d: %+v", i, k)
		}
	}
	if boldBytes != 11 {
		t.Fatalf("bold span covers %d bytes, want 11", boldBytes)
	}
}

const kingParagraph = `In olden times when wishing still helped one, there lived a king`
const princessParagraph = `In olden times when wishing still helped one, there lived a king whose daughters were all beautiful; and the youngest was so beautiful that the sun itself, which has seen so much, was astonished whenever it shone in her face. Close by the king's castle lay a great dark forest, and under an old lime-tree in the forest was a well, and when the day was very warm, the king's child went out into the forest and sat down by the side of the cool fountain; and when she was bored she took a golden ball, and threw it up on high and caught it; and this ball was her favorite plaything.`

// These paragraphs previously depended on FixedWidthCursor and AOS marks.
// Both modern algorithms now consume encoded metrics and return knot indices.
func TestModernBreakersParagraphRegressions(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		columns      int
		stretch      float64 // percent of natural U+0020 width; no shrink in this corpus
		wantLines    int     // zero: validate a multi-line result without prescribing optimization policy
	}{
		{"underfull", " ", 10, 0, 1},
		{"exact fit", "The quick.", 10, 0, 1},
		{"wrap", "The quick brown fox.", 10, 0, 2},
		{"king", kingParagraph, 45, 300, 2},
		{"princess", princessParagraph, 45, 200, 0},
	} {
		for _, algorithm := range []string{"firstfit", "Knuth-Plass"} {
			t.Run(tc.name+"/"+algorithm, func(t *testing.T) {
				params := &khipu.Params{SpaceStretchPercent: tc.stretch}
				khp, err := khipu.NewKhipukamayuq(khipu.Config{Params: params}).EncodeParagraph(t.Context(), encodingParagraph(t, tc.source))
				if err != nil {
					t.Fatal(err)
				}
				// In particular, verify later cord chunks: an incorrect bounded
				// read once turned their spaces into non-discardable textboxes.
				for i := 0; i < len(khp.Kind)-1; i++ {
					k := khp.KnotByIndex(i)
					text := tc.source[k.Pos : k.Pos+uint64(k.Len)]
					if text == " " && (k.Kind != khipu.KTGlue || !k.IsDiscardable() || k.MinW != k.W || k.MaxW != k.W*dimen.DU(1+tc.stretch/100)) {
						t.Fatalf("space classification/elasticity at byte %d: %+v", k.Pos, k)
					}
				}
				width := dimen.DU(tc.columns) * 5 * dimen.PT
				shape := linebreak.RectangularParShape(width)
				var breaks []int
				if algorithm == "firstfit" {
					breaks, err = firstfit.BreakParagraph(khp, firstfit.DefaultLayout(shape), nil)
				} else {
					params := knuthplass.NewKPDefaultParameters()
					layout := knuthplass.DefaultLayout(shape)
					layout.RightSkip = khipu.KnotCore{MaxW: 5 * dimen.PT}
					breaks, err = knuthplass.BreakParagraph(khp, layout, params)
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(breaks) == 0 || breaks[len(breaks)-1] != len(khp.Kind)-1 {
					t.Fatalf("incomplete paragraph: %v", breaks)
				}
				if (tc.wantLines > 0 && len(breaks) != tc.wantLines) || (tc.wantLines == 0 && len(breaks) < 2) {
					t.Fatalf("unexpected line count %d: %v", len(breaks), breaks)
				}
				start := 0
				var reconstructed strings.Builder
				for _, end := range breaks {
					if end < start || end >= len(khp.Kind) || khp.Penalty[end] >= khipu.MaxPenalty {
						t.Fatalf("illegal breakpoint %d: %v", end, breaks)
					}
					left, right := start, end
					// The retained zero-width trailer does not stop whitespace trimming.
					if right == len(khp.Kind)-1 {
						right--
					}
					for left <= right && khp.KnotByIndex(left).IsDiscardable() {
						left++
					}
					for right >= left && khp.KnotByIndex(right).IsDiscardable() {
						right--
					}
					var minimum dimen.DU
					for i := left; i <= right; i++ {
						minimum += khp.MinW[i]
					}
					if minimum > width {
						t.Fatalf("line %d..%d exceeds target: %v > %v", start, end, minimum, width)
					}
					for i := start; i <= end; i++ {
						reconstructed.WriteString(tc.source[khp.Pos[i] : khp.Pos[i]+uint64(khp.Len[i])])
					}
					start = end + 1
				}
				if reconstructed.String() != tc.source {
					t.Fatal("breaking lost or duplicated source text")
				}
				if len(khp.SelectedDiscretionaries) != 0 {
					t.Fatal("unexpected discretionary selection")
				}
			})
		}
	}
}
