package knuthplass

import "slices"

type kinx = int   // khipu knot index
type merits int16 // (de-)merits for a line break
type lineNo int   // line number

const noinx = -1

type origin struct {
	from kinx
	line lineNo
}

type edge struct {
	from, to    kinx // this is an edge between two khipu-knots
	cost, total merits
	lineno      lineNo
}

// nullEdge denotes an edge that is not present in a graph.
var nullEdge = edge{}

// isNull checks if an edge is null, i.e. non-existent.
func (e edge) isNull() bool {
	return e == nullEdge
}

type graph struct {
	feasBP      map[kinx]struct{}        // set of feasible breakpoints, as indices into the khipu
	edgesTo     map[kinx]map[origin]edge // edge to, from, with line-number
	prunedEdges map[kinx]map[origin]edge // we keep deleted edges around
}

func newGraph() *graph {
	return &graph{
		feasBP:      make(map[kinx]struct{}),
		edgesTo:     make(map[kinx]map[origin]edge),
		prunedEdges: make(map[kinx]map[origin]edge),
	}
}

// newWEdge returns a new weighted edge from one breakpoint to another,
// given two breakpoints and a label-key.
// It is not yet inserted into a graph.
func newWEdge(from, to kinx, cost, total merits, line lineNo) edge {
	// if from.books[linecnt-1] == nil {
	// 	panic(fmt.Errorf("startpoint of new line %d seems to have incorrent books: %v", linecnt, from))
	// }
	// if to.books[linecnt] == nil {
	// 	panic(fmt.Errorf("endpoint of new line %d seems to have incorrent books: %v", linecnt, to))
	// }
	return edge{
		from:   from,
		to:     to,
		cost:   cost,
		total:  total,
		lineno: line,
	}
}

// AddBP adds a feasible breakpoint to the graph.
// It returns an error if the breakpoint is already present.
func (g *graph) AddBP(bp kinx) error {
	tracer().Debugf("adding breakpoint at %d", bp)
	//tracer().Debugf("Added new breakpoint at %d/%v", fb.mark.Position(), fb.mark.Knot())
	// if _, exists := g.nodes[fb.mark.Position()]; exists {
	// 	return fmt.Errorf("Breakpoint at position %d already known", fb.mark.Position())
	// }
	// g.nodes[fb.mark.Position()] = fb
	// return nil
	if _, ok := g.feasBP[bp]; ok {
		tracer().Errorf("Breakpoint at position %d already known", bp)
	}
	g.feasBP[bp] = struct{}{}
	return nil
}

// Edge returns the edge (from,to), if such an edge exists,
// otherwise it returns nullEdge.
// The to-node must be directly reachable from the from-node.
func (g *graph) Edge(from, to kinx, line lineNo) edge {
	if edge, ok := g.edgesTo[to][origin{from, line}]; ok {
		return edge
	}
	// if edge, ok := edges[linecnt]; ok {
	// 	return edge
	// }
	return nullEdge
}

func (g *graph) StartOfEdge(edge edge) kinx {
	if _, ok := g.feasBP[edge.from]; ok {
		return edge.from
	}
	return noinx
}

// Breakpoint returns the feasible breakpoint at the given position if it exists in the graph,
// and nil otherwise.
func (g *graph) Breakpoint(bp kinx) kinx {
	if _, ok := g.feasBP[bp]; ok {
		return bp
	}
	return noinx
}

// RemoveEdge removes the edge between two breakpoints for a linecount.
// The breakpoints are not deleted from the graph.
// If the edge does not exist, this is a no-op.
//
// Deleted edges are conserved and may be collected with g.Edges(true).
func (g *graph) RemoveEdge(from, to kinx, line lineNo) {
	var ok bool
	if _, ok = g.feasBP[from]; !ok {
		return
	} else if _, ok = g.feasBP[to]; !ok {
		return
	}
	var e edge
	if e = g.Edge(from, to, line); e.isNull() {
		return
	}
	var prunedTo map[origin]edge
	if prunedTo, ok = g.prunedEdges[to]; !ok {
		g.prunedEdges[to] = make(map[origin]edge)
		prunedTo = g.prunedEdges[to]
	}
	prunedTo[origin{from, line}] = e
	delete(g.edgesTo[to], origin{from, line})
	if len(g.edgesTo[to]) == 0 {
		delete(g.edgesTo, to)
	}

	// if edgesFrom, ok := g.edgesTo[to.mark.Position()]; ok {
	// 	if edges, ok := edgesFrom[from.mark.Position()]; ok {
	// 		if e, ok := edges[linecnt]; ok { // edge exists => move it to prunedEdges
	// 			if t, ok := g.prunedEdges[to.mark.Position()]; ok { // 'to' dict exists
	// 				edges := t[from.mark.Position()]
	// 				if edges == nil {
	// 					edges = make(map[int32]wEdge)
	// 					t[from.mark.Position()] = edges
	// 				}
	// 				edges[linecnt] = e
	// 			} else {
	// 				//edges := map[int32]wEdge{linecnt: e}
	// 				g.prunedEdges[to.mark.Position()] = map[int64]map[int32]wEdge{from.mark.Position(): edges}
	// 				//g.prunedEdges[to.mark.Position()][from.mark.Position()][linecnt] = e
	// 			}
	// 		}
	// 		delete(edges, linecnt)
	// 		if len(edges) == 0 {
	// 			delete(edgesFrom, to.mark.Position())
	// 		}
	// 	}
	// }
}

