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

type explicitBreakPolicy struct{ calls int }

func (p *explicitBreakPolicy) Discretionaries(text string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	p.calls++
	if text != "\u00ad" {
		panic("automatic discovery requested")
	}
	return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
}

func TestStreamingExplicitBreaksEndToEnd(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		for _, tc := range []struct {
			text  string
			width dimen.DU
			split uint64
		}{
			{"ab\u00adcd", 15 * dimen.PT, 4},
			{"a ab\u00adcd", 25 * dimen.PT, 6},
			{"ab\u00adcd\u00adef", 25 * dimen.PT, 8},
			{"abcd\u00adef", 15 * dimen.PT, 0}, // no pre-break fragment fits
		} {
			t.Run(engine+tc.text, func(t *testing.T) {
				policy := &explicitBreakPolicy{}
				owner := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: policy, Streaming: &khipu.StreamingConfig{
					Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT, Spacing: contracts.SpacingElastic},
					NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(0) },
				}})
				text := styled.TextFromString(tc.text)
				para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
				if err != nil {
					t.Fatal(err)
				}
				k, err := owner.EncodeParagraph(t.Context(), para)
				if err != nil {
					t.Fatal(err)
				}
				before := make([]khipu.KnotCore, len(k.Kind))
				for i := range before {
					before[i] = k.KnotByIndex(i)
				}
				breakAt := func(width dimen.DU) ([]int, error) {
					shape := linebreak.RectangularParShape(width)
					if engine == "firstfit" {
						p := *firstfit.DefaultParameters
						p.DiscretionaryProvider = owner
						return firstfit.BreakParagraph(k, firstfit.DefaultLayout(shape), &p)
					}
					p := knuthplass.NewKPDefaultParameters()
					layout := knuthplass.DefaultLayout(shape)
					p.DiscretionaryProvider = owner
					// Keep a small finite stretch so intermediate short segments
					// remain geometrically evaluable, but exceed pass-1 tolerance.
					if width == tc.width {
						layout.RightSkip = khipu.KnotCore{MaxW: dimen.PT}
						p.EmergencyStretch = 0
					}
					return knuthplass.BreakParagraph(k, layout, p)
				}
				wide, err := breakAt(100 * dimen.PT)
				if err != nil || len(wide) != 1 || policy.calls != 0 {
					t.Fatal("eager discovery", wide, err, policy.calls)
				}
				breaks, err := breakAt(tc.width)
				if tc.split == 0 {
					if err != nil || len(k.SelectedDiscretionaries) != 0 || policy.calls != 1 || len(k.DiscretionariesAt(0)) != 1 {
						t.Fatal("non-fitting candidate was not safely declined", breaks, err, k.SelectedDiscretionaries, policy.calls)
					}
					return
				}
				if err != nil || len(breaks) != 2 || len(k.SelectedDiscretionaries) != 1 {
					t.Fatalf("breaks=%v selections=%v err=%v", breaks, k.SelectedDiscretionaries, err)
				}
				choice, ok := k.SelectedDiscretionaryAt(breaks[0])
				if !ok {
					t.Fatal("missing choice")
				}
				found := false
				for _, c := range k.DiscretionariesAt(choice.Source) {
					if c.Variant == choice.Variant {
						found = true
						if !c.SourceSplit.Valid || c.SourceSplit.ByteOffset != tc.split {
							t.Fatal(c)
						}
					}
				}
				if !found || policy.calls != 1 {
					t.Fatal("missing cached variant", policy.calls)
				}
				if positions, err := k.BreakpointPositions(breaks); err != nil || !reflect.DeepEqual(positions, []uint64{tc.split, uint64(len(tc.text))}) {
					t.Fatal("wrong source positions", positions, err)
				}
				if _, err = breakAt(tc.width); err != nil || policy.calls != 1 {
					t.Fatal("cache not reused", err, policy.calls)
				}
				if _, err = breakAt(100 * dimen.PT); err != nil || len(k.SelectedDiscretionaries) != 0 {
					t.Fatal("stale selection", err)
				}
				after := make([]khipu.KnotCore, len(k.Kind))
				for i := range after {
					after[i] = k.KnotByIndex(i)
				}
				if !reflect.DeepEqual(before, after) {
					t.Fatal("base paragraph modified")
				}
			})
		}
	}
}
