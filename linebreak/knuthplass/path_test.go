package knuthplass

import (
	"slices"
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestBreakRefHelpers(t *testing.T) {
	plain := PlainBreak(7)
	if plain.At != 7 || plain.Variant != 0 {
		t.Fatalf("unexpected plain break %+v", plain)
	}
	if plain.IsDiscretionary() {
		t.Fatalf("plain break must not be discretionary")
	}
	if got := plain.String(); got != "k-7" {
		t.Fatalf("expected plain break string k-7, got %q", got)
	}
	discretionary := BreakRef{At: 7, Variant: 2}
	if !discretionary.IsDiscretionary() {
		t.Fatalf("discretionary break must report discretionary")
	}
	if got := discretionary.Kinx(); got != 7 {
		t.Fatalf("expected discretionary break kinx 7, got %d", got)
	}
	if got := discretionary.String(); got != "k-7[v:2]" {
		t.Fatalf("expected discretionary break string k-7[v:2], got %q", got)
	}
	start := PlainBreak(noinx)
	if got := start.String(); got != "START" {
		t.Fatalf("expected start break string START, got %q", got)
	}
}

func TestPathTableAddBreakpoint(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	pt := newPathTable()
	if err := pt.AddBP(PlainBreak(3)); err != nil {
		t.Fatalf("unexpected error adding breakpoint: %v", err)
	}
	if bp := pt.Breakpoint(PlainBreak(3)); bp != PlainBreak(3) {
		t.Fatalf("expected breakpoint 3, got %v", bp)
	}
	if bp := pt.Breakpoint(PlainBreak(5)); bp != NoBreakRef {
		t.Fatalf("expected unknown breakpoint to return %v, got %v", NoBreakRef, bp)
	}
}

func TestPathTableStoresPredecessor(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	pt := newPathTable()
	if !pt.SetPred(PlainBreak(3), PlainBreak(5), 100, 120, 2) {
		t.Fatalf("expected predecessor state to be stored")
	}
	pred, ok := pt.Pred(PlainBreak(5), 2)
	if !ok {
		t.Fatalf("expected predecessor for state (to=5,line=2)")
	}
	if pred.from != PlainBreak(3) || pred.cost != 100 || pred.total != 120 {
		t.Fatalf("unexpected predecessor %+v", pred)
	}
	if bp := pt.Breakpoint(PlainBreak(5)); bp != PlainBreak(5) {
		t.Fatalf("expected endpoint 5 to be registered as breakpoint, got %v", bp)
	}
}

func TestPathTablePrefersCheaperPredecessor(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	pt := newPathTable()
	if !pt.SetPred(PlainBreak(1), PlainBreak(5), 100, 100, 2) {
		t.Fatalf("expected first predecessor to be stored")
	}
	if !pt.SetPred(PlainBreak(3), PlainBreak(5), 80, 90, 2) {
		t.Fatalf("expected cheaper predecessor to replace existing state")
	}
	pred, ok := pt.Pred(PlainBreak(5), 2)
	if !ok {
		t.Fatalf("expected predecessor for state (to=5,line=2)")
	}
	if pred.from != PlainBreak(3) || pred.total != 90 {
		t.Fatalf("expected cheaper predecessor to survive, got %+v", pred)
	}
}

func TestPathTableCanonicalPredecessorOnTie(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	for _, pair := range [][2]BreakRef{
		{PlainBreak(1), PlainBreak(3)},
		{{At: 1, Variant: 1}, {At: 1, Variant: 2}},
	} {
		for _, reverse := range []bool{false, true} {
			pt := newPathTable()
			order := pair
			if reverse {
				order[0], order[1] = order[1], order[0]
			}
			for _, from := range order {
				pt.SetPred(from, PlainBreak(5), 100, 100, 2)
			}
			pred, ok := pt.Pred(PlainBreak(5), 2)
			if !ok || pred.from != pair[0] || pred.total != 100 {
				t.Fatalf("order %v: unexpected predecessor %+v", order, pred)
			}
			if pt.SetPred(pair[0], PlainBreak(5), 100, 100, 2) {
				t.Fatal("identical predecessor should not replace itself")
			}
		}
	}
}

func TestCollectOptimalBreakRefsCostThenLineCount(t *testing.T) {
	for _, longCost := range []merits{10, 9} {
		kp := newLinebreaker(DefaultLayout(nil), NewKPDefaultParameters())
		kp.root = PlainBreak(-1)
		mid, end := PlainBreak(4), PlainBreak(9)
		kp.pathTable.SetPred(kp.root, end, 10, 10, 1)
		kp.pathTable.SetPred(kp.root, mid, 5, 5, 1)
		kp.pathTable.SetPred(mid, end, longCost-5, longCost, 2)
		kp.paths[end] = map[lineNo]bookkeeping{
			1: {totalcost: 10, worstbadness: 30},
			2: {totalcost: longCost, worstbadness: 20},
		}
		// Neither another knot nor a variant at the same knot is the terminus.
		kp.paths[PlainBreak(8)] = map[lineNo]bookkeeping{1: {totalcost: -1000}}
		kp.paths[BreakRef{At: end.At, Variant: 1}] = map[lineNo]bookkeeping{1: {totalcost: -1000}}
		want, worst := []BreakRef{end}, merits(30)
		if longCost < 10 {
			want, worst = []BreakRef{mid, end}, 20
		}
		for range 100 {
			got, quality, ok := kp.collectOptimalBreakRefs(end)
			if !ok || !slices.Equal(got, want) || quality.totalCost != longCost || quality.worstBadness != worst {
				t.Fatalf("cost %d: breaks %v, quality %+v, ok %v", longCost, got, quality, ok)
			}
		}
	}
}

func TestCanonicalTieReplacesLivePathAndSeed(t *testing.T) {
	kp := newLinebreaker(DefaultLayout(nil), NewKPDefaultParameters())
	low, high, end := PlainBreak(1), PlainBreak(3), PlainBreak(5)
	kp.paths[low] = map[lineNo]bookkeeping{1: {totalcost: 10, worstbadness: 20}}
	kp.paths[high] = map[lineNo]bookkeeping{1: {totalcost: 10, worstbadness: 40}}
	seed := bookkeeping{segment: WSS{W: 7}, seenContent: true}
	kp.evalNewSegmentSeed(high, end, 2, cost{demerits: 5, badness: 10}, bookkeeping{})
	kp.evalNewSegmentSeed(low, end, 2, cost{demerits: 5, badness: 10}, seed)
	pred, ok := kp.pathTable.Pred(end, 2)
	path := kp.paths[end][2]
	if !ok || pred.from != low || path.totalcost != 15 || path.worstbadness != 20 ||
		path.segment != seed.segment || kp.seeds[origin{end, 2}] != seed {
		t.Fatalf("canonical tie left inconsistent state: pred %+v, path %+v, seed %+v", pred, path, kp.seeds[origin{end, 2}])
	}
}

func TestPathTableReportsMissingState(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	pt := newPathTable()
	if _, ok := pt.Pred(PlainBreak(5), 2); ok {
		t.Fatalf("expected missing predecessor state")
	}
}
