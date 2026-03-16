package knuthplass

import (
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestPathTableAddBreakpoint(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	pt := newPathTable()
	if err := pt.AddBP(3); err != nil {
		t.Fatalf("unexpected error adding breakpoint: %v", err)
	}
	if bp := pt.Breakpoint(3); bp != 3 {
		t.Fatalf("expected breakpoint 3, got %d", bp)
	}
	if bp := pt.Breakpoint(5); bp != noinx {
		t.Fatalf("expected unknown breakpoint to return %d, got %d", noinx, bp)
	}
}

func TestPathTableStoresPredecessor(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	pt := newPathTable()
	if !pt.SetPred(3, 5, 100, 120, 2) {
		t.Fatalf("expected predecessor state to be stored")
	}
	pred, ok := pt.Pred(5, 2)
	if !ok {
		t.Fatalf("expected predecessor for state (to=5,line=2)")
	}
	if pred.from != 3 || pred.cost != 100 || pred.total != 120 {
		t.Fatalf("unexpected predecessor %+v", pred)
	}
	if bp := pt.Breakpoint(5); bp != 5 {
		t.Fatalf("expected endpoint 5 to be registered as breakpoint, got %d", bp)
	}
}

func TestPathTablePrefersCheaperPredecessor(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	pt := newPathTable()
	if !pt.SetPred(1, 5, 100, 100, 2) {
		t.Fatalf("expected first predecessor to be stored")
	}
	if !pt.SetPred(3, 5, 80, 90, 2) {
		t.Fatalf("expected cheaper predecessor to replace existing state")
	}
	pred, ok := pt.Pred(5, 2)
	if !ok {
		t.Fatalf("expected predecessor for state (to=5,line=2)")
	}
	if pred.from != 3 || pred.total != 90 {
		t.Fatalf("expected cheaper predecessor to survive, got %+v", pred)
	}
}

func TestPathTableKeepsStablePredecessorOnTie(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	pt := newPathTable()
	if !pt.SetPred(1, 5, 100, 100, 2) {
		t.Fatalf("expected first predecessor to be stored")
	}
	if pt.SetPred(3, 5, 90, 100, 2) {
		t.Fatalf("expected equal-cost predecessor not to replace existing state")
	}
	pred, ok := pt.Pred(5, 2)
	if !ok {
		t.Fatalf("expected predecessor for state (to=5,line=2)")
	}
	if pred.from != 1 || pred.total != 100 {
		t.Fatalf("expected original predecessor to remain on tie, got %+v", pred)
	}
}

func TestPathTableReportsMissingState(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	pt := newPathTable()
	if _, ok := pt.Pred(5, 2); ok {
		t.Fatalf("expected missing predecessor state")
	}
}
