package khipu

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/uax/bidi"
)

func explicitOwner(material []HyphenationFragment) (*Khipukamayuq, *testhyphenator, *int) {
	o := intrinsicOwner()
	h := &testhyphenator{want: material}
	o.env.hyphenator = h
	shapes := new(int)
	o.env.streaming.NewShaper = func() (contracts.StreamingShaper, error) { *shapes++; return helpers.NewSyntheticStreamingShaper(8) }
	return o, h, shapes
}

func TestExplicitIntrinsicMaterialPolicies(t *testing.T) {
	for _, tc := range []struct {
		name, text       string
		material         []HyphenationFragment
		min, max         dimen.DU
		calls, fragments int
		unsupported      bool
	}{
		{"ordinary", "abcd", []HyphenationFragment{{PreBreak: "-"}}, 20, 20, 0, 0, false},
		{"edges", "\u00adab\u00ad", []HyphenationFragment{{PreBreak: "-"}}, 10, 10, 0, 0, false},
		{"markers only", "\u00ad\u00ad", []HyphenationFragment{{PreBreak: "-"}}, 0, 0, 0, 0, false},
		{"one", "ab\u00adcd", []HyphenationFragment{{PreBreak: "-"}}, 15, 20, 1, 2, false},
		{"unicode", "éβ\u00adжδ", []HyphenationFragment{{PreBreak: "-"}}, 15, 20, 1, 2, false},
		{"silent", "ab\u00adcd", []HyphenationFragment{{}}, 10, 20, 1, 2, false},
		{"declined", "ab\u00adcd", nil, 20, 20, 1, 0, false},
		{"prohibited", "ab\u00adcd", []HyphenationFragment{{PreBreak: "-", Penalty: MaxPenalty}}, 20, 20, 1, 2, false},
		{"one permitting", "ab\u00adcd", []HyphenationFragment{{PreBreak: "----", Penalty: MaxPenalty}, {PreBreak: "-", Penalty: 1}}, 15, 20, 1, 4, false},
		{"two permitting", "ab\u00adcd", []HyphenationFragment{{PreBreak: "-"}, {PreBreak: "-"}}, 0, 0, 1, 0, true},
		{"two positions", "ab\u00adcd\u00adef", nil, 0, 0, 0, 0, true},
		{"consecutive positions", "ab\u00ad\u00adcd", nil, 0, 0, 0, 0, true},
		{"forced boundaries", "ab\u00adcd\nef", []HyphenationFragment{{PreBreak: "-"}}, 15, 20, 1, 2, false},
		{"trim", " ab\u00adcd ", []HyphenationFragment{{PreBreak: "-"}}, 15, 20, 1, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, h, shapes := explicitOwner(tc.material)
			p := intrinsicPrepare(t, o, tc.text, PrepareOptions{Hyphenation: ExplicitHyphenation})
			before := *shapes
			for n := 0; n < 2; n++ {
				got, err := o.MeasureIntrinsic(t.Context(), p, MeasureRequest{})
				if tc.unsupported {
					if got != (IntrinsicWidths{}) || !errors.Is(err, ErrUnsupportedPolicy) {
						t.Fatal(got, err)
					}
				} else if err != nil || got != (IntrinsicWidths{tc.min * dimen.BP, tc.max * dimen.BP}) {
					t.Fatal(got, err)
				}
				wantCalls := tc.calls
				if tc.unsupported {
					wantCalls *= n + 1
				}
				if h.calls != wantCalls || *shapes != before+tc.fragments {
					t.Fatal("service calls", h.calls, *shapes, before)
				}
			}
		})
	}
}

