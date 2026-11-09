package khipu_test

import (
	"io"
	"maps"
	"reflect"
	"slices"
	"testing"
	"unicode/utf8"

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

func pipelineBreak(t *testing.T, engine string, owner *khipu.Khipukamayuq, k *khipu.Khipu, layout khipu.Params, discover bool) []int {
	t.Helper()
	var ends []int
	var err error
	if engine == "firstfit" {
		p := firstfit.NewDefaultParameters()
		if discover {
			p.DiscretionaryProvider = owner
		}
		ends, err = firstfit.BreakParagraph(k, layout, p)
	} else {
		p := knuthplass.NewDefaultParameters()
		if discover {
			p.DiscretionaryProvider = owner
		}
		ends, err = knuthplass.BreakParagraph(k, layout, p)
	}
	if err != nil {
		t.Fatal(err)
	}
	return ends
}

// Check the contract using only public queries. Exact source endpoints come
// from the chosen breaks, not the visual order of individual source spans.
func checkPipelineResult(t *testing.T, f *khipu.FrozenKhipu, source string, ends []uint64) {
	t.Helper()
	if f.LineCount() != len(ends) || f.SourceLen() != uint64(len(source)) {
		t.Fatal("wrong coverage")
	}
	var start uint64
	next, fills := 0, 0
	for i, end := range ends {
		line, ok := f.Line(i)
		if !ok || line.FirstKnot != next || line.Source != (khipu.ByteRange{Start: start, End: end}) {
			t.Fatal("wrong line range", line)
		}
		sum := line.LeftSkip + line.RightSkip
		for j := 0; j < line.KnotCount; j++ {
			at := next + j
			knot, ok := f.Knot(at)
			if !ok {
				t.Fatal(at)
			}
			sum += knot.W
			if knot.Source.Bytes.Start > knot.Source.Bytes.End || knot.Source.Bytes.End > uint64(len(source)) {
				t.Fatal("invalid provenance", knot)
			}
			if knot.Material == khipu.ParagraphFillMaterial {
				fills++
				if i != len(ends)-1 {
					t.Fatal("early fill")
				}
			}
			if text, ok := f.CopyText(at); ok {
				for _, c := range text.Clusters {
					if c.FirstGlyph < 0 || c.GlyphCount < 0 || c.FirstGlyph+c.GlyphCount > len(text.Glyphs) {
						t.Fatal("invalid glyph range")
					}
				}
				// Synthetic cells are fixed text. Width allocation must never
				// silently disagree with the retained glyph advances.
				var advance float64
				for _, g := range text.Glyphs {
					advance += text.Context.Scale.ToDU(g.XAdvance)
				}
				if delta := advance - float64(knot.W); delta < -1 || delta > 1 {
					t.Fatal("width/glyph mismatch", at, advance, knot.W)
				}
			}
		}
		if sum != line.Width || line.Residual != line.TargetWidth-line.Width {
			t.Fatal("nonconserving line", line)
		}
		next += line.KnotCount
		start = end
	}
	if fills != 1 || next != f.KnotCount() || start != uint64(len(source)) {
		t.Fatal("incomplete traversal")
	}
	r, err := f.SourceReader(khipu.ByteRange{End: uint64(len(source))})
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil || string(b) != source {
		t.Fatal("source changed", err)
	}
}

func TestPublicPipelineGeometry(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		for _, tc := range []struct {
			name, source string
			target       dimen.DU
			fixed        bool
			right        dimen.DU
			fit          khipu.FrozenFit
			lines        int
		}{
			{"empty", "", 30 * dimen.PT, false, dimen.Fil, khipu.FitWithinCapacity, 1},
			{"fixed", "aa bb cc", 24 * dimen.PT, true, dimen.Fil, khipu.FitWithinCapacity, 3},
			{"finite overstretch", "aa bb\ncc", 30 * dimen.PT, false, 0, khipu.FitUnderfull, 2},
			{"finite shrink", "aa bb\ncc", 24 * dimen.PT, false, 0, khipu.FitWithinCapacity, 2},
			{"overfull word", "abcdefgh", 15 * dimen.PT, true, dimen.Fil, khipu.FitOverfull, 1},
			{"fill wins", "aa", 30 * dimen.PT, false, dimen.Fil, khipu.FitWithinCapacity, 1},
			{"filll wins", "aa", 30 * dimen.PT, false, dimen.Filll, khipu.FitWithinCapacity, 1},
		} {
			t.Run(engine+"/"+tc.name, func(t *testing.T) {
				params := khipu.DefaultParams()
				if tc.fixed {
					params.SpaceStretchPercent = 0
					params.SpaceShrinkPercent = 0
				}
				owner := khipu.NewKhipukamayuq(khipu.Config{Params: &params})
				text := styled.TextFromString(tc.source)
				para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
				if err != nil {
					t.Fatal(err)
				}
				k, err := owner.EncodeParagraph(t.Context(), para)
				if err != nil {
					t.Fatal(err)
				}
				layout := khipu.Params{LineShape: linebreak.RectangularParShape(tc.target), RightSkip: khipu.KnotCore{MaxW: tc.right}}
				breaks := pipelineBreak(t, engine, owner, k, layout, false)
				ends, err := k.BreakpointPositions(breaks)
				if err != nil {
					t.Fatal(err)
				}
				before := slices.Clone(k.W)
				f, err := owner.Freeze(t.Context(), k, breaks, layout)
				if err != nil {
					t.Fatal(err)
				}
				checkPipelineResult(t, f, tc.source, ends)
				line, _ := f.Line(0)
				if f.LineCount() != tc.lines || line.Fit != tc.fit || !slices.Equal(before, k.W) {
					t.Fatal("unexpected fitting", line, f.LineCount())
				}
				if tc.name == "fill wins" && (line.RightSkip != 0 || line.FillOrder != dimen.Fill) {
					t.Fatal(line)
				}
				if tc.name == "finite shrink" && line.Ratio >= 0 {
					t.Fatal("fixture did not shrink", line)
				}
				if tc.name == "filll wins" && (line.RightSkip != 20*dimen.PT || line.FillOrder != dimen.Filll) {
					t.Fatal(line)
				}
			})
		}
	}
}

