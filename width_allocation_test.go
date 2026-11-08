package khipu

import (
	"errors"
	"math/rand"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/khipu/dimen"
)

func allocationItem(w, min, max dimen.DU) KnotCore {
	return KnotCore{W: w, MinW: min, MaxW: max}
}

func TestAllocateWidths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		items  []KnotCore
		target dimen.DU
		want   []dimen.DU
	}{
		{"empty", nil, 0, nil},
		{"exact", []KnotCore{allocationItem(10, 5, 20), allocationItem(20, 0, dimen.Fill)}, 30, []dimen.DU{10, 20}},
		{"proportional stretch", []KnotCore{allocationItem(10, 10, 20), allocationItem(20, 20, 50)}, 50, []dimen.DU{15, 35}},
		{"stretch remainder", []KnotCore{allocationItem(0, 0, 1), allocationItem(0, 0, 3)}, 2, []dimen.DU{1, 1}},
		{"larger remainder wins", []KnotCore{allocationItem(0, 0, 1), allocationItem(0, 0, 2)}, 1, []dimen.DU{0, 1}},
		{"ties left to right", []KnotCore{allocationItem(0, 0, 3), allocationItem(0, 0, 3), allocationItem(0, 0, 3)}, 2, []dimen.DU{1, 1, 0}},
		{"fixed and saturated", []KnotCore{allocationItem(2, 2, 2), allocationItem(1, 0, 4), allocationItem(3, 3, 5)}, 11, []dimen.DU{2, 4, 5}},
		{"proportional shrink", []KnotCore{allocationItem(10, 5, 10), allocationItem(20, 5, 20)}, 20, []dimen.DU{7, 13}},
		{"shrink remainder", []KnotCore{allocationItem(10, 8, 10), allocationItem(20, 15, 20)}, 27, []dimen.DU{9, 18}},
		{"shrink exhaustion", []KnotCore{allocationItem(10, 5, 10), allocationItem(20, 5, 20)}, 10, []dimen.DU{5, 5}},
		{"finite shrink despite fill", []KnotCore{allocationItem(5, 0, dimen.Filll), allocationItem(5, 0, 10)}, 7, []dimen.DU{3, 4}},
		{"fil over finite", []KnotCore{allocationItem(10, 0, 100), allocationItem(2, 2, dimen.Fil)}, 20, []dimen.DU{10, 10}},
		{"fill over fil", []KnotCore{allocationItem(0, 0, dimen.Fil), allocationItem(0, 0, dimen.Fill)}, 20, []dimen.DU{0, 20}},
		{"filll over fill", []KnotCore{allocationItem(1, 0, dimen.Fill), allocationItem(2, 0, dimen.Filll), allocationItem(3, 0, dimen.Fil)}, 10, []dimen.DU{1, 6, 3}},
		{"equal fill weights", []KnotCore{allocationItem(2, 0, dimen.Fill), allocationItem(5, 0, dimen.Fill)}, 12, []dimen.DU{5, 7}},
		{"zero width capacity", []KnotCore{allocationItem(0, 0, 10)}, 5, []dimen.DU{5}},
		{"negative finite item", []KnotCore{allocationItem(-5, -5, 5), allocationItem(10, 10, 10)}, 10, []dimen.DU{0, 10}},
		{"near DU limit", []KnotCore{allocationItem(-dimen.Fil+1, -dimen.Fil+1, dimen.Fil-1), allocationItem(dimen.Fil-1, dimen.Fil-1, dimen.Fil-1)}, dimen.Fil - 1, []dimen.DU{0, dimen.Fil - 1}},
		{"capacity sum exceeds DU", []KnotCore{allocationItem(0, 0, dimen.Fil-1), allocationItem(0, 0, dimen.Fil-1), allocationItem(0, 0, dimen.Fil-1)}, dimen.Fil - 1, []dimen.DU{715827881, 715827881, 715827881}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := slices.Clone(tc.items)
			got, err := allocateWidths(tc.items, tc.target)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatal(got, tc.want, err)
			}
			if !reflect.DeepEqual(before, tc.items) {
				t.Fatal("mutated input")
			}
			checkWidthAllocation(t, tc.items, tc.target, got)
			again, err := allocateWidths(tc.items, tc.target)
			if err != nil || !slices.Equal(got, again) {
				t.Fatal("not deterministic", err)
			}
			if len(got) > 0 {
				got[0]++
				if !reflect.DeepEqual(before, tc.items) || slices.Equal(got, again) {
					t.Fatal("aliased output")
				}
			}
		})
	}
}

