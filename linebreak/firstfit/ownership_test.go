package firstfit

import (
	"errors"
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

type selectionProviderFunc func(*khipu.Khipu, int) ([]khipu.DiscretionaryCandidate, error)

func (f selectionProviderFunc) DiscretionaryCandidates(k *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error) {
	return f(k, at)
}

func selectionParagraph(penalty int) *khipu.Khipu {
	k := newKhipu([]node{
		{w: 15, minw: 15, maxw: 15, kind: khipu.KTTextBox},
		{w: 20, minw: 20, maxw: 20, p: penalty, kind: khipu.KTTextBox},
		{w: 1, minw: 1, maxw: 1, kind: khipu.KTTextBox},
		{w: 20, minw: 20, maxw: 20, p: penalty, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	k.Flags[4] = 0
	return k
}

func TestSelectionsPublishOnlyOnSuccess(t *testing.T) {
	// Both ordinary and prohibited textbox branches can apply discretionaries.
	for _, penalty := range []int{0, InfinityDemerits} {
		for _, failAt := range []int{1, 3} {
			for _, stale := range []bool{false, true} {
				k := selectionParagraph(penalty)
				if stale {
					k.SelectedDiscretionaries = map[int]khipu.DiscretionarySelection{0: {Source: 0, Variant: 9}}
				}
				before := maps.Clone(k.SelectedDiscretionaries)
				alias := k.SelectedDiscretionaries
				failure := errors.New("provider failed")
				failing := true
				var calls []int
				params := NewDefaultParameters()
				params.DiscretionaryProvider = selectionProviderFunc(func(k *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error) {
					calls = append(calls, at)
					if !reflect.DeepEqual(k.SelectedDiscretionaries, before) {
						t.Fatal("partial selections visible to provider")
					}
					if failing && at == failAt {
						return nil, failure
					}
					cand := khipu.DiscretionaryCandidate{
						Variant:   1,
						PreBreak:  khipu.KnotCore{W: 9 * dimen.BP, MinW: 9 * dimen.BP, MaxW: 9 * dimen.BP, Kind: khipu.KTTextBox},
						PostBreak: khipu.KnotCore{W: 11 * dimen.BP, MinW: 11 * dimen.BP, MaxW: 11 * dimen.BP, Kind: khipu.KTTextBox},
					}
					k.AddDiscretionaryCandidate(at, cand)
					return []khipu.DiscretionaryCandidate{cand}, nil
				})
				layout := DefaultLayout(linebreak.RectangularParShape(24 * dimen.BP))
				got, err := BreakParagraph(k, layout, params)
				if !errors.Is(err, failure) || got != nil || !reflect.DeepEqual(k.SelectedDiscretionaries, before) || !reflect.DeepEqual(alias, before) {
					t.Fatalf("failed call changed decisions: %v %v %+v", got, err, k.SelectedDiscretionaries)
				}
				wantCalls := []int{1}
				if failAt == 3 {
					wantCalls = []int{1, 3}
				}
				if !slices.Equal(calls, wantCalls) {
					t.Fatalf("calls %v, want %v", calls, wantCalls)
				}
				if failAt == 3 && len(k.DiscretionariesAt(1)) == 0 {
					t.Fatal("candidate cache unexpectedly rolled back")
				}
				failing, calls = false, nil
				got, err = BreakParagraph(k, layout, params)
				want := map[int]khipu.DiscretionarySelection{1: {Source: 1, Variant: 1}, 3: {Source: 3, Variant: 1}}
				if err != nil || !slices.Equal(got, []int{1, 3, 4}) || !maps.Equal(k.SelectedDiscretionaries, want) || !slices.Equal(calls, []int{1, 3}) {
					t.Fatalf("retry did not publish complete result: %v %v %+v", got, err, k.SelectedDiscretionaries)
				}
				// A later successful run without selections clears the committed map.
				params.DiscretionaryProvider = nil
				params.MinHyphenGain = 100 * dimen.BP
				if _, err := BreakParagraph(k, layout, params); err != nil || len(k.SelectedDiscretionaries) != 0 {
					t.Fatal("stale selections survived success", err)
				}
			}
		}
	}
}

func TestValidationFailurePreservesSelections(t *testing.T) {
	for _, empty := range []bool{false, true} {
		k := selectionParagraph(InfinityDemerits)
		layout := khipu.Params{} // missing shape
		if empty {
			k = newKhipu(nil)
			layout = DefaultLayout(linebreak.RectangularParShape(24 * dimen.BP))
		}
		k.SelectedDiscretionaries = map[int]khipu.DiscretionarySelection{0: {Source: 0, Variant: 9}}
		before := maps.Clone(k.SelectedDiscretionaries)
		if breaks, err := BreakParagraph(k, layout, nil); err == nil || breaks != nil || !maps.Equal(k.SelectedDiscretionaries, before) {
			t.Fatal("validation failure changed result", breaks, err)
		}
	}
}

func TestDefaultParametersAreIndependent(t *testing.T) {
	p := NewDefaultParameters()
	if p.MinHyphenGain != 0 || p.DiscretionaryProvider != nil {
		t.Fatal("defaults changed", p)
	}
	p.MinHyphenGain = 100 * dimen.BP
	p.DiscretionaryProvider = selectionProviderFunc(func(*khipu.Khipu, int) ([]khipu.DiscretionaryCandidate, error) {
		t.Fatal("mutated defaults leaked")
		return nil, nil
	})
	fresh := NewDefaultParameters()
	if fresh.MinHyphenGain != 0 || fresh.DiscretionaryProvider != nil {
		t.Fatal("constructor shares state")
	}
	layout := DefaultLayout(linebreak.RectangularParShape(24 * dimen.BP))
	a, err := BreakParagraph(selectionParagraph(InfinityDemerits), layout, nil)
	b, err2 := BreakParagraph(selectionParagraph(InfinityDemerits), layout, fresh)
	if err != nil || err2 != nil || !slices.Equal(a, b) {
		t.Fatal("nil differs from explicit defaults", a, b, err, err2)
	}
}