func TestPublicPipelineMixedBidi(t *testing.T) {
	const source = "aa אב גד bb"
	for _, engine := range []string{"firstfit", "knuthplass"} {
		for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
			t.Run(engine+dir.String(), func(t *testing.T) {
				text := styled.TextFromString(source)
				para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), dir, nil)
				if err != nil {
					t.Fatal(err)
				}
				owner := khipu.NewKhipukamayuq(khipu.Config{})
				k, err := owner.EncodeParagraph(t.Context(), para)
				if err != nil {
					t.Fatal(err)
				}
				layout := firstfit.DefaultLayout(linebreak.RectangularParShape(100 * dimen.PT))
				breaks := pipelineBreak(t, engine, owner, k, layout, false)
				ends, err := k.BreakpointPositions(breaks)
				if err != nil {
					t.Fatal(err)
				}
				f, err := owner.Freeze(t.Context(), k, breaks, layout)
				if err != nil {
					t.Fatal(err)
				}
				checkPipelineResult(t, f, source, ends)
				var starts []uint64
				for i := 0; i < f.KnotCount(); i++ {
					knot, _ := f.Knot(i)
					if txt, ok := f.CopyText(i); ok {
						starts = append(starts, knot.Source.Bytes.Start)
						r, _ := utf8.DecodeRuneInString(source[knot.Source.Bytes.Start:knot.Source.Bytes.End])
						if len(txt.Glyphs) == 0 || txt.Glyphs[0].GlyphID != uint32(r)+1 {
							t.Fatal("glyph array reversed", txt)
						}
					}
				}
				want := []uint64{0, 8, 3, 13}
				fillAt := f.KnotCount() - 1
				if dir == bidi.RightToLeft {
					want = []uint64{13, 8, 3, 0}
					fillAt = 0
				}
				fill, _ := f.Knot(fillAt)
				if !slices.Equal(starts, want) || fill.Material != khipu.ParagraphFillMaterial {
					t.Fatal("incorrect visual order", starts, fill)
				}
			})
		}
	}
}

func TestPublicPipelineDiscretionaries(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		for _, source := range []string{"a abcd", "a ab\u00adcd"} {
			t.Run(engine+source, func(t *testing.T) {
				policy := &streamingWordPolicy{}
				streams := 0
				owner := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: policy, Streaming: &khipu.StreamingConfig{
					Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT, Spacing: contracts.SpacingElastic},
					NewShaper: func() (contracts.StreamingShaper, error) { streams++; return helpers.NewSyntheticStreamingShaper(0) },
				}})
				text := styled.TextFromString(source)
				para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
				if err != nil {
					t.Fatal(err)
				}
				k, err := owner.EncodeParagraph(t.Context(), para)
				if err != nil {
					t.Fatal(err)
				}
				layout := khipu.Params{LineShape: linebreak.RectangularParShape(25 * dimen.PT), RightSkip: khipu.KnotCore{MaxW: dimen.PT}}
				breaks := pipelineBreak(t, engine, owner, k, layout, true)
				if len(k.SelectedDiscretionaries) != 1 {
					t.Fatal("fixture did not select a variant")
				}
				ends, err := k.BreakpointPositions(breaks)
				if err != nil {
					t.Fatal(err)
				}
				chosen := maps.Clone(k.SelectedDiscretionaries)
				calls, shaped := len(policy.requests), streams
				f, err := owner.Freeze(t.Context(), k, breaks, layout)
				if err != nil {
					t.Fatal(err)
				}
				checkPipelineResult(t, f, source, ends)
				if calls != len(policy.requests) || shaped != streams || !maps.Equal(chosen, k.SelectedDiscretionaries) {
					t.Fatal("freeze reshaped or changed decisions")
				}
				materials := map[khipu.FrozenMaterial]string{}
				for i := 0; i < f.KnotCount(); i++ {
					knot, _ := f.Knot(i)
					if txt, ok := f.CopyText(i); ok && txt.IsReplacement {
						materials[knot.Material] = txt.ReplacementText
					}
				}
				if !reflect.DeepEqual(materials, map[khipu.FrozenMaterial]string{khipu.PreBreakMaterial: "ab-", khipu.PostBreakMaterial: "cd"}) {
					t.Fatal(materials)
				}
				// A later layout clears applied choices but must not invalidate an
				// already frozen result or trigger repeated discovery on warm input.
				pipelineBreak(t, engine, owner, k, layout, true)
				if len(policy.requests) != calls || streams != shaped {
					t.Fatal("warm discovery repeated")
				}
				wide := layout
				wide.LineShape = linebreak.RectangularParShape(100 * dimen.PT)
				wide.RightSkip = khipu.KnotCore{MaxW: dimen.Fil}
				pipelineBreak(t, engine, owner, k, wide, true)
				if len(k.SelectedDiscretionaries) != 0 {
					t.Fatal("stale choices")
				}
				checkPipelineResult(t, f, source, ends)
			})
		}
	}
}
