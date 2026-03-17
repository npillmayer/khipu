package knuthplass

import (
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

type testDiscretionaryProvider struct {
	calls       int
	want        []khipu.DiscretionaryCandidate
	requestedAt []int
}

func (p *testDiscretionaryProvider) DiscretionaryCandidates(khp *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error) {
	p.calls++
	p.requestedAt = append(p.requestedAt, at)
	return p.want, nil
}

func TestDiscretionaryCandidatesFallbackToKhipuCache(t *testing.T) {
	khp := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
	})
	khp.AddDiscretionaryCandidate(0, khipu.DiscretionaryCandidate{
		Variant: 1,
		PreBreak: khipu.KnotCore{
			W: 8 * dimen.BP, MinW: 8 * dimen.BP, MaxW: 8 * dimen.BP, Penalty: khipu.Penalty(50), Kind: khipu.KTTextBox,
		},
		PostBreak: khipu.KnotCore{
			W: 12 * dimen.BP, MinW: 12 * dimen.BP, MaxW: 12 * dimen.BP, Kind: khipu.KTTextBox,
		},
	})
	kp := newLinebreaker(nil, NewKPDefaultParameters())
	got, err := kp.discretionaryCandidates(khp, 0)
	if err != nil {
		t.Fatalf("expected cached discretionary lookup to succeed: %v", err)
	}
	if len(got) != 1 || got[0].Variant != 1 {
		t.Fatalf("unexpected discretionary candidates: %+v", got)
	}
}

func TestDiscretionaryCandidatesDelegateToProvider(t *testing.T) {
	provider := &testDiscretionaryProvider{
		want: []khipu.DiscretionaryCandidate{{
			Variant: 4,
			PreBreak: khipu.KnotCore{
				W: 9 * dimen.BP, MinW: 9 * dimen.BP, MaxW: 9 * dimen.BP, Penalty: khipu.Penalty(50), Kind: khipu.KTTextBox,
			},
			PostBreak: khipu.KnotCore{
				W: 11 * dimen.BP, MinW: 11 * dimen.BP, MaxW: 11 * dimen.BP, Kind: khipu.KTTextBox,
			},
		}},
	}
	params := NewKPDefaultParameters()
	params.DiscretionaryProvider = provider
	khp := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
	})
	kp := newLinebreaker(nil, params)
	got, err := kp.discretionaryCandidates(khp, 0)
	if err != nil {
		t.Fatalf("expected provider lookup to succeed: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("expected provider to be called once, got %d", provider.calls)
	}
	if len(got) != 1 || got[0].Variant != 4 {
		t.Fatalf("unexpected provider candidates: %+v", got)
	}
}

func TestScanSkipsDiscardablesToPreviousAndNextContentBox(t *testing.T) {
	khp := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 5, minw: 5, maxw: 5, p: 0, kind: khipu.KTGlue},
		{w: 3, minw: 3, maxw: 3, p: 0, kind: khipu.KTKern},
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
	})
	if prev, ok := previousContentBox(khp, 2); !ok || prev != 0 {
		t.Fatalf("expected previous content box at 0, got %d ok=%v", prev, ok)
	}
	if next, ok := nextContentBox(khp, 1); !ok || next != 3 {
		t.Fatalf("expected next content box at 3, got %d ok=%v", next, ok)
	}
	if prev, ok := previousNonDiscardable(khp, 2); !ok || prev != 0 {
		t.Fatalf("expected previous non-discardable at 0, got %d ok=%v", prev, ok)
	}
	if next, ok := nextNonDiscardable(khp, 1); !ok || next != 3 {
		t.Fatalf("expected next non-discardable at 3, got %d ok=%v", next, ok)
	}
}

func TestScanStopsAtNonTextboxNonDiscardable(t *testing.T) {
	khp := testKhipu()
	khp.W = append(khp.W, 20*dimen.BP, 0, 5*dimen.BP)
	khp.MinW = append(khp.MinW, 20*dimen.BP, 0, 5*dimen.BP)
	khp.MaxW = append(khp.MaxW, 20*dimen.BP, 0, 5*dimen.BP)
	khp.Penalty = append(khp.Penalty, 0, 0, 0)
	khp.Kind = append(khp.Kind, khipu.KTTextBox, khipu.KTDiscretionary, khipu.KTGlue)
	khp.Pos = append(khp.Pos, 0, 0, 0)
	khp.Len = append(khp.Len, 0, 0, 0)
	khp.Flags = append(khp.Flags, 0, 0, khipu.KFDiscardable)
	if next, ok := nextNonDiscardable(khp, 1); !ok || next != 1 {
		t.Fatalf("expected next non-discardable at 1, got %d ok=%v", next, ok)
	}
	if _, ok := nextContentBox(khp, 1); ok {
		t.Fatalf("expected no next content box past discretionary")
	}
	if prev, ok := previousNonDiscardable(khp, 2); !ok || prev != 1 {
		t.Fatalf("expected previous non-discardable at 1, got %d ok=%v", prev, ok)
	}
	if _, ok := previousContentBox(khp, 2); ok {
		t.Fatalf("expected no previous content box when discretionary intervenes")
	}
}

