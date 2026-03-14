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

// ---------------------------------------------------------------------------

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
	}
	return &khipu
}
