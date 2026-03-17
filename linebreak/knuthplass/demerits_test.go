package knuthplass

import (
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

func TestCalcAdjustmentRatioAndBadness(t *testing.T) {
	tests := []struct {
		name      string
		segwss    WSS
		linelen   int
		wantRatio float64
		wantBad   merits
		wantOK    bool
	}{
		{
			name:      "exact fit",
			segwss:    WSS{W: 100, Min: 90, Max: 120},
			linelen:   100,
			wantRatio: 0,
			wantBad:   0,
			wantOK:    true,
		},
		{
			name:      "stretch",
			segwss:    WSS{W: 90, Min: 90, Max: 110},
			linelen:   100,
			wantRatio: 0.5,
			wantBad:   12,
			wantOK:    true,
		},
		{
			name:      "shrink",
			segwss:    WSS{W: 120, Min: 110, Max: 200},
			linelen:   115,
			wantRatio: -0.5,
			wantBad:   12,
			wantOK:    true,
		},
		{
			name:      "capped awful badness",
			segwss:    WSS{W: 90, Min: 90, Max: 95},
			linelen:   100,
			wantRatio: 2,
			wantBad:   800,
			wantOK:    true,
		},
		{
			name:      "no stretch available",
			segwss:    WSS{W: 100, Min: 100, Max: 100},
			linelen:   110,
			wantRatio: 0,
			wantBad:   0,
			wantOK:    false,
		},
		{
			name:      "no shrink available",
			segwss:    WSS{W: 100, Min: 100, Max: 120},
			linelen:   90,
			wantRatio: 0,
			wantBad:   0,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio, ok := calcAdjustmentRatio(tt.segwss, dimenDU(tt.linelen))
			if ok != tt.wantOK {
				t.Fatalf("expected ratio ok=%v, got %v", tt.wantOK, ok)
			}
			if !ok {
				return
			}
			if ratio != tt.wantRatio {
				t.Fatalf("expected ratio %.2f, got %.2f", tt.wantRatio, ratio)
			}
			badness, ok := calcBadness(tt.segwss, dimenDU(tt.linelen))
			if !ok {
				t.Fatalf("expected badness computation to succeed")
			}
			if badness != tt.wantBad {
				t.Fatalf("expected badness %d, got %d", tt.wantBad, badness)
			}
		})
	}
}

func TestCalcDemeritsSquaresPenalty(t *testing.T) {
	params := NewKPDefaultParameters()
	params.LinePenalty = 10
	if got := calcDemerits(50, 50, params); got != 6100 {
		t.Fatalf("expected positive-penalty demerits 6100, got %d", got)
	}
	if got := calcDemerits(50, -50, params); got != 1100 {
		t.Fatalf("expected negative-penalty demerits 1100, got %d", got)
	}
}

func TestFirstPassAcceptsBadnessAtPreToleranceBoundary(t *testing.T) {
	params := NewKPDefaultParameters()
	params.LinePenalty = 0
	params.RightSkip = glue(0, 0, 0)
	khp := testKhipu()
	khp.W = append(khp.W, 7990, 7990, params.ParFillSkip.W)
	khp.MinW = append(khp.MinW, 7990, 7990, params.ParFillSkip.MinW)
	khp.MaxW = append(khp.MaxW, 8000, 7990, params.ParFillSkip.MaxW)
	khp.Penalty = append(khp.Penalty, -100, InfinityDemerits, InfinityMerits)
	khp.Kind = append(khp.Kind, khipu.KTTextBox, khipu.KTTextBox, khipu.KTGlue)
	khp.Pos = append(khp.Pos, 0, 0, 0)
	khp.Len = append(khp.Len, 0, 0, 0)
	khp.Flags = append(khp.Flags, 0, 0, khipu.KFDiscardable)

	breakpoints, quality, ok, err := breakParagraphPass(khp, linebreak.RectangularParShape(8000), params, false)
	if err != nil {
		t.Fatalf("first pass failed unexpectedly: %v", err)
	}
	if !ok {
		t.Fatalf("expected first pass to accept badness at PreTolerance boundary")
	}
	if want := []kinx{0, 2}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected breakpoints %v, got %v", want, breakpoints)
	}
	if quality.worstBadness != params.PreTolerance {
		t.Fatalf("expected worst badness %d, got %d", params.PreTolerance, quality.worstBadness)
	}
}

