package knuthplass

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/uax/bidi"
)

func TestZeroStretchShortOriginsStayActive(t *testing.T) {
	text := styled.TextFromString("aaa bbb ccc ddd")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	khp, err := khipu.NewKhipukamayuq(khipu.Config{}).EncodeParagraph(t.Context(), para)
	if err != nil {
		t.Fatal(err)
	}
	layout := khipu.Params{LineShape: linebreak.RectangularParShape(200 * dimen.PT)}
	breaks, _, ok, err := breakParagraphPass(khp, layout, nil, passConfig{id: passPreTolerance})
	if err != nil || !ok || !slices.Equal(breaks, []int{7}) {
		t.Fatalf("breaks=%v ok=%v err=%v", breaks, ok, err)
	}
}

func TestForcedBoundaryRescuesZeroShrinkWord(t *testing.T) {
	for _, internal := range []bool{false, true} {
		provider := &testDiscretionaryProvider{want: []khipu.DiscretionaryCandidate{{
			Variant:   1,
			PreBreak:  khipu.KnotCore{Kind: khipu.KTTextBox, W: 100 * dimen.BP, MinW: 100 * dimen.BP, MaxW: 100 * dimen.BP},
			PostBreak: khipu.KnotCore{Kind: khipu.KTTextBox, W: 50 * dimen.BP, MinW: 50 * dimen.BP, MaxW: 50 * dimen.BP},
		}}}
		params := NewKPDefaultParameters()
		params.DiscretionaryProvider = provider
		nodes := []node{
			{w: 150, minw: 150, maxw: 150, p: InfinityDemerits, kind: khipu.KTTextBox},
			{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
		}
		want := []int{0, 1}
		if internal {
			nodes = append(nodes, node{w: 100, minw: 100, maxw: 100, p: InfinityDemerits, kind: khipu.KTTextBox}, nodes[1])
			want = append(want, 3)
		}
		khp := newKhipu(nodes)
		khp.SetKnotFlags(1, 0) // retain the internal boundary's fill, as at paragraph end
		layout := khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}
		if breaks, _, ok, err := breakParagraphPass(khp, layout, params, passConfig{id: passPreTolerance}); err != nil || ok || len(breaks) != 0 || provider.calls != 0 {
			t.Fatalf("first pass must fail normally without discovery: %v %v %v", breaks, ok, err)
		}
		breaks, err := BreakParagraph(khp, layout, params)
		if err != nil || !slices.Equal(breaks, want) || len(khp.SelectedDiscretionaries) != 1 || khp.SelectedDiscretionaries[0].Variant != 1 {
			t.Fatalf("internal=%v breaks=%v selected=%v err=%v", internal, breaks, khp.SelectedDiscretionaries, err)
		}
		if provider.calls == 0 || provider.requestedAt[0] != 0 {
			t.Fatal("terminal marker prevented last-box discovery")
		}
	}
}

func TestFallbackIsFinalPassOnly(t *testing.T) {
	for _, width := range []dimen.DU{10, 150} {
		// A forced short line without stretch and an unbreakable overlong
		// line both need fallback, not unconditional forced acceptance.
		khp := newKhipu([]node{
			{w: width, minw: width, maxw: width, p: InfinityMerits, kind: khipu.KTTextBox},
			{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
		})
		layout := khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}
		if breaks, _, ok, err := breakParagraphPass(khp, layout, nil, passConfig{id: passPreTolerance}); err != nil || ok || len(breaks) != 0 {
			t.Fatalf("first pass: breaks=%v ok=%v err=%v", breaks, ok, err)
		}
		breaks, quality, ok, err := breakParagraphPass(khp, layout, nil, passConfig{id: passTolerance, hyphenating: true, final: true})
		if err != nil || !ok || !slices.Equal(breaks, []int{0, 1}) || quality.totalCost < AwfulDemerits || quality.worstBadness != 10000 {
			t.Fatalf("final pass: breaks=%v quality=%+v ok=%v err=%v", breaks, quality, ok, err)
		}
	}
}

