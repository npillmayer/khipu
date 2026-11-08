package knuthplass

import (
	"maps"
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

type alternatingIndexShape struct{}

func (alternatingIndexShape) LineLength(line int32) dimen.DU {
	if line%2 == 0 {
		return 100 * dimen.PT
	}
	return 150 * dimen.PT
}

func TestIndexedCorpusBaseline(t *testing.T) {
	// Breaks and quality were captured with the original flat map. Retirement
	// and forced-boundary screening now remove dead work, not reconstruction.
	for _, tc := range []struct {
		shape   khipu.LineShape
		breaks  []kinx
		quality pathQuality
	}{
		{linebreak.RectangularParShape(150 * dimen.PT), []kinx{9, 13, 23, 33, 41, 49, 57, 65, 75, 85, 91, 100}, pathQuality{-2770, 43}},
		{alternatingIndexShape{}, []kinx{7, 11, 21, 25, 35, 41, 49, 55, 63, 71, 81, 87, 97, 100}, pathQuality{-5930, 47}},
	} {
		khp, err := khipu.NewKhipukamayuq(khipu.Config{}).EncodeParagraph(t.Context(), loremParagraph(t, 50))
		if err != nil {
			t.Fatal(err)
		}
		kp, err := prepareLineBreaker(DefaultLayout(tc.shape), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := kp.constructBreakpointGraph(khp); err != nil {
			t.Fatal(err)
		}
		breaks, quality, ok := kp.collectOptimalBreakRefs(kp.end)
		if !ok || !slices.Equal(projectBreakRefs(breaks), tc.breaks) || quality != tc.quality {
			t.Fatalf("shape %T: breaks=%v quality=%+v ok=%v preds=%d", tc.shape, breaks, quality, ok, len(kp.preds))
		}
		retained := 0
		for _, lines := range kp.paths {
			retained += len(lines)
		}
		if len(kp.horizon) != 1 || len(kp.paths) != 1 || retained != len(kp.paths[kp.end]) || len(kp.seeds) != retained || len(kp.retired) == 0 || len(kp.preds) <= retained {
			t.Fatal("forced boundary must retain only terminal work, but preserve predecessor history")
		}
	}
}

func TestIndexedUpdatesAndScoringStayWithinBreakpoint(t *testing.T) {
	kp := newLinebreaker(khipu.Params{LineShape: linebreak.RectangularParShape(100)}, nil)
	target := BreakRef{At: 2, Variant: 1}
	refs := []BreakRef{PlainBreak(2), target, {At: 2, Variant: 2}, PlainBreak(7)}
	before := make(map[BreakRef]map[lineNo]bookkeeping)
	for _, ref := range refs {
		kp.paths[ref] = map[lineNo]bookkeeping{
			1: {segment: WSS{W: 20, Min: 10, Max: 200}, seenContent: true, totalcost: 5},
			4: {segment: WSS{W: 30, Min: 10, Max: 200}, seenContent: true, totalcost: 9},
		}
		before[ref] = maps.Clone(kp.paths[ref])
	}
	knot := khipu.KnotCore{Kind: khipu.KTTextBox, W: 10, MinW: 10, MaxW: 10}
	kp.updatePath(target, target.At, knot)
	if !maps.Equal(kp.paths[target], before[target]) {
		t.Fatal("discretionary state advanced before its source textbox")
	}
	kp.updatePath(target, target.At+1, knot)
	for _, ref := range refs {
		for line, want := range before[ref] {
			if ref == target {
				appendKnotToBook(&want, knot)
			}
			if got := kp.paths[ref][line]; got != want {
				t.Fatalf("state %v/%d: got %+v want %+v", ref, line, got, want)
			}
		}
	}
	evals := kp.evaluateCandidates(target, 0)
	if len(evals) != 2 {
		t.Fatalf("unexpected evaluations %v", evals)
	}
	seen := make(map[lineNo]bool)
	for _, ev := range evals {
		path, ok := kp.paths[target][ev.line]
		badness, _ := calcBadness(path.effectiveWidth(kp.layout), 100)
		if !ok || seen[ev.line] || ev.bp != target || !ev.accepted() || ev.badness != badness {
			t.Fatalf("incorrect indexed evaluation: %+v", ev)
		}
		seen[ev.line] = true
	}
	missing := PlainBreak(99)
	kp.updatePath(missing, 100, knot)
	if evals := kp.evaluateCandidates(missing, 0); len(evals) != 0 {
		t.Fatalf("missing bucket produced evaluations: %v", evals)
	}
	if _, exists := kp.paths[missing]; exists {
		t.Fatal("read created a missing bucket")
	}
}

func TestIndexedDiscretionaryScoringUsesMatchingLinesAndSeeds(t *testing.T) {
	kp := newLinebreaker(khipu.Params{
		LineShape: linebreak.RectangularParShape(30 * dimen.BP), RightSkip: glue(0, 0, 20*dimen.BP),
	}, nil)
	from := BreakRef{At: 0, Variant: 1}
	seed := seedBookkeepingFromKnot(khipu.KnotCore{Kind: khipu.KTTextBox, W: 10 * dimen.BP, MinW: 10 * dimen.BP, MaxW: 10 * dimen.BP})
	for _, ref := range []BreakRef{PlainBreak(0), from, {At: 0, Variant: 2}} {
		kp.paths[ref] = map[lineNo]bookkeeping{1: seed, 4: seed}
		for line := range kp.paths[ref] {
			kp.seeds[origin{ref, line}] = seed
		}
	}
	// Distinct seeds catch accidentally rescanning with another line's state.
	seed4 := seed
	seed4.segment = WSS{W: 12 * dimen.BP, Min: 12 * dimen.BP, Max: 12 * dimen.BP}
	kp.seeds[origin{from, 4}] = seed4
	khp := newKhipu([]node{
		{w: 50, minw: 50, maxw: 50, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 1, minw: 1, maxw: 1, kind: khipu.KTGlue},
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	cand := khipu.DiscretionaryCandidate{
		Variant:   3,
		PreBreak:  khipu.KnotCore{Kind: khipu.KTTextBox, W: 10 * dimen.BP, MinW: 10 * dimen.BP, MaxW: 10 * dimen.BP},
		PostBreak: khipu.KnotCore{Kind: khipu.KTTextBox, W: 5 * dimen.BP, MinW: 5 * dimen.BP, MaxW: 5 * dimen.BP},
	}
	khp.AddDiscretionaryCandidate(2, cand)
	// Exercise both next-box lookahead and a rescan from the stored seed.
	for _, at := range []kinx{1, 2} {
		got := kp.evaluateDiscretionaryCandidates(khp, from, at, 2)
		if len(got) != 2 {
			t.Fatalf("at %d: got %d candidates", at, len(got))
		}
		seen := make(map[lineNo]bool)
		for _, result := range got {
			if result.eval.bp != from || (result.eval.line != 1 && result.eval.line != 4) || seen[result.eval.line] ||
				result.target != (BreakRef{At: 2, Variant: 3}) || result.seed != seedBookkeepingFromKnot(cand.PostBreak) {
				t.Fatalf("wrong bucket, line or fragment: %+v", result)
			}
			wantWidth := dimen.DU(20 * dimen.BP)
			if at == 2 {
				wantWidth = 21 * dimen.BP
				if result.eval.line == 4 {
					wantWidth = 23 * dimen.BP
				}
			}
			wantBadness, _ := calcBadness(WSS{W: wantWidth, Min: wantWidth, Max: wantWidth + 20*dimen.BP}, 30*dimen.BP)
			if result.eval.badness != wantBadness || result.eval.stretch != 30*dimen.BP-wantWidth {
				t.Fatalf("wrong line-specific seed at %d: %+v", at, result.eval)
			}
			seen[result.eval.line] = true
		}
	}
	if got := kp.evaluateDiscretionaryCandidates(khp, PlainBreak(99), 1, 2); len(got) != 0 {
		t.Fatalf("missing origin: %v", got)
	}
}

func TestIndexedReplacementKeepsSiblingStates(t *testing.T) {
	kp := newLinebreaker(DefaultLayout(nil), nil)
	from, cheaper := PlainBreak(1), PlainBreak(2)
	target := BreakRef{At: 5, Variant: 1}
	otherVariant := BreakRef{At: 5, Variant: 2}
	kp.paths[from] = map[lineNo]bookkeeping{1: {totalcost: 10}, 2: {totalcost: 20}}
	kp.paths[cheaper] = map[lineNo]bookkeeping{1: {totalcost: 5, worstbadness: 3}}
	oldSeed := bookkeeping{segment: WSS{W: 7}, seenContent: true}
	newSeed := bookkeeping{segment: WSS{W: 9}, seenContent: true}
	kp.evalNewSegmentSeed(from, target, 2, cost{demerits: 10}, oldSeed)
	kp.evalNewSegmentSeed(from, target, 3, cost{demerits: 10}, oldSeed)
	kp.evalNewSegmentSeed(from, otherVariant, 2, cost{demerits: 10}, oldSeed)
	sibling := kp.paths[target][3]
	variant := kp.paths[otherVariant][2]
	kp.evalNewSegmentSeed(cheaper, target, 2, cost{demerits: 5, badness: 2}, newSeed)
	pred, ok := kp.Pred(target, 2)
	path := kp.paths[target][2]
	if !ok || pred.from != cheaper || pred.total != 10 || path.totalcost != 10 || path.worstbadness != 3 || path.segment != newSeed.segment || kp.seeds[origin{target, 2}] != newSeed {
		t.Fatalf("replacement did not update the matching state: %+v, %+v", pred, path)
	}
	if len(kp.paths[target]) != 2 || kp.paths[target][3] != sibling || kp.paths[otherVariant][2] != variant ||
		kp.seeds[origin{target, 3}] != oldSeed || kp.seeds[origin{otherVariant, 2}] != oldSeed {
		t.Fatal("replacement changed a sibling line or discretionary variant")
	}
}