func TestExplicitIntrinsicDefaultPenaltyAndInvalidMaterial(t *testing.T) {
	o, h, _ := explicitOwner([]HyphenationFragment{{PreBreak: "-"}})
	o.env.params.Hyphenpenalty = int(MaxPenalty)
	p := intrinsicPrepare(t, o, "ab\u00adcd", PrepareOptions{Hyphenation: ExplicitHyphenation})
	if got, err := o.MeasureIntrinsic(t.Context(), p, MeasureRequest{}); err != nil || got != (IntrinsicWidths{20 * dimen.BP, 20 * dimen.BP}) {
		t.Fatal(got, err)
	}
	if h.calls != 1 {
		t.Fatal(h.calls)
	}
	for _, bad := range []string{"\xff", "\u00ad"} {
		o, _, shapes := explicitOwner([]HyphenationFragment{{PreBreak: "-"}, {PreBreak: bad, Penalty: MaxPenalty}})
		p := intrinsicPrepare(t, o, "ab\u00adcd", PrepareOptions{Hyphenation: ExplicitHyphenation})
		before := *shapes
		got, err := o.MeasureIntrinsic(t.Context(), p, MeasureRequest{})
		if got != (IntrinsicWidths{}) || !errors.Is(err, contracts.ErrIllegalArguments) || *shapes != before || len(p.explicit) != 0 {
			t.Fatal(got, err)
		}
	}
}

func TestExplicitIntrinsicNeighborsAndSignedAtoms(t *testing.T) {
	o, h, shapes := explicitOwner([]HyphenationFragment{{PreBreak: "-"}})
	// Neither atomic boundary permits a break. Prefix joins pre; post joins suffix.
	p := intrinsicPrepare(t, o, "\uFFFCab\u00adcd\uFFFC", PrepareOptions{Hyphenation: ExplicitHyphenation, AtomicBoundaries: []AtomicBoundary{{3, ProhibitBoundary}, {9, ProhibitBoundary}}})
	base := *shapes
	knots, glyphs := slices.Clone(p.knots), slices.Clone(p.shaping.glyphs)
	for _, tc := range []struct{ prefix, suffix, min, max dimen.DU }{
		{3, 6, 18, 29}, {-10, 6, 16, 16}, {3, -20, 18, 18},
	} {
		got, err := o.MeasureIntrinsic(t.Context(), p, MeasureRequest{[]AtomicIntrinsic{{1, tc.prefix * dimen.BP, tc.prefix * dimen.BP}, {2, tc.suffix * dimen.BP, tc.suffix * dimen.BP}}})
		if err != nil || got != (IntrinsicWidths{tc.min * dimen.BP, tc.max * dimen.BP}) {
			t.Fatal(tc, got, err)
		}
	}
	if h.calls != 1 || *shapes != base+2 || !reflect.DeepEqual(knots, p.knots) || !reflect.DeepEqual(glyphs, p.shaping.glyphs) {
		t.Fatal("base mutated or cache not reused")
	}
	// Max-content can be finite while the pre-break prefix overflows the domain.
	got, err := o.MeasureIntrinsic(t.Context(), p, MeasureRequest{[]AtomicIntrinsic{{1, dimen.Fil - 1, dimen.Fil - 1}, {2, -100 * dimen.BP, -100 * dimen.BP}}})
	if got != (IntrinsicWidths{}) || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal(got, err)
	}
}

func TestExplicitIntrinsicAdjacentStyledTextboxes(t *testing.T) {
	o, _, _ := explicitOwner([]HyphenationFragment{{PreBreak: "-"}})
	o.env.streaming.Resolver = preparationResolver(func(_ styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) { return d, nil })
	text := styled.TextFromString("xab\u00adcdef\u00adghy")
	// Prefix x, ab SHY cd, ef SHY gh, suffix y. No ordinary break at style edges.
	for i, bounds := range [][2]uint64{{0, 1}, {1, 7}, {7, 13}, {13, 14}} {
		var err error
		text, err = text.Style(intrinsicStyle(string(rune('a'+i))), bounds[0], bounds[1])
		if err != nil {
			t.Fatal(err)
		}
	}
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := o.PrepareParagraph(t.Context(), ParagraphInput{Paragraph: para}, PrepareOptions{Hyphenation: ExplicitHyphenation})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.knots) != 5 {
		t.Fatal("fixture", p.knots)
	}
	// xab- | cdef- | ghy = 20,25,15; no-break xabcdefghy = 50.
	// A spurious break after the first post fragment would incorrectly give 20.
	got, err := o.MeasureIntrinsic(t.Context(), p, MeasureRequest{})
	if err != nil || got != (IntrinsicWidths{25 * dimen.BP, 50 * dimen.BP}) {
		t.Fatal(got, err)
	}
}

