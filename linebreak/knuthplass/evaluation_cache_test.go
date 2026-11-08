package knuthplass

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

func TestBoundaryEvaluationsScopeAndScratch(t *testing.T) {
	kp, _, from := scratchFixture()
	enableWorkMetrics(kp)
	kp.boundaryEvals.begin()
	want := slices.Clone(kp.evaluateCandidates(from, 0))
	kp.evaluateCandidates(PlainBreak(99), 0) // reuse ordinary scratch
	got := kp.evaluateCandidates(from, 0)
	if !slices.Equal(want, got) || kp.metrics.ordinary != 2 || kp.metrics.ordinaryReused != 2 {
		t.Fatal("cache did not survive scratch reuse", kp.metrics)
	}
	// A changed penalty cannot reuse ranking values from the same geometry.
	kp.evaluateCandidates(from, 50)
	if kp.metrics.ordinary != 4 {
		t.Fatal("reused a different penalty")
	}
	kp.boundaryEvals.end()
	kp.evaluateCandidates(from, 50)
	if kp.metrics.ordinary != 6 {
		t.Fatal("cache escaped its scope")
	}
	kp.boundaryEvals.begin()
	kp.evaluateCandidates(from, 50)
	if kp.metrics.ordinary != 8 {
		t.Fatal("next boundary reused previous-knot evaluations")
	}
	kp.boundaryEvals.end()
	if allocs := testing.AllocsPerRun(100, func() {
		kp.boundaryEvals.begin()
		kp.evaluateCandidates(from, 0)
		kp.evaluateCandidates(from, 0)
		kp.boundaryEvals.end()
	}); allocs != 0 {
		t.Fatalf("warm boundary storage allocated %g times", allocs)
	}
}

func TestBoundaryEvaluationsInvalidateWorkingWrites(t *testing.T) {
	kp, _ := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}, nil)
	enableWorkMetrics(kp)
	seed := func(w dimen.DU) bookkeeping {
		return seedBookkeepingFromKnot(khipu.KnotCore{Kind: khipu.KTTextBox, W: w, MinW: w, MaxW: w})
	}
	from, target := PlainBreak(0), BreakRef{At: 2, Variant: 1}
	kp.evalNewSegment(kp.root, from, 1, cost{})
	kp.evalNewSegmentSeed(kp.root, target, 1, cost{demerits: 10}, seed(150*dimen.BP))
	kp.boundaryEvals.begin()
	if kp.evaluateCandidates(target, 0)[0].accepted() {
		t.Fatal("expected initially overfull seed")
	}
	// Losing proposals leave both bookkeeping and cached classification valid.
	kp.evalNewSegmentSeed(kp.root, target, 1, cost{demerits: 20}, seed(100*dimen.BP))
	kp.evaluateCandidates(target, 0)
	if kp.metrics.ordinary != 1 || kp.metrics.ordinaryReused != 1 {
		t.Fatal("losing proposal invalidated cache")
	}
	// A cheaper predecessor can supply different segment geometry for the same ID.
	kp.evalNewSegmentSeed(kp.root, target, 1, cost{demerits: 5}, seed(100*dimen.BP))
	if !kp.evaluateCandidates(target, 0)[0].accepted() || kp.metrics.ordinary != 2 {
		t.Fatal("replacement retained stale overfull classification")
	}
	// New line counts invalidate the whole bucket, not just a known state.
	kp.evalNewSegmentSeed(from, target, 2, cost{}, seed(100*dimen.BP))
	if got := kp.evaluateCandidates(target, 0); len(got) != 2 || kp.metrics.ordinary != 4 {
		t.Fatal("new sibling missing", got)
	}
	kp.updatePath(target, 3, khipu.KnotCore{Kind: khipu.KTTextBox, W: 1, MinW: 1, MaxW: 1})
	for _, ev := range kp.evaluateCandidates(target, 0) {
		if ev.accepted() {
			t.Fatal("width update retained exact-fit classification")
		}
	}
	kp.retireState(origin{target, 2})
	if got := kp.evaluateCandidates(target, 0); len(got) != 1 || got[0].line != 1 || kp.metrics.ordinary != 7 {
		t.Fatal("retired sibling retained in evaluation", got)
	}
	kp.retireState(origin{target, 1})
	if got := kp.evaluateCandidates(target, 0); len(got) != 0 {
		t.Fatal("deleted bucket returned old evaluations")
	}
	kp.boundaryEvals.end()
}