func TestHyphenationCandidateForLoosePass2Line(t *testing.T) {
	khp := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: -100, kind: khipu.KTTextBox},
		{w: 5, minw: 5, maxw: 5, p: 0, kind: khipu.KTGlue},
		{w: 30, minw: 30, maxw: 30, p: InfinityDemerits, kind: khipu.KTTextBox},
	})
	ev := lineEvaluation{disposition: lineAccepted, ratio: 0.5, badness: 137}
	candidate, ok := hyphenationCandidateForPass2(khp, 0, ev)
	if !ok || candidate != 2 {
		t.Fatalf("expected loose pass-2 candidate at 2, got %d ok=%v", candidate, ok)
	}
}

func TestHyphenationCandidateForTightPass2Line(t *testing.T) {
	khp := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
		{w: 5, minw: 5, maxw: 5, p: 0, kind: khipu.KTGlue},
	})
	ev := lineEvaluation{disposition: lineScreenedOut, ratio: -0.5, badness: 250}
	candidate, ok := hyphenationCandidateForPass2(khp, 1, ev)
	if !ok || candidate != 0 {
		t.Fatalf("expected tight pass-2 candidate at 0, got %d ok=%v", candidate, ok)
	}
}

func TestPass2RequestsDiscretionariesForScreenedOutCandidate(t *testing.T) {
	provider := &testDiscretionaryProvider{}
	params := NewKPDefaultParameters()
	params.LinePenalty = 0
	params.RightSkip = glue(0, 0, 0)
	params.DiscretionaryProvider = provider
	khp := testKhipu()
	khp.W = append(khp.W, 7990, 7990, params.ParFillSkip.W)
	khp.MinW = append(khp.MinW, 7990, 7990, params.ParFillSkip.MinW)
	khp.MaxW = append(khp.MaxW, 7999, 7990, params.ParFillSkip.MaxW)
	khp.Penalty = append(khp.Penalty, -100, InfinityDemerits, InfinityMerits)
	khp.Kind = append(khp.Kind, khipu.KTTextBox, khipu.KTTextBox, khipu.KTGlue)
	khp.Pos = append(khp.Pos, 0, 0, 0)
	khp.Len = append(khp.Len, 0, 0, 0)

	_, _, ok, err := breakParagraphPass(khp, linebreak.RectangularParShape(8000), params, true)
	if err != nil {
		t.Fatalf("second pass failed unexpectedly: %v", err)
	}
	if !ok {
		t.Fatalf("expected second pass to finish")
	}
	if provider.calls == 0 {
		t.Fatalf("expected provider to be called for pass-2 candidate")
	}
	if provider.requestedAt[0] != 1 {
		t.Fatalf("expected provider to be asked for textbox 1, got %v", provider.requestedAt)
	}
}

func TestBreakParagraphSelectsDiscretionaryVariantOnSecondPass(t *testing.T) {
	provider := &testDiscretionaryProvider{
		want: []khipu.DiscretionaryCandidate{{
			Variant: 1,
			PreBreak: khipu.KnotCore{
				W: 5 * dimen.BP, MinW: 5 * dimen.BP, MaxW: 5 * dimen.BP, Penalty: khipu.Penalty(50), Kind: khipu.KTTextBox,
			},
			PostBreak: khipu.KnotCore{
				W: 15 * dimen.BP, MinW: 15 * dimen.BP, MaxW: 15 * dimen.BP, Kind: khipu.KTTextBox,
			},
		}},
	}
	params := NewKPDefaultParameters()
	params.LinePenalty = 0
	params.RightSkip = glue(0, 0, 5*dimen.BP)
	params.DiscretionaryProvider = provider
	khp := testKhipu()
	khp.W = append(khp.W, 20*dimen.BP, 1*dimen.BP, 20*dimen.BP, params.ParFillSkip.W)
	khp.MinW = append(khp.MinW, 20*dimen.BP, 1*dimen.BP, 20*dimen.BP, params.ParFillSkip.MinW)
	khp.MaxW = append(khp.MaxW, 20*dimen.BP, 1*dimen.BP, 20*dimen.BP, params.ParFillSkip.MaxW)
	khp.Penalty = append(khp.Penalty, -100, 0, InfinityDemerits, InfinityMerits)
	khp.Kind = append(khp.Kind, khipu.KTTextBox, khipu.KTGlue, khipu.KTTextBox, khipu.KTGlue)
	khp.Pos = append(khp.Pos, 0, 0, 0, 0)
	khp.Len = append(khp.Len, 0, 0, 0, 0)

	breakpoints, err := BreakParagraph(khp, linebreak.RectangularParShape(26*dimen.BP), params)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{2, 3}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected discretionary breakpoints %v, got %v", want, breakpoints)
	}
	choice, ok := khp.SelectedDiscretionaryAt(2)
	if !ok {
		t.Fatalf("expected selected discretionary to be recorded at breakpoint 2")
	}
	if choice.Variant != 1 {
		t.Fatalf("expected discretionary variant 1 to be selected, got %+v", choice)
	}
	if len(khp.SelectedDiscretionaries) != 1 {
		t.Fatalf("expected exactly one selected discretionary, got %+v", khp.SelectedDiscretionaries)
	}
	if provider.calls == 0 {
		t.Fatalf("expected discretionary provider to be consulted")
	}
}

