package knuthplass

import (
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

func scratchFixture() (*linebreaker, *khipu.Khipu, BreakRef) {
	kp := newLinebreaker(khipu.Params{LineShape: linebreak.RectangularParShape(30 * dimen.BP), RightSkip: glue(0, 0, 20*dimen.BP)}, nil)
	kp.root = PlainBreak(-1)
	kp.hyphenating = true
	kp.params.PreTolerance = 50
	from := PlainBreak(0)
	seed := seedBookkeepingFromKnot(khipu.KnotCore{Kind: khipu.KTTextBox, W: 10 * dimen.BP, MinW: 10 * dimen.BP, MaxW: 10 * dimen.BP})
	kp.paths[from] = map[lineNo]bookkeeping{1: seed, 2: seed}
	kp.seeds[origin{from, 1}], kp.seeds[origin{from, 2}] = seed, seed
	kp.horizon[from] = struct{}{}
	khp := newKhipu([]node{
		{w: 10, minw: 10, maxw: 10, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 1, minw: 1, maxw: 1, kind: khipu.KTGlue},
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	for _, variant := range []uint16{1, 2} {
		khp.AddDiscretionaryCandidate(2, khipu.DiscretionaryCandidate{
			Variant:   variant,
			PreBreak:  khipu.KnotCore{Kind: khipu.KTTextBox, W: 10 * dimen.BP, MinW: 10 * dimen.BP, MaxW: 10 * dimen.BP},
			PostBreak: khipu.KnotCore{Kind: khipu.KTTextBox, W: 5 * dimen.BP, MinW: 5 * dimen.BP, MaxW: 5 * dimen.BP},
		})
	}
	return kp, khp, from
}

func TestEvaluationScratchReuseAndGrowth(t *testing.T) {
	kp, _, from := scratchFixture()
	first, _ := kp.evaluateCandidates(from, 50)
	base := &first[0]
	delete(kp.paths[from], 2)
	got, _ := kp.evaluateCandidates(from, 0)
	if len(got) != 1 || &got[0] != base || got[0].demerits != calcDemerits(got[0].badness, 0, kp.params) {
		t.Fatalf("buffer not reused or stale results returned: %v", got)
	}
	capacity := cap(got)
	if empty, reachable := kp.evaluateCandidates(PlainBreak(99), 0); len(empty) != 0 || reachable || cap(kp.evaluationScratch) != capacity {
		t.Fatal("empty evaluation lost capacity or returned stale results")
	}
	for line := 1; line <= capacity+1; line++ {
		kp.paths[from][lineNo(line)] = kp.paths[from][1]
	}
	large, _ := kp.evaluateCandidates(from, 0)
	if len(large) != capacity+1 || cap(large) <= capacity {
		t.Fatal("buffer did not grow for a larger bucket")
	}
	base = &large[0]
	clear(kp.paths[from])
	kp.paths[from][1] = bookkeeping{}
	small, _ := kp.evaluateCandidates(from, 0)
	if len(small) != 1 || &small[0] != base {
		t.Fatal("grown buffer was not reused")
	}
	other, _, otherFrom := scratchFixture()
	independent, _ := other.evaluateCandidates(otherFrom, 0)
	if &independent[0] == base {
		t.Fatal("breakers share scratch storage")
	}
}

func TestPass2ScratchBuffersStayIndependent(t *testing.T) {
	kp, khp, from := scratchFixture()
	horizon := kp.snapshotHorizon()
	wantHorizon := slices.Clone(horizon)
	evals, _ := kp.evaluateCandidates(from, 0)
	wantEvals := slices.Clone(evals)
	var first *breakCandidate
	for _, ev := range evals {
		if !kp.wantsHyphenation(ev) {
			t.Fatal("fixture must exercise nested pass-2 evaluation")
		}
		cands := kp.evaluateDiscretionaryCandidates(khp, from, 1, 2)
		if len(cands) != 4 {
			t.Fatalf("got %d candidates", len(cands))
		}
		if first == nil {
			first = &cands[0]
		} else if &cands[0] != first {
			t.Fatal("discretionary buffer not reused")
		}
		for _, cand := range cands {
			c := cand.lineCost()
			kp.evalNewSegmentSeed(from, cand.target, c.line+1, c.cost, cand.seed)
			kp.horizon[cand.target] = struct{}{}
		}
		if !slices.Equal(evals, wantEvals) || !slices.Equal(horizon, wantHorizon) {
			t.Fatal("nested evaluation or path insertion overwrote outstanding results")
		}
	}
	capacity := cap(kp.discretionaryScratch)
	for _, candidate := range []kinx{-1, 0, 1, 99} {
		if got := kp.evaluateDiscretionaryCandidates(khp, from, 1, candidate); len(got) != 0 || len(kp.discretionaryScratch) != 0 || cap(kp.discretionaryScratch) != capacity {
			t.Fatal("invalid or non-text candidate retained results or lost capacity")
		}
	}
	delete(kp.paths[from], 2)
	got := kp.evaluateDiscretionaryCandidates(khp, from, 1, 2)
	if len(got) != 2 || &got[0] != first {
		t.Fatal("smaller discretionary result did not reuse storage")
	}
	// Ordinary rescoring must likewise leave a live discretionary result intact.
	want := slices.Clone(got)
	kp.evaluateCandidates(from, 50)
	if !slices.Equal(got, want) {
		t.Fatal("ordinary scoring overwrote discretionary results")
	}
	template := khp.DiscretionariesAt(2)[0]
	for variant := 3; variant <= capacity+1; variant++ {
		template.Variant = uint16(variant)
		khp.AddDiscretionaryCandidate(2, template)
	}
	got = kp.evaluateDiscretionaryCandidates(khp, from, 1, 2)
	if len(got) != capacity+1 || cap(got) <= capacity {
		t.Fatal("discretionary buffer did not grow for additional variants")
	}
	clear(kp.paths[from])
	if got := kp.evaluateDiscretionaryCandidates(khp, from, 1, 2); len(got) != 0 {
		t.Fatal("empty bucket returned stale discretionaries")
	}
}

func TestHorizonScratchSnapshotAndReuse(t *testing.T) {
	kp, _, from := scratchFixture()
	variant := BreakRef{At: 0, Variant: 1}
	kp.horizon[variant] = struct{}{}
	kp.horizon[kp.root] = struct{}{}
	got := kp.snapshotHorizon()
	if !slices.Equal(got, []BreakRef{kp.root, from, variant}) {
		t.Fatalf("unsorted snapshot: %v", got)
	}
	base := &got[0]
	clear(kp.horizon)
	kp.horizon[PlainBreak(9)] = struct{}{}
	if !slices.Equal(got, []BreakRef{kp.root, from, variant}) {
		t.Fatal("live horizon mutations changed snapshot")
	}
	got = kp.snapshotHorizon()
	if len(got) != 1 || got[0] != PlainBreak(9) || &got[0] != base {
		t.Fatal("smaller snapshot lost storage or retained stale entries")
	}
	capacity := cap(got)
	clear(kp.horizon)
	if got := kp.snapshotHorizon(); len(got) != 0 || cap(got) != capacity {
		t.Fatal("empty horizon lost capacity")
	}
	for i := 0; i <= capacity; i++ {
		kp.horizon[PlainBreak(i)] = struct{}{}
	}
	if got := kp.snapshotHorizon(); len(got) != capacity+1 || !slices.IsSortedFunc(got, compareBreakRef) {
		t.Fatal("growing snapshot is incomplete or unsorted")
	}
}

func TestScratchSteadyStateAllocations(t *testing.T) {
	kp, khp, from := scratchFixture()
	// AllocsPerRun performs a warmup before measuring. These checks concern
	// only local scoring/snapshots, not path insertion, shaping or diagnostics.
	for name, run := range map[string]func(){
		"ordinary":      func() { kp.evaluateCandidates(from, 0) },
		"discretionary": func() { kp.evaluateDiscretionaryCandidates(khp, from, 1, 2) },
		"horizon":       func() { kp.snapshotHorizon() },
	} {
		t.Run(name, func(t *testing.T) {
			if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
				t.Fatalf("got %g allocations after warmup, want zero", allocs)
			}
		})
	}
}