func checkWidthAllocation(t *testing.T, items []KnotCore, target dimen.DU, got []dimen.DU) {
	t.Helper()
	if len(got) != len(items) {
		t.Fatal("lost items")
	}
	var sum int64
	for i, w := range got {
		if w <= -dimen.Fil || w >= dimen.Fil || w < items[i].MinW || w > items[i].MaxW {
			t.Fatal("out of bounds", i, w, items[i])
		}
		sum += int64(w)
	}
	if sum != int64(target) {
		t.Fatal("lost width", sum, target)
	}
}

func TestAllocateWidthsRejects(t *testing.T) {
	for _, tc := range []struct {
		name     string
		items    []KnotCore
		target   dimen.DU
		capacity bool
	}{
		{"empty nonzero", nil, 1, true},
		{"no stretch", []KnotCore{allocationItem(10, 5, 10)}, 11, true},
		{"too much stretch", []KnotCore{allocationItem(10, 5, 15)}, 16, true},
		{"no shrink", []KnotCore{allocationItem(10, 10, 20)}, 9, true},
		{"too much shrink", []KnotCore{allocationItem(10, 5, 20)}, 4, true},
		{"negative target", nil, -1, false},
		{"fill target", nil, dimen.Fil, false},
		{"unordered min", []KnotCore{allocationItem(10, 11, 20)}, 10, false},
		{"unordered max", []KnotCore{allocationItem(10, 0, 9)}, 10, false},
		{"infinite shrink", []KnotCore{allocationItem(10, -dimen.Fil, 20)}, 10, false},
		{"infinity max", []KnotCore{allocationItem(0, 0, dimen.Infinity)}, 0, false},
		{"nonfinite natural", []KnotCore{allocationItem(dimen.Fil, 0, dimen.Fill)}, 0, false},
		{"negative total", []KnotCore{allocationItem(-1, -1, 1)}, 0, false},
		{"natural sum overflow DU", []KnotCore{allocationItem(dimen.Fil-1, 0, dimen.Fill), allocationItem(10, 0, 10)}, 0, false},
		{"nonfinite assignment", []KnotCore{allocationItem(dimen.Fil-2, 0, dimen.Fill), allocationItem(-dimen.Fil+2, -dimen.Fil+2, -dimen.Fil+2)}, dimen.Fil - 1, false},
		{"rounding reaches sentinel", []KnotCore{allocationItem(dimen.Fil-2, 0, dimen.Fill), allocationItem(0, 0, dimen.Fill), allocationItem(-dimen.Fil+2, -dimen.Fil+2, -dimen.Fil+2)}, 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := slices.Clone(tc.items)
			got, err := allocateWidths(tc.items, tc.target)
			want := ErrIllegalArguments
			if tc.capacity {
				want = errInsufficientCapacity
			}
			if got != nil || !errors.Is(err, want) || !reflect.DeepEqual(before, tc.items) {
				t.Fatal(got, err)
			}
		})
	}
}

// The numerical model is deliberately independent of glyph realization. An
// elastic textbox receives the same quota as glue, but this does not change W.
func TestAllocateWidthsKindIndependent(t *testing.T) {
	items := []KnotCore{allocationItem(10, 5, 20), allocationItem(20, 10, 40)}
	for _, kind := range []KnotType{KTGlue, KTTextBox, KTKern} {
		for i := range items {
			items[i].Kind = kind
			items[i].Flags = KFDiscardable
			items[i].Penalty = MaxPenalty
		}
		got, err := allocateWidths(items, 45)
		if err != nil || !slices.Equal(got, []dimen.DU{15, 30}) || items[0].W != 10 || items[1].W != 20 {
			t.Fatal(got, err)
		}
	}
}

func TestAllocateWidthsFiniteProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(37))
	for trial := 0; trial < 2000; trial++ {
		items := make([]KnotCore, 1+rng.Intn(12))
		var natural, min, max int64
		for i := range items {
			w := dimen.DU(rng.Intn(100))
			items[i] = allocationItem(w, dimen.DU(rng.Intn(int(w)+1)), w+dimen.DU(rng.Intn(100)))
			natural += int64(w)
			min += int64(items[i].MinW)
			max += int64(items[i].MaxW)
		}
		target := dimen.DU(min + rng.Int63n(max-min+1))
		got, err := allocateWidths(items, target)
		if err != nil {
			t.Fatal(trial, err)
		}
		checkWidthAllocation(t, items, target, got)
		delta, capacity := int64(target)-natural, max-natural
		if delta < 0 {
			delta, capacity = -delta, natural-min
		}
		for i, k := range items {
			weight, adjustment := int64(k.MaxW)-int64(k.W), int64(got[i])-int64(k.W)
			if int64(target) < natural {
				weight, adjustment = int64(k.W)-int64(k.MinW), -adjustment
			}
			if capacity == 0 {
				continue
			}
			floor := delta * weight / capacity
			if adjustment < floor || adjustment > floor+1 {
				t.Fatal("not a proportional quota", trial, i)
			}
		}
	}
}