func TestBoundaryEvaluationsCommitReplacedAndNewSuccessors(t *testing.T) {
	for _, initiallyPresent := range []bool{false, true} {
		kp, _ := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}, nil)
		kp.hyphenating, kp.final, kp.prune, kp.end = true, true, true, PlainBreak(5)
		kp.updatePath(kp.root, 0, khipu.KnotCore{Kind: khipu.KTTextBox, W: 150 * dimen.BP, MinW: 150 * dimen.BP, MaxW: 150 * dimen.BP})
		target := BreakRef{At: 2, Variant: 1}
		seed := seedBookkeepingFromKnot(khipu.KnotCore{Kind: khipu.KTTextBox, W: 150 * dimen.BP, MinW: 150 * dimen.BP, MaxW: 150 * dimen.BP})
		if initiallyPresent {
			kp.evalNewSegmentSeed(kp.root, target, 1, cost{demerits: 10}, seed)
			kp.horizon[target] = struct{}{}
		}
		kp.boundaryEvals.begin()
		kp.evaluateCandidates(target, 0) // caches an overfull bucket or an empty one
		seed.segment = WSS{W: 100 * dimen.BP, Min: 100 * dimen.BP, Max: 100 * dimen.BP}
		kp.activateDiscretionary(kp.root, breakCandidate{
			eval: lineEvaluation{line: 0, demerits: 5}, target: target, seed: seed, initial: seed,
		})
		if !kp.finishBoundary(5, InfinityMerits) {
			t.Fatal("forced boundary failed to score discovered successor")
		}
		pred, ok := kp.Pred(kp.end, 2)
		if !ok || pred.from != target || kp.paths[kp.end][2].worstbadness != 0 {
			t.Fatal("stale classification caused fallback or lost successor", pred)
		}
		kp.boundaryEvals.end()
	}
}

func TestBoundaryEvaluationsEndOnProviderError(t *testing.T) {
	want := errors.New("provider failed")
	params := NewKPDefaultParameters()
	params.DiscretionaryProvider = failedRetirementProvider{want}
	kp, _ := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}, params)
	kp.hyphenating, kp.final = true, true
	khp := newKhipu([]node{
		{w: 150, minw: 150, maxw: 150, p: InfinityDemerits, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	if err := kp.constructBreakpointGraph(khp); !errors.Is(err, want) || kp.boundaryEvals.active {
		t.Fatal("error escaped active boundary scope", err)
	}
}

func TestBoundaryEvaluationReuseMatchesUncached(t *testing.T) {
	for _, shape := range []khipu.LineShape{
		linebreak.RectangularParShape(150 * dimen.PT), alternatingIndexShape{},
		linebreak.RectangularParShape(5 * dimen.PT), // final fallback, not just feasible paths
	} {
		for _, forced := range []bool{false, true} {
			for _, mode := range []string{"first", "empty", "productive"} {
				var breakers [2]*linebreaker
				var paragraphs [2]*khipu.Khipu
				for i := range breakers {
					config := khipu.Config{Hyphenator: &decliningHyphenator{}}
					if mode == "productive" {
						config.Hyphenator = &loremHyphenator{}
					}
					owner := khipu.NewKhipukamayuq(config)
					khp, err := owner.EncodeParagraph(t.Context(), loremParagraph(t, 50))
					if err != nil {
						t.Fatal(err)
					}
					if forced {
						for at := 17; at < len(khp.Kind)-1; at += 18 {
							khp.Penalty[at] = InfinityMerits
						}
					}
					params := NewKPDefaultParameters()
					params.DiscretionaryProvider = owner
					kp, err := prepareLineBreaker(DefaultLayout(shape), params)
					if err != nil {
						t.Fatal(err)
					}
					kp.hyphenating, kp.final = mode != "first", mode != "first"
					kp.boundaryEvals.disabled = i == 0
					kp.end = PlainBreak(len(khp.Kind) - 1)
					kp.prepareRetirement(khp)
					enableWorkMetrics(kp)
					breakers[i], paragraphs[i] = kp, khp
				}
				ref, cached := breakers[0], breakers[1]
				for at := range paragraphs[0].Kind {
					live, err := ref.processKnot(paragraphs[0], at)
					liveCached, errCached := cached.processKnot(paragraphs[1], at)
					if err != nil || errCached != nil || live != liveCached || cached.boundaryEvals.active ||
						!reflect.DeepEqual(ref.paths, cached.paths) || !reflect.DeepEqual(ref.preds, cached.preds) ||
						!reflect.DeepEqual(ref.seeds, cached.seeds) || !reflect.DeepEqual(ref.retired, cached.retired) || !reflect.DeepEqual(ref.horizon, cached.horizon) {
						t.Fatalf("mode=%s shape=%T forced=%v knot=%d: cached/uncached state differs (%v/%v)", mode, shape, forced, at, err, errCached)
					}
					if !live {
						break
					}
				}
				want, q, ok := ref.collectOptimalBreakRefs(ref.end)
				got, gq, gok := cached.collectOptimalBreakRefs(cached.end)
				if !slices.Equal(want, got) || q != gq || ok != gok ||
					ref.metrics.ordinary != cached.metrics.ordinary+cached.metrics.ordinaryReused ||
					ref.metrics.discretionary != cached.metrics.discretionary || ref.metrics.peak != cached.metrics.peak {
					t.Fatal("changed result or work beyond ordinary reuse", ref.metrics, cached.metrics)
				}
				if mode == "first" && cached.boundaryEvals.index != nil {
					t.Fatal("first pass allocated an unused boundary cache")
				}
			}
		}
	}
}