func TestDrainingOrdinaryBoundaryFallsBackBeforeDeletingSource(t *testing.T) {
	khp := newKhipu([]node{
		{w: 150, minw: 150, maxw: 150, p: InfinityDemerits, kind: khipu.KTTextBox},
		{kind: khipu.KTGlue},
		{w: 100, minw: 100, maxw: 100, p: InfinityDemerits, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	layout := khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}
	if _, _, ok, err := breakParagraphPass(khp, layout, nil, passConfig{id: passPreTolerance}); err != nil || ok {
		t.Fatalf("first pass should exhaust normally: %v %v", ok, err)
	}
	breaks, quality, ok, err := breakParagraphPass(khp, layout, nil, passConfig{id: passTolerance, hyphenating: true, final: true})
	if err != nil || !ok || !slices.Equal(breaks, []int{1, 3}) || quality.totalCost < AwfulDemerits {
		t.Fatalf("fallback did not survive source deletion: %v %+v %v %v", breaks, quality, ok, err)
	}
}

func TestRetroactiveDiscretionaryKeepsOnlyFragmentInRescanSeed(t *testing.T) {
	provider := &testDiscretionaryProvider{want: []khipu.DiscretionaryCandidate{{
		Variant:   1,
		PreBreak:  khipu.KnotCore{Kind: khipu.KTTextBox, W: 29 * dimen.BP, MinW: 29 * dimen.BP, MaxW: 29 * dimen.BP},
		PostBreak: khipu.KnotCore{Kind: khipu.KTTextBox, W: 20 * dimen.BP, MinW: 20 * dimen.BP, MaxW: 20 * dimen.BP},
	}}}
	params := NewKPDefaultParameters()
	params.DiscretionaryProvider = provider
	khp := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 1, minw: 1, maxw: 1, p: InfinityDemerits, kind: khipu.KTGlue},
		{w: 60, minw: 60, maxw: 60, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 5, minw: 5, maxw: 5, kind: khipu.KTGlue},
		{w: 25, minw: 25, maxw: 25, p: InfinityDemerits, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	pre := khipu.KnotCore{Kind: khipu.KTTextBox, W: 25 * dimen.BP, MinW: 25 * dimen.BP, MaxW: 25 * dimen.BP}
	khp.AddDiscretionaryCandidate(4, khipu.DiscretionaryCandidate{Variant: 1, PreBreak: pre, PostBreak: pre})
	kp, _ := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(50 * dimen.BP)}, params)
	kp.hyphenating, kp.final, kp.end = true, true, PlainBreak(5)
	kp.prepareRetirement(khp)
	for i := 0; i <= 3; i++ {
		if live, err := kp.processKnot(khp, i); !live || err != nil {
			t.Fatalf("knot %d: %v %v", i, live, err)
		}
	}
	from := BreakRef{At: 2, Variant: 1}
	if kp.paths[from][1].segment.W != 25*dimen.BP || kp.seeds[origin{from, 1}].segment.W != 20*dimen.BP {
		t.Fatal("consumed trailing glue was lost or included in immutable seed")
	}
	if live, err := kp.processKnot(khp, 4); !live || err != nil {
		t.Fatalf("rescan failed: %v %v", live, err)
	}
	if pred, ok := kp.Pred(BreakRef{At: 4, Variant: 1}, 2); !ok || pred.from != from {
		t.Fatal("rescan duplicated the consumed suffix and lost an exact-fit successor")
	}
}

type retirementShape func(int32) dimen.DU

func (f retirementShape) LineLength(line int32) dimen.DU { return f(line) }

