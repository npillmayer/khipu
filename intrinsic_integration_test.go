package khipu_test

import (
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

func TestIntrinsicAcrossBothBreakersAndFreeze(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		t.Run(engine, func(t *testing.T) {
			shapes := 0
			owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: &khipu.StreamingConfig{
				Resolver: helpers.DefaultStyleResolver{},
				Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP, Spacing: contracts.SpacingElastic},
				NewShaper: func() (contracts.StreamingShaper, error) {
					shapes++
					return helpers.NewSyntheticStreamingShaper(8)
				},
			}})
			text := styled.TextFromString(" ab \uFFFC cd ")
			para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
			if err != nil {
				t.Fatal(err)
			}
			p, err := owner.PrepareParagraph(t.Context(), khipu.ParagraphInput{
				Paragraph: para, Atomics: []contracts.AtomicAnchor{{ID: 1, ByteOffset: 4}},
			}, khipu.PrepareOptions{})
			if err != nil {
				t.Fatal(err)
			}
			preparedShapes := shapes
			request := khipu.MeasureRequest{Atomics: []khipu.AtomicIntrinsic{{ID: 1, MinAdvance: 10 * dimen.BP, MaxAdvance: 20 * dimen.BP}}}
			want := khipu.IntrinsicWidths{MinContent: 10 * dimen.BP, MaxContent: 50 * dimen.BP}
			measure := func() {
				t.Helper()
				if got, err := owner.MeasureIntrinsic(t.Context(), p, request); err != nil || got != want {
					t.Fatalf("measure = %v, %v; want %v", got, err, want)
				}
				if shapes != preparedShapes {
					t.Fatal("measurement or layout reshaped prepared text")
				}
			}
			measure()
			var retained *khipu.FrozenKhipu
			var lines []khipu.FrozenLine
			var knots []khipu.FrozenKnot
			var texts []khipu.FrozenText
			for _, width := range []dimen.DU{50 * dimen.BP, 20 * dimen.BP} {
				k, err := owner.BindParagraph(t.Context(), p, []khipu.AtomicAdvance{{ID: 1, Advance: 20 * dimen.BP}})
				if err != nil {
					t.Fatal(err)
				}
				params := khipu.Params{LineShape: atomicShape(width)}
				var breaks []int
				if engine == "firstfit" {
					breaks, err = firstfit.BreakParagraph(k, params, nil)
				} else {
					breaks, err = knuthplass.BreakParagraph(k, params, nil)
				}
				if err != nil {
					t.Fatal(err)
				}
				f, err := owner.Freeze(t.Context(), k, breaks, params)
				if err != nil {
					t.Fatal(err)
				}
				if retained == nil {
					if f.LineCount() != 1 {
						t.Fatal("max-content width should fit one line")
					}
					line, _ := f.Line(0)
					var natural dimen.DU
					for i := 0; i < f.KnotCount(); i++ {
						knot, _ := f.Knot(i)
						if knot.Material != khipu.ParagraphFillMaterial {
							natural += knot.W
						}
						knots = append(knots, knot)
						text, _ := f.CopyText(i)
						texts = append(texts, text)
					}
					if natural != want.MaxContent {
						t.Fatalf("frozen natural width %d != max-content %d", natural, want.MaxContent)
					}
					lines, retained = append(lines, line), f
				} else if f.LineCount() <= 1 {
					t.Fatal("narrow layout should wrap")
				}
				measure()
				for i := range k.W {
					k.W[i] = 0
					k.Kind[i] = khipu.KTGlue
				}
				measure()
			}
			request.Atomics[0].MinAdvance, request.Atomics[0].MaxAdvance = 0, 0
			want = khipu.IntrinsicWidths{MinContent: 10 * dimen.BP, MaxContent: 30 * dimen.BP}
			measure() // No retained caller range or binding width may leak into a query.
			for i, before := range lines {
				after, _ := retained.Line(i)
				if !reflect.DeepEqual(before, after) {
					t.Fatal("frozen line changed")
				}
			}
			for i, before := range knots {
				after, _ := retained.Knot(i)
				text, _ := retained.CopyText(i)
				if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(texts[i], text) {
					t.Fatal("frozen geometry or glyphs changed")
				}
			}
		})
	}
}

