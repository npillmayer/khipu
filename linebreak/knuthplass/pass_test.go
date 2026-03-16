package knuthplass

import (
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

func TestFirstPassUsesPreTolerance(t *testing.T) {
	params := NewKPDefaultParameters()
	kp := newLinebreaker(nil, params)
	if got := kp.badnessLimit(); got != params.PreTolerance {
		t.Fatalf("expected first-pass badness limit %d, got %d", params.PreTolerance, got)
	}
	kp.hyphenating = true
	if got := kp.badnessLimit(); got != params.Tolerance {
		t.Fatalf("expected second-pass badness limit %d, got %d", params.Tolerance, got)
	}
}

func TestCollectOptimalBreakpointsTracksWorstBadness(t *testing.T) {
	params := NewKPDefaultParameters()
	parfillnode := node{
		params.ParFillSkip.W,
		params.ParFillSkip.MinW,
		params.ParFillSkip.MaxW,
		InfinityMerits,
		khipu.KTGlue,
	}
	khp := newKhipu([]node{
		{w: 10, minw: 10, maxw: 10, p: InfinityMerits, kind: khipu.KTTextBox},
		{w: 80, minw: 80, maxw: 80, p: InfinityDemerits, kind: khipu.KTTextBox},
		parfillnode,
	})
	kp, err := prepareLineBreaker(linebreak.RectangularParShape(80*dimen.BP), params)
	if err != nil {
		t.Fatalf("prepareLineBreaker failed: %v", err)
	}
	if err := kp.constructBreakpointGraph(khp, kp.parshape, kp.params); err != nil {
		t.Fatalf("constructBreakpointGraph failed: %v", err)
	}
	breakpoints, quality, ok := kp.collectOptimalBreakpoints(kp.end)
	if !ok {
		t.Fatalf("expected optimal breakpoint path to exist")
	}
	if want := []kinx{0, 2}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected breakpoints %v, got %v", want, breakpoints)
	}
	if quality.worstBadness <= params.PreTolerance {
		t.Fatalf("expected worst badness above PreTolerance, got %d <= %d", quality.worstBadness, params.PreTolerance)
	}
}

func TestBreakParagraphStartsSecondPassAfterFailedPreTolerancePass(t *testing.T) {
	params := NewKPDefaultParameters()
	params.LinePenalty = 0
	params.RightSkip = glue(0, 0, 0)
	khp := testKhipu()
	khp.W = append(khp.W, 7990, 7990, params.ParFillSkip.W)
	khp.MinW = append(khp.MinW, 7990, 7990, params.ParFillSkip.MinW)
	khp.MaxW = append(khp.MaxW, 7999, 7990, params.ParFillSkip.MaxW)
	khp.Penalty = append(khp.Penalty, -100, InfinityDemerits, InfinityMerits)
	khp.Kind = append(khp.Kind, khipu.KTTextBox, khipu.KTTextBox, khipu.KTGlue)
	khp.Pos = append(khp.Pos, 0, 0, 0)
	khp.Len = append(khp.Len, 0, 0, 0)
	parshape := linebreak.RectangularParShape(8000)

	firstPassBreaks, quality, ok, err := breakParagraphPass(khp, parshape, params, false)
	if err != nil {
		t.Fatalf("first pass failed unexpectedly: %v", err)
	}
	if !ok || len(firstPassBreaks) == 0 {
		t.Fatalf("expected first pass to produce a fallback path, got ok=%v breakpoints=%v", ok, firstPassBreaks)
	}
	if want := []kinx{0, 2}; slices.Equal(firstPassBreaks, want) {
		t.Fatalf("expected first pass to miss the better breakpoint path, got %v", firstPassBreaks)
	}
	if quality.totalCost < AwfulDemerits {
		t.Fatalf("expected first-pass fallback to carry awful cost, got %+v", quality)
	}
	secondPassBreaks, _, ok, err := breakParagraphPass(khp, parshape, params, true)
	if err != nil {
		t.Fatalf("second pass failed unexpectedly: %v", err)
	}
	if !ok || len(secondPassBreaks) == 0 {
		t.Fatalf("expected second pass to find a path, got ok=%v breakpoints=%v", ok, secondPassBreaks)
	}

	breakpoints, err := BreakParagraph(khp, parshape, params)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{0, 2}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected second-pass breakpoints %v, got %v", want, breakpoints)
	}
}