func TestRetirementIsPerLineAndPreservesHistory(t *testing.T) {
	shape := retirementShape(func(line int32) dimen.DU { return dimen.DU(line) * 100 })
	kp, err := prepareLineBreaker(khipu.Params{LineShape: shape, RightSkip: khipu.KnotCore{MaxW: 50}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	kp.prune = true
	for _, line := range []lineNo{0, 1} {
		kp.paths[kp.root][line] = bookkeeping{segment: WSS{W: 150, Min: 150, Max: 150}, seenContent: true}
		kp.seeds[origin{kp.root, line}] = bookkeeping{}
	}
	if !kp.finishBoundary(1, 0) {
		t.Fatal("live sibling must survive")
	}
	if _, exists := kp.paths[kp.root][0]; exists || len(kp.paths[kp.root]) != 1 {
		t.Fatal("retirement removed the wrong line states")
	}
	if _, exists := kp.seeds[origin{kp.root, 0}]; exists {
		t.Fatal("dead seed retained")
	}
	if _, exists := kp.horizon[kp.root]; !exists {
		t.Fatal("live sibling lost its horizon membership")
	}
	if pred, exists := kp.Pred(PlainBreak(1), 2); !exists || pred.from != kp.root {
		t.Fatal("surviving sibling failed to create a successor")
	}
	kp.retireState(origin{kp.root, 1})
	if _, exists := kp.Pred(PlainBreak(1), 2); !exists {
		t.Fatal("retirement deleted descendant history")
	}
}

func TestRetiredTargetCannotBeResurrected(t *testing.T) {
	kp, _ := prepareLineBreaker(DefaultLayout(linebreak.RectangularParShape(100)), nil)
	target := BreakRef{At: 2, Variant: 1}
	kp.evalNewSegment(kp.root, target, 1, cost{demerits: 20})
	pred, _ := kp.Pred(target, 1)
	kp.retireState(origin{target, 1})
	kp.activateDiscretionary(kp.root, breakCandidate{
		target: target, eval: lineEvaluation{line: 0, demerits: 1},
	})
	if _, exists := kp.horizon[target]; exists || len(kp.paths[target]) != 0 {
		t.Fatal("rejected insertion revived a retired target")
	}
	if got, _ := kp.Pred(target, 1); got != pred {
		t.Fatal("rejected cheaper transition rewrote historical cost")
	}
}

func TestNegativeWidthsDisablePruningIncludingTrailer(t *testing.T) {
	for _, trailer := range []bool{false, true} {
		nodes := []node{
			{w: 120, minw: 120, maxw: 120, kind: khipu.KTTextBox},
			{w: -30, minw: -30, maxw: -30, p: InfinityDemerits, kind: khipu.KTKern},
			{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
		}
		if trailer {
			nodes = []node{nodes[0], {w: -30, minw: -30, maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue}}
		}
		khp := newKhipu(nodes)
		khp.SetKnotFlags(1, 0) // retained negative width, not a trailing trim
		kp, _ := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}, nil)
		if err := kp.constructBreakpointGraph(khp); err != nil {
			t.Fatal(err)
		}
		breaks, _, ok := kp.collectOptimalBreakpoints(kp.end)
		if kp.prune || !ok || !slices.Equal(breaks, []int{len(nodes) - 1}) {
			t.Fatalf("negative width could not rescue origin: prune=%v breaks=%v ok=%v", kp.prune, breaks, ok)
		}
	}
	// Avoid claiming monotonicity after saturating raw minimum accumulation.
	kp, _ := prepareLineBreaker(DefaultLayout(linebreak.RectangularParShape(100)), nil)
	khp := &khipu.Khipu{MinW: []dimen.DU{math.MaxInt32, 1}}
	kp.prepareRetirement(khp)
	if kp.prune {
		t.Fatal("minimum-sum overflow must disable pruning")
	}
}

func TestCachedNegativeFragmentsOptOutAndLazyNegativeFragmentsFail(t *testing.T) {
	negative := khipu.DiscretionaryCandidate{Variant: 1, PreBreak: khipu.KnotCore{MinW: -1}}
	khp := newKhipu([]node{{w: 150, minw: 150, maxw: 150, p: InfinityDemerits, kind: khipu.KTTextBox}, {maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue}})
	khp.AddDiscretionaryCandidate(0, negative)
	layout := khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}
	kp, _ := prepareLineBreaker(layout, nil)
	kp.hyphenating = true
	kp.prepareRetirement(khp)
	if kp.prune {
		t.Fatal("cached negative fragment must disable pruning before scanning")
	}
	clear(khp.DiscretionaryCandidates)
	params := NewKPDefaultParameters()
	params.DiscretionaryProvider = &testDiscretionaryProvider{want: []khipu.DiscretionaryCandidate{negative}}
	if _, err := BreakParagraph(khp, layout, params); !errors.Is(err, khipu.ErrIllegalArguments) {
		t.Fatalf("lazy negative fragment must fail rather than invalidate retirement: %v", err)
	}
	params.DiscretionaryProvider = &testDiscretionaryProvider{want: []khipu.DiscretionaryCandidate{{
		Variant: 1, PreBreak: khipu.KnotCore{W: math.MaxInt32, MinW: math.MaxInt32, MaxW: math.MaxInt32},
	}}}
	if _, err := BreakParagraph(khp, layout, params); !errors.Is(err, khipu.ErrIllegalArguments) {
		t.Fatalf("lazy fragments must obey the accumulation safety bound: %v", err)
	}
}

type failedRetirementProvider struct{ err error }

func (p failedRetirementProvider) DiscretionaryCandidates(*khipu.Khipu, int) ([]khipu.DiscretionaryCandidate, error) {
	return nil, p.err
}

