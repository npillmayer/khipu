package knuthplass

import (
	"maps"
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

func TestEmergencyDeterministicTiesAndWarmRuns(t *testing.T) {
	want := []BreakRef{PlainBreak(0), {At: 1, Variant: 1}, PlainBreak(2)}
	for run := range 20 {
		khp := emergencyLaterDiscovery()
		original := emergencyKnotSnapshot(khp)
		candidate := khipu.DiscretionaryCandidate{
			Variant:   1,
			PreBreak:  khipu.KnotCore{Kind: khipu.KTTextBox, W: 90 * dimen.BP, MinW: 90 * dimen.BP, MaxW: 90 * dimen.BP},
			PostBreak: khipu.KnotCore{Kind: khipu.KTTextBox, W: 60 * dimen.BP, MinW: 60 * dimen.BP, MaxW: 60 * dimen.BP},
		}
		other := candidate
		other.Variant = 2 // identical geometry/cost, different path identity
		candidates := []khipu.DiscretionaryCandidate{candidate, other}
		if run%2 != 0 {
			slices.Reverse(candidates)
		}
		provider := &testDiscretionaryProvider{want: slices.Clone(candidates)}
		params := NewDefaultParameters()
		params.EmergencyStretch, params.DiscretionaryProvider = 10*dimen.BP, provider
		layout := khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}
		quality := pathQuality{
			totalCost:    2*calcDemerits(100, 0, params) + calcDemerits(0, 0, params) + params.FinalHyphenDemerits,
			worstBadness: 100,
		}
		for warm := range 3 {
			khp.SelectedDiscretionaries = map[int]khipu.DiscretionarySelection{0: {Source: 0, Variant: 9}}
			before := maps.Clone(khp.SelectedDiscretionaries)
			result, err := selectParagraphBreaks(khp, layout, params)
			if err != nil || !result.complete || result.pass.id != passEmergency || result.pass.emergencyStretch != params.EmergencyStretch ||
				!slices.Equal(result.breaks, want) || result.quality != quality || !maps.Equal(before, khp.SelectedDiscretionaries) {
				t.Fatalf("run=%d warm=%d: unstable result or premature publication: %+v %v", run, warm, result, err)
			}
			breaks, err := BreakParagraph(khp, layout, params)
			wantSelected := map[int]khipu.DiscretionarySelection{1: {Source: 1, Variant: 1}}
			if err != nil || !slices.Equal(breaks, projectBreakRefs(want)) || !maps.Equal(khp.SelectedDiscretionaries, wantSelected) ||
				!slices.Equal(original, emergencyKnotSnapshot(khp)) || !slices.Equal(candidates, khp.DiscretionariesAt(1)) || !slices.Equal(candidates, provider.want) {
				t.Fatalf("run=%d warm=%d: wrong winner, changed widths or mutated candidate storage: %v %v", run, warm, breaks, err)
			}
		}
	}
}

func TestEmergencyRetroactiveSeedsRemainPhysical(t *testing.T) {
	for _, uncached := range []bool{false, true} {
		params := NewDefaultParameters()
		candidate := khipu.DiscretionaryCandidate{
			Variant:   1,
			PreBreak:  khipu.KnotCore{Kind: khipu.KTTextBox, W: 90 * dimen.BP, MinW: 90 * dimen.BP, MaxW: 90 * dimen.BP},
			PostBreak: khipu.KnotCore{Kind: khipu.KTTextBox, W: 60 * dimen.BP, MinW: 60 * dimen.BP, MaxW: 60 * dimen.BP},
		}
		params.DiscretionaryProvider = &testDiscretionaryProvider{want: []khipu.DiscretionaryCandidate{candidate}}
		khp := newKhipu([]node{
			{w: 90, minw: 90, maxw: 90, p: InfinityMerits, kind: khipu.KTTextBox},
			{w: 150, minw: 150, maxw: 150, p: InfinityDemerits, kind: khipu.KTTextBox},
			{w: 5, minw: 5, maxw: 5, kind: khipu.KTGlue},
			{w: 30, minw: 30, maxw: 30, p: InfinityDemerits, kind: khipu.KTTextBox},
			{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
		})
		next := khipu.DiscretionaryCandidate{
			Variant:   1,
			PreBreak:  khipu.KnotCore{Kind: khipu.KTTextBox, W: 25 * dimen.BP, MinW: 25 * dimen.BP, MaxW: 25 * dimen.BP},
			PostBreak: khipu.KnotCore{Kind: khipu.KTTextBox, W: 5 * dimen.BP, MinW: 5 * dimen.BP, MaxW: 5 * dimen.BP},
		}
		khp.AddDiscretionaryCandidate(3, next)
		original := emergencyKnotSnapshot(khp)
		kp, err := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}, params)
		if err != nil {
			t.Fatal(err)
		}
		kp.hyphenating, kp.final, kp.emergencyStretch = true, true, 10*dimen.BP
		kp.boundaryEvals.disabled, kp.end = uncached, PlainBreak(4)
		kp.prepareRetirement(khp)
		for at := range 3 {
			if live, err := kp.processKnot(khp, at); err != nil || !live {
				t.Fatalf("knot %d: %v %v", at, live, err)
			}
		}
		from := BreakRef{At: 1, Variant: 1}
		seed, ok := kp.seeds[origin{from, 2}]
		// Retroactive discovery has consumed the 5 pt suffix glue. Keep it
		// in live bookkeeping, not in the immutable 60 pt post-break seed.
		if !ok || seed != seedBookkeepingFromKnot(candidate.PostBreak) || kp.paths[from][2].segment != (linebreak.WSS{W: 65 * dimen.BP, Min: 65 * dimen.BP, Max: 65 * dimen.BP}) {
			t.Fatal("virtual capacity or consumed suffix leaked into the rescan seed", seed)
		}
		if live, err := kp.processKnot(khp, 3); err != nil || !live {
			t.Fatal(live, err)
		}
		target := BreakRef{At: 3, Variant: 1}
		pred, ok := kp.Pred(target, 3)
		// Rescanning 60 + 5 + 25 gives 90 pt. Only virtual E=10 makes
		// this successor acceptable, without adding E to either real seed.
		if !ok || pred.from != from || pred.cost != calcDemerits(100, 0, params) || kp.seeds[origin{from, 2}] != seed ||
			kp.seeds[origin{target, 3}] != seedBookkeepingFromKnot(next.PostBreak) || !slices.Equal(original, emergencyKnotSnapshot(khp)) ||
			khp.DiscretionariesAt(1)[0] != candidate || khp.DiscretionariesAt(3)[0] != next {
			t.Fatal("emergency rescan changed physical fragments/seeds or duplicated suffix", pred)
		}
	}
}
