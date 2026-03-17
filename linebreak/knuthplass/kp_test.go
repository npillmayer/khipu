package knuthplass

import (
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestKPVoid(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	params := DefaultParameters
	parfillnode := node{
		params.ParFillSkip.W,
		params.ParFillSkip.MinW,
		params.ParFillSkip.MaxW,
		InfinityMerits,
		khipu.KTGlue,
	}
	khp := newKhipu([]node{
		{w: 100, minw: 80, maxw: 120, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 10, minw: 5, maxw: 15, p: -100, kind: khipu.KTGlue},
		{w: 80, minw: 60, maxw: 110, p: 5000, kind: khipu.KTTextBox},
		parfillnode,
	})
	t.Logf("khipu of length %d", len(khp.Kind))
	t.Logf("khipu: %v", khp)
	parshape := linebreak.RectangularParShape(100 * dimen.BP)
	breakpoints, err := BreakParagraph(khp, parshape, nil)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{1, 3}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected breakpoints %v, got %v", want, breakpoints)
	}
}

func TestBookkeepingAppendItem(t *testing.T) {
	book := bookkeeping{}
	glue := WSS{}.SetFromKnot(khipu.KnotCore{
		W: 10 * dimen.BP, MinW: 5 * dimen.BP, MaxW: 15 * dimen.BP, Kind: khipu.KTGlue,
	})
	text := WSS{}.SetFromKnot(khipu.KnotCore{
		W: 80 * dimen.BP, MinW: 80 * dimen.BP, MaxW: 80 * dimen.BP, Kind: khipu.KTTextBox,
	})
	terminal := WSS{}.SetFromKnot(khipu.KnotCore{
		W: 0, MinW: 0, MaxW: dimen.Fill, Penalty: InfinityMerits, Kind: khipu.KTGlue,
	})

	book.appendItem(LICTrimDiscardable, glue)
	if book.leadingTrim != glue {
		t.Fatalf("expected leading trim %v, got %v", glue, book.leadingTrim)
	}
	if book.seenContent {
		t.Fatalf("leading glue must not count as content")
	}

	book.appendItem(LICContent, text)
	if !book.seenContent {
		t.Fatalf("text must mark bookkeeping as containing content")
	}
	if book.trailingTrim != (WSS{}) {
		t.Fatalf("text must clear trailing trim, got %v", book.trailingTrim)
	}

	book.appendItem(LICTrimDiscardable, glue)
	if book.trailingTrim != glue {
		t.Fatalf("expected trailing trim %v, got %v", glue, book.trailingTrim)
	}

	book.appendItem(LICRetainedNeutral, terminal)
	if book.trailingTrim != glue {
		t.Fatalf("retained neutral item must not alter trailing trim, got %v", book.trailingTrim)
	}
}

func TestClassifyLineItemUsesDiscardableFlags(t *testing.T) {
	k := khipu.KnotCore{
		W: 10 * dimen.BP, MinW: 5 * dimen.BP, MaxW: 15 * dimen.BP, Kind: khipu.KTGlue,
	}
	if got := classifyLineItem(k); got != LICRetainedNeutral {
		t.Fatalf("expected unflagged glue to be retained-neutral, got %v", got)
	}
	k.Flags = khipu.KFDiscardable
	if got := classifyLineItem(k); got != LICTrimDiscardable {
		t.Fatalf("expected flagged glue to be trim-discardable, got %v", got)
	}
}

func TestBookkeepingEffectiveWidth(t *testing.T) {
	book := bookkeeping{
		segment:      WSS{W: 100 * dimen.BP, Min: 90 * dimen.BP, Max: 120 * dimen.BP},
		leadingTrim:  WSS{W: 10 * dimen.BP, Min: 5 * dimen.BP, Max: 15 * dimen.BP},
		trailingTrim: WSS{W: 12 * dimen.BP, Min: 6 * dimen.BP, Max: 18 * dimen.BP},
	}
	params := &Parameters{
		LeftSkip:  glue(3, 0, 0),
		RightSkip: glue(4, 0, 10*dimen.BP),
	}

	got := book.effectiveWidth(params)
	want := WSS{
		W:   85 * dimen.BP,
		Min: 79 * dimen.BP,
		Max: 97 * dimen.BP,
	}
	if got != want {
		t.Fatalf("expected effective width %v, got %v", want, got)
	}
}

func TestBookkeepingEffectiveWidthRetainsInternalDiscardable(t *testing.T) {
	book := bookkeeping{}
	text := WSS{W: 40 * dimen.BP, Min: 40 * dimen.BP, Max: 40 * dimen.BP}
	glue := WSS{W: 10 * dimen.BP, Min: 10 * dimen.BP, Max: 10 * dimen.BP}

	book.appendItem(LICContent, text)
	book.appendItem(LICTrimDiscardable, glue)
	book.appendItem(LICContent, text)

	got := book.effectiveWidth(&Parameters{
		LeftSkip:  glueKnot(0, 0, 0),
		RightSkip: glueKnot(0, 0, 0),
	})
	want := WSS{W: 90 * dimen.BP, Min: 90 * dimen.BP, Max: 90 * dimen.BP}
	if got != want {
		t.Fatalf("expected effective width %v, got %v", want, got)
	}
}