func TestProviderErrorIsNotNormalPassFailure(t *testing.T) {
	want := errors.New("hyphenator unavailable")
	params := NewKPDefaultParameters()
	params.DiscretionaryProvider = failedRetirementProvider{want}
	khp := newKhipu([]node{
		{w: 150, minw: 150, maxw: 150, p: InfinityDemerits, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	if _, err := BreakParagraph(khp, khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}, params); !errors.Is(err, want) {
		t.Fatalf("provider error must propagate instead of becoming fallback: %v", err)
	}
}

func TestForcedFallbackWhenPruningIsDisabled(t *testing.T) {
	khp := newKhipu([]node{
		{w: 150, minw: 150, maxw: 150, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: -1, minw: -1, maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	kp, _ := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(100 * dimen.BP)}, nil)
	kp.final = true
	if err := kp.constructBreakpointGraph(khp); err != nil {
		t.Fatal(err)
	}
	breaks, q, ok := kp.collectOptimalBreakpoints(kp.end)
	if kp.prune || !ok || !slices.Equal(breaks, []int{1}) || q.totalCost != AwfulDemerits {
		t.Fatalf("fallback must not require actual deletion: %v %+v %v", breaks, q, ok)
	}
}

func TestFallbackCanonicalCostIncludesTerminalHyphenExtra(t *testing.T) {
	for _, extra := range []merits{0, 91} {
		for run := range 20 {
			params := NewKPDefaultParameters()
			params.FinalHyphenDemerits = extra
			kp, _ := prepareLineBreaker(khipu.Params{LineShape: linebreak.RectangularParShape(100)}, params)
			kp.final, kp.prune, kp.end = true, true, PlainBreak(9)
			kp.retireState(origin{kp.root, 0})
			refs := []BreakRef{{At: 1, Variant: 1}, PlainBreak(2)}
			if run%2 != 0 {
				slices.Reverse(refs)
			}
			for _, ref := range refs {
				kp.horizon[ref] = struct{}{}
				kp.paths[ref] = map[lineNo]bookkeeping{1: {segment: WSS{W: 200, Min: 200, Max: 200}, seenContent: true}}
				kp.seeds[origin{ref, 1}] = bookkeeping{}
				kp.SetPred(kp.root, ref, 0, 0, 1)
			}
			if !kp.finishBoundary(kp.end.At, InfinityMerits) {
				t.Fatal("final fallback failed")
			}
			want := BreakRef{At: 1, Variant: 1}
			if extra != 0 {
				want = PlainBreak(2)
			}
			pred, _ := kp.Pred(kp.end, 2)
			breaks, _, ok := kp.collectOptimalBreakRefs(kp.end)
			if pred.from != want || pred.total != AwfulDemerits || !ok || !slices.Equal(breaks, []BreakRef{want, kp.end}) {
				t.Fatalf("extra=%d pred=%+v breaks=%v ok=%v", extra, pred, breaks, ok)
			}
		}
	}
}

func TestRetirementMatchesUnprunedScoring(t *testing.T) {
	for _, hyphenating := range []bool{false, true} {
		for _, shape := range []khipu.LineShape{linebreak.RectangularParShape(150 * dimen.PT), alternatingIndexShape{}} {
			var want []BreakRef
			var quality pathQuality
			var unprunedPeak int
			for _, prune := range []bool{false, true} {
				khp, kp, _, _ := hyphenatingLorem(t, 100)
				kp.hyphenating, kp.final, kp.layout.LineShape = hyphenating, hyphenating, shape
				kp.end = PlainBreak(len(khp.Kind) - 1)
				kp.prepareRetirement(khp)
				kp.prune = prune
				peak := 0
				for i := range khp.Kind {
					live, err := kp.processKnot(khp, i)
					if err != nil || !live {
						t.Fatalf("prune=%v hyphenating=%v knot=%d live=%v err=%v", prune, hyphenating, i, live, err)
					}
					active := 0
					for ref, lines := range kp.paths {
						active += len(lines)
						for line, path := range lines {
							if prune && khp.Penalty[i] < InfinityDemerits && ref.At < i && path.effectiveWidth(kp.layout).Min > shape.LineLength(int32(line+1)) {
								t.Fatalf("dead mature state retained at knot %d: %v/%d", i, ref, line)
							}
						}
					}
					peak = max(peak, active)
				}
				breaks, q, ok := kp.collectOptimalBreakRefs(kp.end)
				if !ok {
					t.Fatal("no complete path")
				}
				if !prune {
					want, quality, unprunedPeak = breaks, q, peak
				} else if !slices.Equal(breaks, want) || q != quality || peak >= unprunedPeak {
					t.Fatalf("hyphenating=%v shape=%T: breaks=%v want=%v quality=%+v want=%+v peaks=%d/%d", hyphenating, shape, breaks, want, q, quality, peak, unprunedPeak)
				}
			}
		}
	}
}
