package knuthplass

import (
	"slices"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

// Projection convenience for fixtures interested in indices rather than variants.
// Each fixture specifies finality independently of discretionary discovery.
func breakParagraphPass(khp *khipu.Khipu, layout khipu.Params,
	params *Parameters, pass passConfig) ([]kinx, pathQuality, bool, error) {
	result, err := breakParagraphPassRefs(khp, layout, params, pass)
	return projectBreakRefs(result.breaks), result.quality, result.complete, err
}

func TestFirstPassUsesPreTolerance(t *testing.T) {
	params := NewDefaultParameters()
	kp := newLinebreaker(DefaultLayout(nil), params)
	if got := kp.badnessLimit(); got != params.PreTolerance {
		t.Fatalf("expected first-pass badness limit %d, got %d", params.PreTolerance, got)
	}
	kp.hyphenating = true
	if got := kp.badnessLimit(); got != params.Tolerance {
		t.Fatalf("expected second-pass badness limit %d, got %d", params.Tolerance, got)
	}
}

func TestForcedBreakFallbacksKeepPositiveAccumulatedCost(t *testing.T) {
	params := NewDefaultParameters()
	layout := khipu.Params{LineShape: linebreak.RectangularParShape(80 * dimen.BP)}
	khp := newKhipu([]node{
		{w: 10, minw: 10, maxw: 10, p: InfinityMerits, kind: khipu.KTTextBox},
		{w: 10, minw: 10, maxw: 10, p: InfinityMerits, kind: khipu.KTTextBox},
		{w: 10, minw: 10, maxw: 10, p: InfinityMerits, kind: khipu.KTTextBox},
		{maxw: dimen.Fill, p: InfinityMerits, kind: khipu.KTGlue},
	})
	breaks, quality, ok, err := breakParagraphPass(khp, layout, params, passConfig{id: passTolerance, hyphenating: true, final: true})
	if err != nil || !ok || !slices.Equal(breaks, []int{0, 1, 2, 3}) {
		t.Fatalf("forced breaks=%v ok=%v err=%v", breaks, ok, err)
	}
	if quality.totalCost < 3*AwfulDemerits {
		t.Fatalf("fallback costs overflowed: %+v", quality)
	}
}

func TestCollectOptimalBreakpointsTracksWorstBadness(t *testing.T) {
	params := NewDefaultParameters()
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
	kp, err := prepareLineBreaker(DefaultLayout(linebreak.RectangularParShape(80*dimen.BP)), params)
	if err != nil {
		t.Fatalf("prepareLineBreaker failed: %v", err)
	}
	kp.final = true // exercise worst-badness collection through a final fallback
	if err := kp.constructBreakpointGraph(khp); err != nil {
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
	params := NewDefaultParameters()
	params.LinePenalty = 0
	khp := testKhipu()
	khp.W = append(khp.W, 7990, 7990, params.ParFillSkip.W)
	khp.MinW = append(khp.MinW, 7990, 7990, params.ParFillSkip.MinW)
	khp.MaxW = append(khp.MaxW, 7999, 7990, params.ParFillSkip.MaxW)
	khp.Penalty = append(khp.Penalty, -100, InfinityDemerits, InfinityMerits)
	khp.Kind = append(khp.Kind, khipu.KTTextBox, khipu.KTTextBox, khipu.KTGlue)
	khp.Pos = append(khp.Pos, 0, 0, 0)
	khp.Len = append(khp.Len, 0, 0, 0)
	khp.Flags = append(khp.Flags, 0, 0, 0) // paragraph fill is retained
	parshape := linebreak.RectangularParShape(8000)
	layout := khipu.Params{LineShape: parshape}

	firstPassBreaks, _, ok, err := breakParagraphPass(khp, layout, params, passConfig{id: passPreTolerance})
	if err != nil {
		t.Fatalf("first pass failed unexpectedly: %v", err)
	}
	if ok || len(firstPassBreaks) != 0 {
		t.Fatalf("expected ordinary first-pass failure without fallback, got ok=%v breaks=%v", ok, firstPassBreaks)
	}
	secondPassBreaks, _, ok, err := breakParagraphPass(khp, layout, params, passConfig{id: passTolerance, hyphenating: true, final: true})
	if err != nil {
		t.Fatalf("second pass failed unexpectedly: %v", err)
	}
	if !ok || len(secondPassBreaks) == 0 {
		t.Fatalf("expected second pass to find a path, got ok=%v breakpoints=%v", ok, secondPassBreaks)
	}

	breakpoints, err := BreakParagraph(khp, layout, params)
	if err != nil {
		t.Fatalf("BreakParagraph failed: %v", err)
	}
	if want := []kinx{0, 2}; !slices.Equal(breakpoints, want) {
		t.Fatalf("expected second-pass breakpoints %v, got %v", want, breakpoints)
	}
}
