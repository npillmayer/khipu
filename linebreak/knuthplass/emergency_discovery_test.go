package knuthplass

import (
	"maps"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/uax/bidi"
)

type emergencyPolicyFunc func(string, *khipu.Params) ([]khipu.HyphenationFragment, error)

func (f emergencyPolicyFunc) Discretionaries(word string, params *khipu.Params) ([]khipu.HyphenationFragment, error) {
	return f(word, params)
}

type emergencyEncoding struct {
	owner   *khipu.Khipukamayuq
	khp     *khipu.Khipu
	streams int
}

// Count real shaping streams as well as policy calls: a warm owner lookup
// must not silently re-shape an already prepared positive or empty response.
func encodeEmergencyText(t *testing.T, source string, policy khipu.Hyphenator) *emergencyEncoding {
	t.Helper()
	f := &emergencyEncoding{}
	f.owner = khipu.NewKhipukamayuq(khipu.Config{Hyphenator: policy, Streaming: &khipu.StreamingConfig{
		NewShaper: func() (contracts.StreamingShaper, error) {
			f.streams++
			return helpers.NewSyntheticStreamingShaper(0)
		},
		Resolver: helpers.DefaultStyleResolver{},
		Defaults: contracts.ResolvedRunStyle{
			Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT, Spacing: contracts.SpacingElastic,
		},
	}})
	text := styled.TextFromString(source)
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.khp, err = f.owner.EncodeParagraph(t.Context(), para)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func emergencyKnotSnapshot(khp *khipu.Khipu) []khipu.KnotCore {
	knots := make([]khipu.KnotCore, len(khp.Kind))
	for at := range knots {
		knots[at] = khp.KnotByIndex(at)
	}
	return knots
}

func TestEmergencyReusesOwnerDiscoveryAcrossPasses(t *testing.T) {
	for _, source := range []string{"abcd", "ab\u00adcd"} {
		for _, positive := range []bool{false, true} {
			name := source + "/empty"
			if positive {
				name = source + "/positive"
			}
			t.Run(name, func(t *testing.T) {
				var policyInputs []string
				f := encodeEmergencyText(t, source, emergencyPolicyFunc(func(word string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
					policyInputs = append(policyInputs, word)
					if !positive {
						return nil, nil
					}
					if word == "\u00ad" {
						return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
					}
					return []khipu.HyphenationFragment{{PreBreak: "ab-", PostBreak: "cd", SourceSplit: khipu.SourceSplit{ByteOffset: 2, Valid: true}}}, nil
				}))
				khp := f.khp
				if len(khp.Kind) != 2 || khp.W[0] != 20*dimen.PT || len(policyInputs) != 0 || f.streams != 1 {
					t.Fatal("fixture must contain one lazily hyphenated 20 pt textbox and a trailer")
				}
				original := emergencyKnotSnapshot(khp)
				khp.SelectedDiscretionaries = map[int]khipu.DiscretionarySelection{0: {Source: 0, Variant: 9}}
				stale := maps.Clone(khp.SelectedDiscretionaries)
				requests := 0
				params := NewDefaultParameters()
				params.EmergencyStretch = 2 * dimen.PT
				params.DiscretionaryProvider = emergencyProviderFunc(func(k *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error) {
					requests++
					if at != 0 {
						t.Fatalf("unexpected discovery at knot %d", at)
					}
					return f.owner.DiscretionaryCandidates(k, at)
				})
				layout := khipu.Params{LineShape: linebreak.RectangularParShape(17 * dimen.PT)}
				passes, err := paragraphPasses(params)
				if err != nil {
					t.Fatal(err)
				}
				// The real 15 pt pre-break fragment has no stretch and fails
				// pass 2. E=2 makes it acceptable in pass 3. A declined request
				// instead requires the unchanged overfull fallback policy.
				for _, pass := range passes[:2] {
					result, err := breakParagraphPassRefs(khp, layout, params, pass)
					if err != nil || result.complete || !maps.Equal(stale, khp.SelectedDiscretionaries) {
						t.Fatalf("non-final search completed or published: %+v %v", result, err)
					}
					if pass.id == passPreTolerance && (requests != 0 || len(policyInputs) != 0 || f.streams != 1) {
						t.Fatal("pass 1 performed eager discovery")
					}
				}
				wantInput := "abcd"
				if source != "abcd" {
					wantInput = "\u00ad"
				}
				if !slices.Equal(policyInputs, []string{wantInput}) || requests == 0 {
					t.Fatal("pass 2 did not populate owner discovery cache", policyInputs, requests)
				}
				candidates := slices.Clone(khp.DiscretionariesAt(0))
				if positive && (len(candidates) != 1 || f.streams <= 1) || !positive && (len(candidates) != 0 || f.streams != 1) {
					t.Fatal("fixture did not exercise the expected positive/empty cache", candidates, f.streams)
				}
				beforeRequests, beforeStreams := requests, f.streams
				result, err := breakParagraphPassRefs(khp, layout, params, passes[2])
				if err != nil || !result.complete || requests <= beforeRequests || len(policyInputs) != 1 || f.streams != beforeStreams {
					t.Fatalf("pass 3 did not reuse owner response: %+v err=%v requests=%d policy=%v streams=%d", result, err, requests, policyInputs, f.streams)
				}
				want := []BreakRef{PlainBreak(1)}
				wantBadness := merits(10000)
				if positive {
					want = []BreakRef{{At: 0, Variant: 1}, PlainBreak(1)}
					wantBadness = 100
				}
				if !slices.Equal(result.breaks, want) || result.quality.worstBadness != wantBadness || !maps.Equal(stale, khp.SelectedDiscretionaries) {
					t.Fatal("wrong emergency path or premature publication", result)
				}
				// Repeat the whole public call on the same owner/paragraph. No
				// new policy or shaping work is allowed, and stale selections
				// must be replaced by exactly the final path's discretionary.
				warm, err := selectParagraphBreaks(khp, layout, params)
				if err != nil || !warm.complete || warm.pass != result.pass || warm.quality != result.quality || !slices.Equal(warm.breaks, result.breaks) {
					t.Fatal("warm discovery changed pass, quality or selected path", warm, err)
				}
				breaks, err := BreakParagraph(khp, layout, params)
				selected := map[int]khipu.DiscretionarySelection{}
				if positive {
					selected[0] = khipu.DiscretionarySelection{Source: 0, Variant: 1}
				}
				if err != nil || !slices.Equal(breaks, projectBreakRefs(want)) || !maps.Equal(selected, khp.SelectedDiscretionaries) ||
					len(policyInputs) != 1 || f.streams != beforeStreams || !slices.Equal(candidates, khp.DiscretionariesAt(0)) || !slices.Equal(original, emergencyKnotSnapshot(khp)) {
					t.Fatal("warm public run changed discovery, geometry or selection ownership", breaks, err)
				}
			})
		}
	}
}

func TestEmergencyAutomaticDiscoveryGate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		width, e dimen.DU
		badness  merits
		wantAt   int
	}{
		{"real capacity only", 90, 0, 10000, 2},
		{"equal to pretolerance suppresses", 90, 10, 100, -1},
		{"above pretolerance but accepted", 90, 9, 137, 2},
		{"still screened out", 90, 1, 10000, 2},
		{"overfull rescue survives", 150, 100, 10000, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &testDiscretionaryProvider{}
			params := NewDefaultParameters()
			params.DiscretionaryProvider = provider
			kp, err := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}, params)
			if err != nil {
				t.Fatal(err)
			}
			kp.hyphenating, kp.final, kp.emergencyStretch = true, true, tc.e*dimen.BP
			khp := newKhipu([]node{
				{w: tc.width, minw: tc.width, maxw: tc.width, p: InfinityDemerits, kind: khipu.KTTextBox},
				{kind: khipu.KTGlue},
				{w: 100, minw: 100, maxw: 100, p: InfinityDemerits, kind: khipu.KTTextBox},
				{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
			})
			kp.end = PlainBreak(3)
			kp.prepareRetirement(khp)
			if _, err := kp.processKnot(khp, 0); err != nil {
				t.Fatal(err)
			}
			if evs := kp.evaluateCandidates(kp.root, 0); len(evs) != 1 || evs[0].badness != tc.badness {
				t.Fatal("fixture has wrong badness", evs)
			}
			if _, err := kp.processKnot(khp, 1); err != nil {
				t.Fatal(err)
			}
			want := []int(nil)
			if tc.wantAt >= 0 {
				want = []int{tc.wantAt}
			}
			if !slices.Equal(provider.requestedAt, want) {
				t.Fatalf("provider requests=%v want=%v", provider.requestedAt, want)
			}
		})
	}
}

