package knuthplass

import (
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestGraphAddEdge(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	g := newGraph()
	g.AddBP(3)
	g.AddBP(5)
	g.AddBP(8)
	g.AddEdge(3, 5, 100, 100, 1)
	g.AddEdge(3, 5, 200, 100, 2)
	g.AddEdge(3, 8, 100, 100, 1)
	g.AddEdge(5, 8, 100, 100, 3)
	if bps := g.To(8); len(bps) != 2 {
		t.Errorf("expected 2 breakpoints, got %d", len(bps))
	}
	c, ok := g.Cost(3, 5, 2)
	if !ok {
		t.Errorf("expected cost for edge 3->5, line 2")
	}
	if c != 200 {
		t.Errorf("expected cost 200, got %d", c)
	}
	if e := g.Edge(1, 5, 1); e != nullEdge {
		t.Errorf("expected null edge for non-existent edge 1->5, line 1")
	}
}

func TestGraphRemoveEdge(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "khipu.linebreak")
	defer teardown()
	//
	g := newGraph()
	g.AddBP(3)
	g.AddBP(5)
	g.AddBP(8)
	g.AddEdge(3, 5, 100, 100, 1)
	g.AddEdge(3, 5, 200, 100, 2)
	g.AddEdge(3, 8, 100, 100, 1)
	g.AddEdge(5, 8, 100, 100, 3)
	g.RemoveEdge(3, 5, 2)
	if e := g.Edge(3, 5, 2); e != nullEdge {
		t.Errorf("expected null edge for removed edge 3->5, line 2")
	}
	g.RemoveEdge(3, 8, 1)
	if bps := g.To(8); len(bps) != 1 {
		t.Errorf("expected 1 edge, got %d", len(bps))
	}
}
