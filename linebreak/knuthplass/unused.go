package knuthplass

import "github.com/emirpasic/gods/sets/hashset"

// --- Horizon (active Nodes) ------------------------------------------------

type activeFBP struct {
	*hashset.Set               // a set of feasible breakpoints
	values       []interface{} // holds breakpoints during iteration
	iterinx      int           // current iteration index
}

// constructor
func newActiveFeasibleBreakpoints() *activeFBP {
	set := hashset.New()
	afb := &activeFBP{set, nil, -1}
	return afb
}

// first starts iteration over the feasible breakpoints of the current horizon.
func (h *activeFBP) first() *feasibleBreakpoint {
	var fb *feasibleBreakpoint // return value
	//fmt.Printf("horizon: there are %d active FBs\n", h.Size())
	if h.Size() > 0 {
		h.values = h.Values() // get set values as list, unsorted
		fb = h.values[0].(*feasibleBreakpoint)
		h.iterinx = 1
	}
	return fb
}

// next gets the next feasible breakpoints of the current horizon.
func (h *activeFBP) next() *feasibleBreakpoint {
	var fb *feasibleBreakpoint
	if h.values != nil && h.iterinx < len(h.values) {
		fb = h.values[h.iterinx].(*feasibleBreakpoint)
		h.iterinx++
	}
	return fb
}