// Snapshot public output, not arena IDs or service pointers. The caller keeps
// the same immutable font services for cold/warm preparations.
type explicitFrozenSnapshot struct {
	breaks   []int
	selected map[int]khipu.DiscretionarySelection
	lines    []khipu.FrozenLine
	knots    []khipu.FrozenKnot
	texts    []khipu.FrozenText
}

type intrinsicExplicitPolicy struct{ calls int }

func (h *intrinsicExplicitPolicy) Discretionaries(word string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	h.calls++
	if word != "\u00ad" {
		return nil, nil
	}
	return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
}

func TestExplicitIntrinsicDoesNotChangeBreakerDiscovery(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		t.Run(engine, func(t *testing.T) {
			shapes := 0
			policy := &intrinsicExplicitPolicy{}
			owner := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: policy, Streaming: &khipu.StreamingConfig{
				Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.BP, Spacing: contracts.SpacingElastic},
				NewShaper: func() (contracts.StreamingShaper, error) { shapes++; return helpers.NewSyntheticStreamingShaper(8) },
			}})
			snapshot := func(k *khipu.Khipu, breaks []int, f *khipu.FrozenKhipu) explicitFrozenSnapshot {
				s := explicitFrozenSnapshot{breaks: breaks, selected: k.SelectedDiscretionaries}
				for i := 0; i < f.LineCount(); i++ {
					x, _ := f.Line(i)
					s.lines = append(s.lines, x)
				}
				for i := 0; i < f.KnotCount(); i++ {
					x, _ := f.Knot(i)
					tx, _ := f.CopyText(i)
					s.knots = append(s.knots, x)
					s.texts = append(s.texts, tx)
				}
				return s
			}
			var expected explicitFrozenSnapshot
			for _, warm := range []bool{false, true} {
				text := styled.TextFromString("ab\u00adcd")
				para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
				if err != nil {
					t.Fatal(err)
				}
				p, err := owner.PrepareParagraph(t.Context(), khipu.ParagraphInput{Paragraph: para}, khipu.PrepareOptions{Hyphenation: khipu.ExplicitHyphenation})
				if err != nil {
					t.Fatal(err)
				}
				baseShapes, baseCalls := shapes, policy.calls
				measure := func() {
					t.Helper()
					got, err := owner.MeasureIntrinsic(t.Context(), p, khipu.MeasureRequest{})
					if err != nil || got != (khipu.IntrinsicWidths{MinContent: 15 * dimen.BP, MaxContent: 20 * dimen.BP}) {
						t.Fatal(got, err)
					}
				}
				if warm {
					measure()
				}
				for range 2 {
					k, err := owner.BindParagraph(t.Context(), p, nil)
					if err != nil {
						t.Fatal(err)
					}
					if len(k.DiscretionaryCandidates) != 0 {
						t.Fatal("candidate visibility changed during bind")
					}
					geometry := firstfit.DefaultLayout(atomicShape(15 * dimen.BP))
					var breaks []int
					if engine == "firstfit" {
						bp := firstfit.NewDefaultParameters()
						bp.DiscretionaryProvider = owner
						breaks, err = firstfit.BreakParagraph(k, geometry, bp)
					} else {
						geometry = knuthplass.DefaultLayout(atomicShape(15 * dimen.BP))
						bp := knuthplass.NewDefaultParameters()
						bp.DiscretionaryProvider = owner
						breaks, err = knuthplass.BreakParagraph(k, geometry, bp)
					}
					if err != nil {
						t.Fatal(err)
					}
					f, err := owner.Freeze(t.Context(), k, breaks, geometry)
					if err != nil {
						t.Fatal(err)
					}
					got := snapshot(k, breaks, f)
					if len(got.selected) != 1 || len(got.lines) != 2 {
						t.Fatal("fixture did not use split", got)
					}
					if expected.breaks == nil {
						expected = got
					} else if !reflect.DeepEqual(expected, got) {
						t.Fatal("measurement changed layout", expected, got)
					}
					measure()
					if !reflect.DeepEqual(got, snapshot(k, breaks, f)) {
						t.Fatal("retained output changed")
					}
					if shapes != baseShapes+2 || policy.calls != baseCalls+1 {
						t.Fatal("repeated services", shapes, baseShapes, policy.calls, baseCalls)
					}
				}
			}
		})
	}
}
