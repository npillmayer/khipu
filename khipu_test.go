package khipu

import (
	"fmt"
	"testing"

	"github.com/npillmayer/khipu/dimen"
)

func TestKhipuInitialCapacity(t *testing.T) {
	owner := NewKhipukamayuq(Config{})
	for _, requested := range []int{0, 3, 2048} {
		t.Run(fmt.Sprint(requested), func(t *testing.T) {
			khp := owner.allocKhipu(requested)
			want := requested
			if want == 0 {
				want = DefaultInitialKhipuCapa
			}
			if khp.owner != owner || khp.DiscretionaryCandidates == nil || khp.SelectedDiscretionaries == nil {
				t.Fatal("allocation did not initialize ownership and discretionary storage")
			}
			capacities := []int{cap(khp.W), cap(khp.MinW), cap(khp.MaxW), cap(khp.Penalty), cap(khp.Pos), cap(khp.Len), cap(khp.Kind), cap(khp.Flags)}
			for _, got := range capacities {
				if got != want {
					t.Fatalf("initial capacities %v, want %d", capacities, want)
				}
			}
			knot := KnotCore{Kind: KTTextBox, W: dimen.PT, MinW: dimen.PT, MaxW: dimen.PT}
			for range want + 1 {
				khp.appendKnot(knot)
			}
			for i := range want + 1 {
				if got := khp.KnotByIndex(i); got != knot {
					t.Fatalf("growing planes lost knot %d: %+v", i, got)
				}
			}
		})
	}
}

// Replaces the legacy list-construction test with the SOA value contract.
func TestKhipuKnotSnapshots(t *testing.T) {
	owner := NewKhipukamayuq(Config{})
	khp := owner.allocKhipu(3)
	want := []KnotCore{
		{Kind: KTKern, W: dimen.PT, MinW: dimen.PT, MaxW: dimen.PT},
		{Kind: KTGlue, W: 2 * dimen.PT, MaxW: 4 * dimen.PT, Flags: KFDiscardable},
		{Kind: KTTextBox, Pos: 10, Len: 5, W: 25 * dimen.PT, MinW: 25 * dimen.PT, MaxW: 25 * dimen.PT, Penalty: MaxPenalty},
	}
	for _, k := range want {
		khp.appendKnot(k)
	}
	if len(khp.Kind) != len(want) {
		t.Fatalf("got %d knots, want %d", len(khp.Kind), len(want))
	}
	for i, k := range want {
		if got := khp.KnotByIndex(i); got != k {
			t.Fatalf("knot %d: got %+v, want %+v", i, got, k)
		}
	}
	snapshot := khp.KnotByIndex(2)
	snapshot.W = 0
	if khp.W[2] != want[2].W {
		t.Fatal("changing a snapshot mutated stored width")
	}
	for _, index := range []int{-1, len(want)} {
		if got := khp.KnotByIndex(index); got != (KnotCore{}) {
			t.Fatalf("out-of-range knot %d: %+v", index, got)
		}
	}
}