func TestBreakParagraphClearsStaleSelectedDiscretionaries(t *testing.T) {
	params := NewKPDefaultParameters()
	params.LinePenalty = 0
	params.RightSkip = glue(0, 0, 0)
	parfillnode := node{
		params.ParFillSkip.W,
		params.ParFillSkip.MinW,
		params.ParFillSkip.MaxW,
		InfinityMerits,
		khipu.KTGlue,
	}
	khp := newKhipu([]node{
		{w: 20, minw: 20, maxw: 20, p: -100, kind: khipu.KTTextBox},
		{w: 20, minw: 20, maxw: 20, p: InfinityDemerits, kind: khipu.KTTextBox},
		parfillnode,
	})
	khp.SelectedDiscretionaries = map[int]khipu.DiscretionarySelection{
		0: {Source: 0, Variant: 9},
	}

	breakpoints, err := BreakParagraph(khp, linebreak.RectangularParShape(20*dimen.BP), params)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{0, 2}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected ordinary breakpoints %v, got %v", want, breakpoints)
	}
	if len(khp.SelectedDiscretionaries) != 0 {
		t.Fatalf("expected stale discretionary selections to be cleared, got %+v", khp.SelectedDiscretionaries)
	}
}

func TestDiscretionaryCandidateUsesPreBreakPenaltyForDemerits(t *testing.T) {
	params := NewKPDefaultParameters()
	params.LinePenalty = 0
	params.RightSkip = glue(0, 0, 0)
	khp := testKhipu()
	khp.W = append(khp.W, 20*dimen.BP, 1*dimen.BP, 20*dimen.BP, params.ParFillSkip.W)
	khp.MinW = append(khp.MinW, 20*dimen.BP, 1*dimen.BP, 20*dimen.BP, params.ParFillSkip.MinW)
	khp.MaxW = append(khp.MaxW, 20*dimen.BP, 1*dimen.BP, 20*dimen.BP, params.ParFillSkip.MaxW)
	khp.Penalty = append(khp.Penalty, -100, 0, InfinityDemerits, InfinityMerits)
	khp.Kind = append(khp.Kind, khipu.KTTextBox, khipu.KTGlue, khipu.KTTextBox, khipu.KTGlue)
	khp.Pos = append(khp.Pos, 0, 0, 0, 0)
	khp.Len = append(khp.Len, 0, 0, 0, 0)
	khp.AddDiscretionaryCandidate(2, khipu.DiscretionaryCandidate{
		Variant: 1,
		PreBreak: khipu.KnotCore{
			W: 5 * dimen.BP, MinW: 5 * dimen.BP, MaxW: 5 * dimen.BP, Penalty: khipu.Penalty(50), Kind: khipu.KTTextBox,
		},
		PostBreak: khipu.KnotCore{
			W: 15 * dimen.BP, MinW: 15 * dimen.BP, MaxW: 15 * dimen.BP, Kind: khipu.KTTextBox,
		},
	})
	khp.AddDiscretionaryCandidate(2, khipu.DiscretionaryCandidate{
		Variant: 2,
		PreBreak: khipu.KnotCore{
			W: 5 * dimen.BP, MinW: 5 * dimen.BP, MaxW: 5 * dimen.BP, Penalty: khipu.Penalty(200), Kind: khipu.KTTextBox,
		},
		PostBreak: khipu.KnotCore{
			W: 15 * dimen.BP, MinW: 15 * dimen.BP, MaxW: 15 * dimen.BP, Kind: khipu.KTTextBox,
		},
	})
	kp, err := prepareLineBreaker(linebreak.RectangularParShape(26*dimen.BP), params)
	if err != nil {
		t.Fatalf("prepareLineBreaker failed: %v", err)
	}
	kp.hyphenating = true
	kp.end = PlainBreak(len(khp.W) - 1)
	kp.updatePath(kp.root, 0, khp.KnotByIndex(0))
	cands := kp.evaluateDiscretionaryCandidates(khp, kp.root, 0, 2)
	if len(cands) != 2 {
		t.Fatalf("expected 2 discretionary candidates, got %d", len(cands))
	}
	if cands[0].target.Variant != 1 || cands[1].target.Variant != 2 {
		t.Fatalf("unexpected target variants: %+v", cands)
	}
	if cands[0].eval.demerits >= cands[1].eval.demerits {
		t.Fatalf("expected lower-penalty candidate to have lower demerits: %d vs %d", cands[0].eval.demerits, cands[1].eval.demerits)
	}
}