func TestExplicitIntrinsicFailureAndRetry(t *testing.T) {
	sentinel := errors.New("explicit service failed")
	for _, stage := range []string{"provider", "pre", "post", "cancel provider", "cancel pre", "cancel post"} {
		t.Run(stage, func(t *testing.T) {
			o, _, _ := explicitOwner(nil)
			p := intrinsicPrepare(t, o, "ab\u00adcd ef\u00adgh", PrepareOptions{Hyphenation: ExplicitHyphenation})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls, shapes := 0, 0
			fail := true
			o.env.hyphenator = explicitMaterialFunc(func(word string, _ *Params) ([]HyphenationFragment, error) {
				if word != softHyphenMarker {
					t.Error("whole word request", word)
				}
				calls++
				if fail && calls == 2 {
					if stage == "provider" {
						return nil, sentinel
					}
					if stage == "cancel provider" {
						cancel()
					}
				}
				return []HyphenationFragment{{PreBreak: "-"}}, nil
			})
			o.env.streaming.NewShaper = func() (contracts.StreamingShaper, error) {
				shapes++
				if fail && ((shapes == 3 && (stage == "pre" || stage == "cancel pre")) || (shapes == 4 && (stage == "post" || stage == "cancel post"))) {
					if stage == "pre" || stage == "post" {
						return nil, sentinel
					}
					cancel()
				}
				return helpers.NewSyntheticStreamingShaper(8)
			}
			got, err := o.MeasureIntrinsic(ctx, p, MeasureRequest{})
			wantErr := sentinel
			if len(stage) > 6 && stage[:6] == "cancel" {
				wantErr = context.Canceled
			}
			if got != (IntrinsicWidths{}) || !errors.Is(err, wantErr) || len(p.explicit) != 1 {
				t.Fatal(got, err, len(p.explicit))
			}
			saved := p.explicit[0].replacements[0].pair
			fail = false
			got, err = o.MeasureIntrinsic(t.Context(), p, MeasureRequest{})
			if err != nil || got != (IntrinsicWidths{15 * dimen.BP, 45 * dimen.BP}) || calls != 3 || p.explicit[0].replacements[0].pair != saved {
				t.Fatal("retry", got, err, calls)
			}
		})
	}
}

func TestExplicitIntrinsicValidatesAtomicsBeforeServices(t *testing.T) {
	o, h, shapes := explicitOwner([]HyphenationFragment{{PreBreak: "-"}})
	p := intrinsicPrepare(t, o, "ab\u00adcd\uFFFC", PrepareOptions{Hyphenation: ExplicitHyphenation})
	before := *shapes
	got, err := o.MeasureIntrinsic(t.Context(), p, MeasureRequest{[]AtomicIntrinsic{{ID: 2}}})
	if got != (IntrinsicWidths{}) || !errors.Is(err, ErrIllegalArguments) || h.calls != 0 || *shapes != before || len(p.explicit) != 0 {
		t.Fatal(got, err)
	}
}

func TestExplicitIntrinsicFeatureErrorPrecedesVariantPolicy(t *testing.T) {
	o, _, shapes := explicitOwner([]HyphenationFragment{{PreBreak: "-"}, {PreBreak: "="}})
	p := intrinsicPrepare(t, o, "ab\u00adcd", PrepareOptions{Hyphenation: ExplicitHyphenation})
	before := *shapes
	p.shaping.contexts[0].params.shaping.Features = []contracts.ShapeFeature{{Start: 1, End: 2}}
	got, err := o.MeasureIntrinsic(t.Context(), p, MeasureRequest{})
	if got != (IntrinsicWidths{}) || !errors.Is(err, contracts.ErrIllegalArguments) || errors.Is(err, ErrUnsupportedPolicy) || *shapes != before || len(p.explicit) != 0 {
		t.Fatal(got, err)
	}
}