func TestEmergencyExplicitRequestBypassesAutomaticGate(t *testing.T) {
	var inputs []string
	f := encodeEmergencyText(t, "ab\u00adcd", emergencyPolicyFunc(func(word string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
		inputs = append(inputs, word)
		return []khipu.HyphenationFragment{{PreBreak: "-"}}, nil
	}))
	params := NewDefaultParameters()
	params.DiscretionaryProvider = f.owner
	kp, err := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(30 * dimen.PT)}, params)
	if err != nil {
		t.Fatal(err)
	}
	kp.hyphenating, kp.final, kp.emergencyStretch = true, true, 20*dimen.PT
	kp.end = PlainBreak(len(f.khp.Kind) - 1)
	kp.prepareRetirement(f.khp)
	if !f.khp.HasExplicitDiscretionary(0) || f.khp.Penalty[0] < InfinityDemerits {
		t.Fatal("fixture must have an explicit marker inside an otherwise unbreakable box")
	}
	if live, err := kp.processKnot(f.khp, 0); err != nil || !live {
		t.Fatal(live, err)
	}
	evs := kp.evaluateCandidates(kp.root, 0)
	if len(evs) != 1 || evs[0].badness != 12 || kp.wantsHyphenation(evs[0]) {
		t.Fatal("ordinary line should not trigger automatic discovery", evs)
	}
	if !slices.Equal(inputs, []string{"\u00ad"}) || len(f.khp.DiscretionariesAt(0)) != 1 {
		t.Fatal("explicit request was suppressed by ordinary badness", inputs)
	}
	if _, ok := kp.Pred(BreakRef{At: 0, Variant: 1}, 1); !ok {
		t.Fatal("explicit alternative was requested but not made selectable")
	}
}
