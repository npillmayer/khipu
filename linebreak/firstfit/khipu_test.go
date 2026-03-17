package firstfit

import (
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

type node struct {
	w, minw, maxw dimen.DU
	p             int
	kind          khipu.KnotType
}

func newKhipu(nodes []node) *khipu.Khipu {
	khp := khipu.Khipu{
		W:       make([]dimen.DU, 0, 16),
		MinW:    make([]dimen.DU, 0, 16),
		MaxW:    make([]dimen.DU, 0, 16),
		Penalty: make([]khipu.Penalty, 0, 16),
		Pos:     make([]uint64, 0, 16),
		Len:     make([]uint16, 0, 16),
		Kind:    make([]khipu.KnotType, 0, 16),
		Flags:   make([]khipu.KnotFlags, 0, 16),
	}
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
	return &khp
}

func TestBreakParagraphKhipuBreaksAtFirstFitCheckpoint(t *testing.T) {
	khp := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: -100, kind: khipu.KTTextBox},
		{w: 20, minw: 20, maxw: 20, p: -100, kind: khipu.KTTextBox},
		{w: 0, minw: 0, maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	breakpoints, err := BreakParagraph(khp, linebreak.RectangularParShape(20*dimen.BP), nil)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{0, 2}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected breakpoints %v, got %v", want, breakpoints)
	}
}

func TestBreakParagraphKhipuTrimsTrailingDiscardablesAtCheckpoint(t *testing.T) {
	params := &Parameters{
		LeftSkip:  glue(0, 0, 0),
		RightSkip: glue(0, 0, 0),
	}
	khp := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 5, minw: 5, maxw: 5, p: -100, kind: khipu.KTGlue},
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 0, minw: 0, maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	khp.Flags[1] = khipu.KFDiscardable
	khp.Flags[3] = khipu.KFDiscardable
	breakpoints, err := BreakParagraph(khp, linebreak.RectangularParShape(20*dimen.BP), params)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{1, 3}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected breakpoints %v, got %v", want, breakpoints)
	}
}

func TestBreakParagraphKhipuRespectsForcedBreak(t *testing.T) {
	khp := newKhipu([]node{
		{w: 10, minw: 10, maxw: 10, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 10, minw: 10, maxw: 10, p: InfinityMerits, kind: khipu.KTTextBox},
		{w: 10, minw: 10, maxw: 10, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 0, minw: 0, maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	breakpoints, err := BreakParagraph(khp, linebreak.RectangularParShape(100*dimen.BP), nil)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{1, 3}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected forced breakpoints %v, got %v", want, breakpoints)
	}
}

func TestBreakParagraphKhipuTrimsLeadingDiscardablesOnLaterLine(t *testing.T) {
	khp := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: -100, kind: khipu.KTTextBox},
		{w: 5, minw: 5, maxw: 5, p: InfinityDemerits, kind: khipu.KTGlue},
		{w: 20, minw: 20, maxw: 20, p: -100, kind: khipu.KTTextBox},
		{w: 0, minw: 0, maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	khp.Flags[1] = khipu.KFDiscardable
	khp.Flags[3] = khipu.KFDiscardable

	breakpoints, err := BreakParagraph(khp, linebreak.RectangularParShape(20*dimen.BP), nil)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{0, 3}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected later-line leading trim breakpoints %v, got %v", want, breakpoints)
	}
}

func TestSegmentStateTracksLeadingAndTrailingTrim(t *testing.T) {
	seg := &segmentState{}

	seg.append(knot(5, 5, 5, -100, khipu.KTGlue))
	if seg.seenContent {
		t.Fatalf("leading glue must not mark segment as contentful")
	}
	if seg.leadingTrim.W != 5*dimen.BP {
		t.Fatalf("expected leading trim 5bp, got %v", seg.leadingTrim)
	}
	if seg.trailingTrim.W != 0 {
		t.Fatalf("expected no trailing trim yet, got %v", seg.trailingTrim)
	}

	seg.append(knot(20, 20, 20, -100, khipu.KTTextBox))
	if !seg.seenContent {
		t.Fatalf("textbox must mark segment as contentful")
	}
	if seg.trailingTrim.W != 0 {
		t.Fatalf("expected trailing trim reset after content, got %v", seg.trailingTrim)
	}

	seg.append(knot(3, 3, 3, -100, khipu.KTKern))
	if seg.trailingTrim.W != 3*dimen.BP {
		t.Fatalf("expected trailing trim 3bp after kern, got %v", seg.trailingTrim)
	}
}

func TestSegmentStateTreatsPenaltyAsNeutral(t *testing.T) {
	seg := &segmentState{}
	seg.append(knot(0, 0, 0, -100, khipu.KTPenalty))
	if seg.seenContent {
		t.Fatalf("penalty must not mark segment as contentful")
	}
	if seg.leadingTrim.W != 0 || seg.trailingTrim.W != 0 {
		t.Fatalf("penalty must not contribute to trim state, got leading=%v trailing=%v", seg.leadingTrim, seg.trailingTrim)
	}
}

func TestSegmentStateTreatsUnflaggedGlueAsNeutral(t *testing.T) {
	seg := &segmentState{}
	g := knot(5, 5, 5, -100, khipu.KTGlue)
	g.Flags = 0
	seg.append(g)
	if seg.seenContent {
		t.Fatalf("unflagged glue must not mark segment as contentful")
	}
	if seg.leadingTrim.W != 0 || seg.trailingTrim.W != 0 {
		t.Fatalf("unflagged glue must not contribute to trim state, got leading=%v trailing=%v", seg.leadingTrim, seg.trailingTrim)
	}
}

func TestSegmentStateWidthTrimsBothEdges(t *testing.T) {
	seg := &segmentState{}
	params := &Parameters{LeftSkip: glue(0, 0, 0), RightSkip: glue(0, 0, 0)}
	seg.append(knot(5, 5, 5, -100, khipu.KTGlue))
	seg.append(knot(20, 20, 20, -100, khipu.KTTextBox))
	seg.append(knot(3, 3, 3, -100, khipu.KTKern))

	if got := seg.width(params); got.W != 20*dimen.BP {
		t.Fatalf("expected effective width 20bp, got %v", got)
	}
}

func TestSegmentStateResetRestoresCarryTrimState(t *testing.T) {
	seg := &segmentState{}
	seg.append(knot(20, 20, 20, -100, khipu.KTTextBox))
	seg.trackcarry()
	seg.append(knot(5, 5, 5, -100, khipu.KTGlue))
	seg.append(knot(20, 20, 20, -100, khipu.KTTextBox))

	seg.reset()

	if seg.length.W != 25*dimen.BP {
		t.Fatalf("expected carried segment width 25bp, got %v", seg.length)
	}
	if seg.leadingTrim.W != 5*dimen.BP {
		t.Fatalf("expected carried leading trim 5bp, got %v", seg.leadingTrim)
	}
	if !seg.seenContent {
		t.Fatalf("expected carried segment to retain content state")
	}
}

func knot(w, minw, maxw dimen.DU, p int, kind khipu.KnotType) khipu.KnotCore {
	return khipu.KnotCore{
		W:       w * dimen.BP,
		MinW:    minw * dimen.BP,
		MaxW:    maxw * dimen.BP,
		Penalty: khipu.Penalty(p),
		Kind:    kind,
		Flags:   discardFlagsForKind(kind),
	}
}

func discardFlagsForKind(kind khipu.KnotType) khipu.KnotFlags {
	switch kind {
	case khipu.KTGlue, khipu.KTKern:
		return khipu.KFDiscardable
	default:
		return 0
	}
}
