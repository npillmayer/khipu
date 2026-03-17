package knuthplass

import (
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

func TestPathTableKeepsStablePredecessorOnTie(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	pt := newPathTable()
	if !pt.SetPred(PlainBreak(1), PlainBreak(5), 100, 100, 2) {
		t.Fatalf("expected first predecessor to be stored")
	}
	if pt.SetPred(PlainBreak(3), PlainBreak(5), 90, 100, 2) {
		t.Fatalf("expected equal-cost predecessor not to replace existing state")
	}
	pred, ok := pt.Pred(PlainBreak(5), 2)
	if !ok {
		t.Fatalf("expected predecessor for state (to=5,line=2)")
	}
	if pred.from != PlainBreak(1) || pred.total != 100 {
		t.Fatalf("expected original predecessor to remain on tie, got %+v", pred)
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
