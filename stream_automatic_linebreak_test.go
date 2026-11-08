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

type streamingWordPolicy struct{ requests []string }

func (p *streamingWordPolicy) Discretionaries(text string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	p.requests = append(p.requests, text)
	switch text {
	case "abcd":
		return []khipu.HyphenationFragment{{PreBreak: "ab-", PostBreak: "cd", SourceSplit: khipu.SourceSplit{ByteOffset: 2, Valid: true}}}, nil
	case "\u00ad":
		return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
	}
	return nil, nil
}

func TestStreamingAutomaticBreaksEndToEnd(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		for _, source := range []string{"a abcd", "a ab\u00adcd"} {
			t.Run(engine+source, func(t *testing.T) {
				policy := &streamingWordPolicy{}
				owner := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: policy, Streaming: &khipu.StreamingConfig{
					Resolver: helpers.DefaultStyleResolver{}, Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT, Spacing: contracts.SpacingElastic},
					NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(0) },
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
					p := knuthplass.NewDefaultParameters()
					layout := knuthplass.DefaultLayout(shape)
					p.DiscretionaryProvider = owner
					if width == 25*dimen.PT {
						layout.RightSkip = khipu.KnotCore{MaxW: dimen.PT}
						p.EmergencyStretch = 0
					}
					return knuthplass.BreakParagraph(k, layout, p)
				}
				if wide, err := breakAt(100 * dimen.PT); err != nil || len(wide) != 1 || len(policy.requests) != 0 {
					t.Fatal("eager lookup", wide, err, policy.requests)
				}
				breaks, err := breakAt(25 * dimen.PT)
				if err != nil || len(breaks) != 2 || len(k.SelectedDiscretionaries) != 1 {
					t.Fatal(breaks, k.SelectedDiscretionaries, err)
				}
				wantRequest, wantSplit := "abcd", uint64(4)
				if source != "a abcd" {
					wantRequest, wantSplit = "\u00ad", 6
				}
				if !reflect.DeepEqual(policy.requests, []string{wantRequest}) {
					t.Fatal("wrong discovery route", policy.requests)
				}
				if positions, err := k.BreakpointPositions(breaks); err != nil || !reflect.DeepEqual(positions, []uint64{wantSplit, uint64(len(source))}) {
					t.Fatal("wrong source positions", positions, err)
				}
				choice, _ := k.SelectedDiscretionaryAt(breaks[0])
				c := k.DiscretionariesAt(choice.Source)[0]
				if c.Variant != choice.Variant || !c.SourceSplit.Valid || c.SourceSplit.ByteOffset != wantSplit || c.PreBreak.W != 15*dimen.PT || c.PostBreak.W != 10*dimen.PT {
					t.Fatal(c)
				}
				if _, err := breakAt(25 * dimen.PT); err != nil || len(policy.requests) != 1 {
					t.Fatal("cache miss", err, policy.requests)
				}
				if _, err := breakAt(100 * dimen.PT); err != nil || len(k.SelectedDiscretionaries) != 0 {
					t.Fatal("stale decisions", err)
				}
				for i := range before {
					if before[i] != k.KnotByIndex(i) {
						t.Fatal("base knots changed")
					}
				}
			})
		}
	}
}