func TestEvaluateCandidatesSeparatesScreeningFromRanking(t *testing.T) {
	params := NewKPDefaultParameters()
	params.LinePenalty = 0
	params.LeftSkip = glue(0, 0, 0)
	params.RightSkip = glue(0, 0, 0)
	kp, err := prepareLineBreaker(linebreak.RectangularParShape(100), params)
	if err != nil {
		t.Fatalf("prepareLineBreaker failed: %v", err)
	}
	kp.paths[origin{kp.root, 0}] = bookkeeping{
		segment:     WSS{W: 100, Min: 100, Max: 100},
		seenContent: true,
	}
	kp.paths[origin{kp.root, 1}] = bookkeeping{
		segment:     WSS{W: 90, Min: 90, Max: 95},
		seenContent: true,
	}

	evals, ok := kp.evaluateCandidates(kp.root, 0)
	if !ok {
		t.Fatalf("expected at least one geometrically feasible line")
	}
	if len(evals) != 2 {
		t.Fatalf("expected 2 evaluations, got %d", len(evals))
	}
	var accepted, screenedOut int
	for _, ev := range evals {
		switch ev.disposition {
		case lineAccepted:
			accepted++
			if ev.demerits != 0 {
				t.Fatalf("expected accepted exact-fit candidate to have 0 demerits, got %d", ev.demerits)
			}
		case lineScreenedOut:
			screenedOut++
			if ev.badness <= params.PreTolerance {
				t.Fatalf("expected screened-out candidate badness above PreTolerance, got %d", ev.badness)
			}
		}
	}
	if accepted != 1 || screenedOut != 1 {
		t.Fatalf("expected one accepted and one screened-out candidate, got accepted=%d screenedOut=%d", accepted, screenedOut)
	}
}

func TestDiscretionaryDemeritsAddFirstLineExtra(t *testing.T) {
	params := NewKPDefaultParameters()
	params.LinePenalty = 0
	params.FirstHyphenDemerits = 37
	kp := newLinebreaker(nil, params)
	kp.root = PlainBreak(noinx)
	base := calcDemerits(12, 50, params)
	got := kp.discretionaryDemerits(12, 50, kp.root)
	if got != base+params.FirstHyphenDemerits {
		t.Fatalf("expected first-line discretionary demerits %d, got %d", base+params.FirstHyphenDemerits, got)
	}
	got = kp.discretionaryDemerits(12, 50, PlainBreak(3))
	if got != base {
		t.Fatalf("expected non-first-line discretionary demerits %d, got %d", base, got)
	}
}

func TestTerminalHyphenDemeritsApplyOnlyToFinalTransitionFromDiscretionary(t *testing.T) {
	params := NewKPDefaultParameters()
	params.FinalHyphenDemerits = 91
	kp := newLinebreaker(nil, params)
	kp.end = PlainBreak(9)
	if got := kp.terminalHyphenDemerits(BreakRef{At: 4, Variant: 1}, kp.end); got != params.FinalHyphenDemerits {
		t.Fatalf("expected final-line discretionary demerits %d, got %d", params.FinalHyphenDemerits, got)
	}
	if got := kp.terminalHyphenDemerits(PlainBreak(4), kp.end); got != 0 {
		t.Fatalf("expected ordinary predecessor not to trigger final-line discretionary demerits, got %d", got)
	}
	if got := kp.terminalHyphenDemerits(BreakRef{At: 4, Variant: 1}, PlainBreak(8)); got != 0 {
		t.Fatalf("expected non-terminal transition not to trigger final-line discretionary demerits, got %d", got)
	}
}

func dimenDU(n int) dimen.DU {
	return dimen.DU(n)
}
