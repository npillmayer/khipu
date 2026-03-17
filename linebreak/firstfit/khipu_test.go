package firstfit

import (
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

type testDiscretionaryProvider struct {
	calls       int
	requestedAt []int
	want        []khipu.DiscretionaryCandidate
}

func (p *testDiscretionaryProvider) DiscretionaryCandidates(khp *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error) {
	p.calls++
	p.requestedAt = append(p.requestedAt, at)
	return p.want, nil
}

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

func TestBreakParagraphKhipuUsesLooseLineDiscretionary(t *testing.T) {
	provider := &testDiscretionaryProvider{
		want: []khipu.DiscretionaryCandidate{
			{
				Variant: 1,
				PreBreak: khipu.KnotCore{
					W: 6 * dimen.BP, MinW: 6 * dimen.BP, MaxW: 6 * dimen.BP, Penalty: 50, Kind: khipu.KTTextBox,
				},
				PostBreak: khipu.KnotCore{
					W: 14 * dimen.BP, MinW: 14 * dimen.BP, MaxW: 14 * dimen.BP, Kind: khipu.KTTextBox,
				},
			},
			{
				Variant: 2,
				PreBreak: khipu.KnotCore{
					W: 9 * dimen.BP, MinW: 9 * dimen.BP, MaxW: 9 * dimen.BP, Penalty: 80, Kind: khipu.KTTextBox,
				},
				PostBreak: khipu.KnotCore{
					W: 11 * dimen.BP, MinW: 11 * dimen.BP, MaxW: 11 * dimen.BP, Kind: khipu.KTTextBox,
				},
			},
		},
	}
	params := &Parameters{
		LeftSkip:              glue(0, 0, 0),
		RightSkip:             glue(0, 0, 0),
		MinHyphenGain:         5 * dimen.BP,
		DiscretionaryProvider: provider,
	}
	khp := newKhipu([]node{
		{w: 15, minw: 15, maxw: 15, p: -100, kind: khipu.KTTextBox},
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 0, minw: 0, maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})

	breakpoints, err := BreakParagraph(khp, linebreak.RectangularParShape(24*dimen.BP), params)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{1, 2}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected discretionary breakpoints %v, got %v", want, breakpoints)
	}
	if provider.calls != 1 || provider.requestedAt[0] != 1 {
		t.Fatalf("expected one provider call for textbox 1, got calls=%d at=%v", provider.calls, provider.requestedAt)
	}
	choice, ok := khp.SelectedDiscretionaryAt(1)
	if !ok || choice.Variant != 2 {
		t.Fatalf("expected variant 2 to be selected at 1, got %+v ok=%v", choice, ok)
	}
}

func TestBreakParagraphKhipuHonorsMinHyphenGain(t *testing.T) {
	provider := &testDiscretionaryProvider{
		want: []khipu.DiscretionaryCandidate{
			{
				Variant: 1,
				PreBreak: khipu.KnotCore{
					W: 3 * dimen.BP, MinW: 3 * dimen.BP, MaxW: 3 * dimen.BP, Penalty: 50, Kind: khipu.KTTextBox,
				},
				PostBreak: khipu.KnotCore{
					W: 17 * dimen.BP, MinW: 17 * dimen.BP, MaxW: 17 * dimen.BP, Kind: khipu.KTTextBox,
				},
			},
		},
	}
	params := &Parameters{
		LeftSkip:              glue(0, 0, 0),
		RightSkip:             glue(0, 0, 0),
		MinHyphenGain:         5 * dimen.BP,
		DiscretionaryProvider: provider,
	}
	khp := newKhipu([]node{
		{w: 15, minw: 15, maxw: 15, p: -100, kind: khipu.KTTextBox},
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 0, minw: 0, maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})

	breakpoints, err := BreakParagraph(khp, linebreak.RectangularParShape(24*dimen.BP), params)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{0, 2}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected non-hyphenating breakpoints %v, got %v", want, breakpoints)
	}
	if len(khp.SelectedDiscretionaries) != 0 {
		t.Fatalf("expected no selected discretionaries, got %+v", khp.SelectedDiscretionaries)
	}
}

func TestBreakParagraphKhipuClearsStaleSelectedDiscretionaries(t *testing.T) {
	khp := newKhipu([]node{
		{w: 15, minw: 15, maxw: 15, p: -100, kind: khipu.KTTextBox},
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 0, minw: 0, maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	khp.SelectedDiscretionaries = map[int]khipu.DiscretionarySelection{
		1: {Source: 1, Variant: 9},
	}

	breakpoints, err := BreakParagraph(khp, linebreak.RectangularParShape(24*dimen.BP), nil)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{0, 2}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected breakpoints %v, got %v", want, breakpoints)
	}
	if len(khp.SelectedDiscretionaries) != 0 {
		t.Fatalf("expected stale selected discretionaries to be cleared, got %+v", khp.SelectedDiscretionaries)
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