func TestBookkeepingAccumulatesConsecutiveDiscardables(t *testing.T) {
	book := bookkeeping{}
	trim := WSS{W: 10 * dimen.BP, Min: 10 * dimen.BP, Max: 10 * dimen.BP}
	text := WSS{W: 40 * dimen.BP, Min: 40 * dimen.BP, Max: 40 * dimen.BP}

	book.appendItem(LICTrimDiscardable, trim)
	book.appendItem(LICTrimDiscardable, trim)
	if want := (WSS{W: 20 * dimen.BP, Min: 20 * dimen.BP, Max: 20 * dimen.BP}); book.leadingTrim != want {
		t.Fatalf("expected leading trim %v, got %v", want, book.leadingTrim)
	}

	book.appendItem(LICContent, text)
	book.appendItem(LICTrimDiscardable, trim)
	book.appendItem(LICTrimDiscardable, trim)
	if want := (WSS{W: 20 * dimen.BP, Min: 20 * dimen.BP, Max: 20 * dimen.BP}); book.trailingTrim != want {
		t.Fatalf("expected trailing trim %v, got %v", want, book.trailingTrim)
	}
}

func TestBreakParagraphTrimsTrailingDiscardableAtBreakpoint(t *testing.T) {
	params := DefaultParameters
	parfillnode := node{
		params.ParFillSkip.W,
		params.ParFillSkip.MinW,
		params.ParFillSkip.MaxW,
		InfinityMerits,
		khipu.KTGlue,
	}
	khp := newKhipu([]node{
		{w: 80, minw: 80, maxw: 80, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 10, minw: 10, maxw: 10, p: -100, kind: khipu.KTKern},
		{w: 80, minw: 80, maxw: 80, p: InfinityDemerits, kind: khipu.KTTextBox},
		parfillnode,
	})

	breakpoints, err := BreakParagraph(khp, linebreak.RectangularParShape(80*dimen.BP), nil)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{1, 3}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected breakpoints %v, got %v", want, breakpoints)
	}
}

func TestBreakParagraphTrimsLeadingDiscardableOnLaterLine(t *testing.T) {
	params := DefaultParameters
	parfillnode := node{
		params.ParFillSkip.W,
		params.ParFillSkip.MinW,
		params.ParFillSkip.MaxW,
		InfinityMerits,
		khipu.KTGlue,
	}
	khp := newKhipu([]node{
		{w: 80, minw: 80, maxw: 80, p: -100, kind: khipu.KTTextBox},
		{w: 10, minw: 10, maxw: 10, p: InfinityDemerits, kind: khipu.KTKern},
		{w: 80, minw: 80, maxw: 80, p: InfinityDemerits, kind: khipu.KTTextBox},
		parfillnode,
	})

	breakpoints, err := BreakParagraph(khp, linebreak.RectangularParShape(80*dimen.BP), nil)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{0, 3}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected breakpoints %v, got %v", want, breakpoints)
	}
}

// ---------------------------------------------------------------------------

func glueKnot(w, wmin int, wmax dimen.DU) khipu.KnotCore {
	return khipu.KnotCore{
		W:       dimen.DU(w) * dimen.BP,
		MinW:    dimen.DU(wmin) * dimen.BP,
		MaxW:    wmax,
		Penalty: 0,
		Kind:    khipu.KTGlue,
		Flags:   khipu.KFDiscardable,
	}
}

type node struct {
	w, minw, maxw dimen.DU
	p             int
	kind          khipu.KnotType
}

func newKhipu(nodes []node) *khipu.Khipu {
	khp := testKhipu()
	for _, n := range nodes {
		khp.W = append(khp.W, n.w*dimen.BP)
		khp.MinW = append(khp.MinW, n.minw*dimen.BP)
		khp.MaxW = append(khp.MaxW, n.maxw*dimen.BP)
		khp.Penalty = append(khp.Penalty, khipu.Penalty(n.p))
		khp.Kind = append(khp.Kind, n.kind)
		khp.Pos = append(khp.Pos, 0)
		khp.Len = append(khp.Len, 0)
		khp.Flags = append(khp.Flags, discardFlagsForKind(n.kind))
	}
	return khp
}

func testKhipu() *khipu.Khipu {
	khipu := khipu.Khipu{
		W:       make([]dimen.DU, 0, 50),
		MinW:    make([]dimen.DU, 0, 50),
		MaxW:    make([]dimen.DU, 0, 50),
		Penalty: make([]khipu.Penalty, 0, 50),
		Pos:     make([]uint64, 0, 50),
		Len:     make([]uint16, 0, 50),
		Kind:    make([]khipu.KnotType, 0, 50),
		Flags:   make([]khipu.KnotFlags, 0, 50),
	}
	return &khipu
}

func discardFlagsForKind(kind khipu.KnotType) khipu.KnotFlags {
	switch kind {
	case khipu.KTGlue, khipu.KTKern:
		return khipu.KFDiscardable
	default:
		return 0
	}
}