// AddEdge adds a weighted edge from one node to another. Endpoints which are
// not yet contained in the graph are added.
// Does nothing if from=to.
func (g *graph) AddEdge(from, to kinx, cost, total merits, line lineNo) {
	if from == to {
		return
	}
	// if from.mark.Position() == to.mark.Position() {
	// 	return
	// }
	g.AddBP(to)
	// if g.Breakpoint(to.mark.Position()) == nil {
	// 	g.Add(to)
	// }
	if !g.Edge(from, to, line).isNull() {
		return
	}
	e := newWEdge(from, to, cost, total, line)
	var ok bool
	var edgesTo map[origin]edge
	if edgesTo, ok = g.edgesTo[to]; !ok {
		g.edgesTo[to] = make(map[origin]edge)
		edgesTo = g.edgesTo[to]
	}
	edgesTo[origin{from, line}] = e

	// if g.Edge(from, to, linecnt).isNull() {
	// 	edge := newWEdge(from, to, cost, total, linecnt)
	// 	if t, ok := g.edgesTo[to.mark.Position()]; ok {
	// 		edges := t[from.mark.Position()]
	// 		if edges == nil {
	// 			edges = make(map[int32]wEdge)
	// 			t[from.mark.Position()] = edges
	// 		}
	// 		edges[linecnt] = edge
	// 	} else {
	// 		edges := map[int32]wEdge{linecnt: edge}
	// 		g.edgesTo[to.mark.Position()] = map[int64]map[int32]wEdge{from.mark.Position(): edges}
	// 	}
	// }
}

// To returns all breakpoints in g that can reach directly to a breakpoint given by
// a position. The returned breakpoints are sorted by position.
func (g *graph) To(bp kinx) []kinx {
	var ok bool
	var edgesTo map[origin]edge
	if edgesTo, ok = g.edgesTo[bp]; !ok {
		return []kinx{}
	}
	breakpoints := make([]kinx, 0, len(edgesTo))
	for orgn := range edgesTo {
		breakpoints = append(breakpoints, orgn.from)
	}
	slices.Sort(breakpoints)
	return breakpoints

	// position := fb.mark.Position()
	// if _, ok := g.edgesTo[position]; !ok || len(g.edgesTo[position]) == 0 {
	// 	return noBreakpoints
	// }
	// breakpoints := make([]*feasibleBreakpoint, len(g.edgesTo[position]))
	// i := 0
	// for pos := range g.edgesTo[position] {
	// 	breakpoints[i] = g.nodes[pos]
	// 	i++
	// }
	// sort.Sort(breakpointSorter{breakpoints})
	// return breakpoints
}

// Cost returns the cost for the edge between two breakpoints, valid for a lineNo.
// If from and to are the same node or if there is no edge (from,to),
// a pseudo-label with infinite cost is returned.
// Cost returns true if an edge (from,to) exists, false otherwise.
func (g *graph) Cost(from, to kinx, line lineNo) (merits, bool) {
	if from == to {
		return infinityDemerits, false
	}
	if e := g.Edge(from, to, line); e != nullEdge {
		return e.cost, true
	}
	// if edgesFrom, ok := g.edgesTo[to.mark.Position()]; ok {
	// 	if edges, ok := edgesFrom[from.mark.Position()]; ok {
	// 		if edge, ok := edges[linecnt]; ok {
	// 			return edge.cost, true
	// 		}
	// 	}
	// }
	return infinityDemerits, false
}
