package firstfit

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

func policyCandidate(variant uint16, width dimen.DU, penalty khipu.Penalty) khipu.DiscretionaryCandidate {
	return khipu.DiscretionaryCandidate{
		Variant:   variant,
		PreBreak:  khipu.KnotCore{Kind: khipu.KTTextBox, W: width, MinW: width, MaxW: width, Penalty: penalty},
		PostBreak: khipu.KnotCore{Kind: khipu.KTTextBox, W: 12 * dimen.BP, MinW: 12 * dimen.BP, MaxW: 12 * dimen.BP},
	}
}

func TestDiscretionaryEligibilityAndRanking(t *testing.T) {
	for _, tc := range []struct {
		name       string
		candidates []khipu.DiscretionaryCandidate
		gain       dimen.DU
		want       uint16 // zero means no eligible variant
	}{
		{"prohibited widest", []khipu.DiscretionaryCandidate{policyCandidate(1, 10*dimen.BP, InfinityDemerits), policyCandidate(2, 8*dimen.BP, 50)}, 0, 2},
		{"above sentinel", []khipu.DiscretionaryCandidate{policyCandidate(1, 10*dimen.BP, InfinityDemerits+1)}, 0, 0},
		{"all prohibited", []khipu.DiscretionaryCandidate{policyCandidate(1, 10*dimen.BP, InfinityDemerits), policyCandidate(2, 8*dimen.BP, InfinityDemerits+1)}, 0, 0},
		{"width outranks finite reward", []khipu.DiscretionaryCandidate{policyCandidate(1, 8*dimen.BP, -50), policyCandidate(2, 9*dimen.BP, InfinityDemerits-1)}, 0, 2},
		{"equal width lower penalty", []khipu.DiscretionaryCandidate{policyCandidate(1, 8*dimen.BP, 50), policyCandidate(2, 8*dimen.BP, -10)}, 0, 2},
		{"exact tie retains order", []khipu.DiscretionaryCandidate{policyCandidate(7, 8*dimen.BP, 50), policyCandidate(2, 8*dimen.BP, 50)}, 0, 7},
		{"gain exactly threshold", []khipu.DiscretionaryCandidate{policyCandidate(1, 5*dimen.BP, 50)}, 5 * dimen.BP, 1},
		{"gain below threshold", []khipu.DiscretionaryCandidate{policyCandidate(1, 5*dimen.BP-1, 50)}, 5 * dimen.BP, 0},
		{"zero gain allowed", []khipu.DiscretionaryCandidate{policyCandidate(1, 0, 50)}, 0, 1},
		{"zero gain rejected by positive threshold", []khipu.DiscretionaryCandidate{policyCandidate(1, 0, 50)}, 1, 0},
		{"overfull despite reward", []khipu.DiscretionaryCandidate{policyCandidate(1, 11*dimen.BP, -500)}, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := newKhipu([]node{{w: 20, minw: 20, maxw: 20, kind: khipu.KTTextBox}})
			for _, c := range tc.candidates {
				k.AddDiscretionaryCandidate(0, c)
			}
			before := slices.Clone(k.DiscretionariesAt(0))
			base := segmentSeedFromKnot(khipu.KnotCore{Kind: khipu.KTTextBox, W: 10 * dimen.BP, MinW: 10 * dimen.BP, MaxW: 10 * dimen.BP})
			layout := khipu.Params{LineShape: linebreak.RectangularParShape(20 * dimen.BP)}
			params := NewDefaultParameters()
			params.MinHyphenGain = tc.gain
			got, seed, ok, err := tryLooseHyphenation(k, 0, base, base.width(layout), 20*dimen.BP, layout, params)
			if err != nil || ok != (tc.want != 0) || got.Variant != tc.want {
				t.Fatalf("got variant %d, ok=%v, err=%v; want %d", got.Variant, ok, err, tc.want)
			}
			if ok && seed.width(layout).W != got.PostBreak.W {
				t.Fatal("wrong post-break seed")
			}
			if !reflect.DeepEqual(k.DiscretionariesAt(0), before) || len(k.SelectedDiscretionaries) != 0 {
				t.Fatal("evaluation modified candidates or published selections")
			}
		})
	}
}

func TestProhibitedDiscretionariesThroughPublicBreaker(t *testing.T) {
	for _, sourcePenalty := range []int{0, InfinityDemerits} {
		for _, provider := range []bool{false, true} {
			for _, allowed := range []bool{false, true} {
				t.Run(fmt.Sprintf("sourcePenalty%d/provider%v/allowed%v", sourcePenalty, provider, allowed), func(t *testing.T) {
					k := newKhipu([]node{
						{w: 15, minw: 15, maxw: 15, kind: khipu.KTTextBox},
						{w: 20, minw: 20, maxw: 20, p: sourcePenalty, kind: khipu.KTTextBox},
						{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
					})
					k.Flags[2] = 0
					candidates := []khipu.DiscretionaryCandidate{policyCandidate(1, 9*dimen.BP, InfinityDemerits), policyCandidate(2, 9*dimen.BP, InfinityDemerits+1)}
					if allowed {
						candidates = append(candidates, policyCandidate(3, 8*dimen.BP, 50))
					}
					params := NewDefaultParameters()
					p := &testDiscretionaryProvider{want: candidates}
					if provider {
						params.DiscretionaryProvider = p
					} else {
						for _, c := range candidates {
							k.AddDiscretionaryCandidate(1, c)
						}
					}
					k.SelectedDiscretionaries = map[int]khipu.DiscretionarySelection{1: {Source: 1, Variant: 99}}
					got, err := BreakParagraph(k, DefaultLayout(linebreak.RectangularParShape(24*dimen.BP)), params)
					want := []int{0, 2}
					if allowed {
						want = []int{1, 2}
					}
					if err != nil || !slices.Equal(got, want) {
						t.Fatalf("breaks %v, err=%v; want %v", got, err, want)
					}
					if provider && (p.calls != 1 || !slices.Equal(p.requestedAt, []int{1})) {
						t.Fatal("unexpected provider requests", p.requestedAt)
					}
					if allowed {
						choice, ok := k.SelectedDiscretionaryAt(1)
						if !ok || choice.Source != 1 || choice.Variant != 3 || len(k.SelectedDiscretionaries) != 1 {
							t.Fatal("wrong published variant", k.SelectedDiscretionaries)
						}
					} else if len(k.SelectedDiscretionaries) != 0 {
						t.Fatal("prohibited or stale variant published")
					}
				})
			}
		}
	}
}
