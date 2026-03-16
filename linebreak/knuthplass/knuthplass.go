package knuthplass

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

// linebreaker is an internal entity for K&P-linebreaking.
type linebreaker struct {
	*pathTable
	horizon     map[kinx]struct{}      // horizon of possible linebreaks
	paths       map[origin]bookkeeping // path(-partials) up to horizon
	params      *Parameters            // typesetting parameters relevant for line-breaking
	parshape    linebreak.ParShape     // target shape of the paragraph
	hyphenating bool                   // second pass may relax limits and use discretionaries
	root        kinx                   // "break" at start of paragraph
	end         kinx                   // "break" at end of paragraph
}

func pathsAsString(paths map[origin]bookkeeping) string {
	var sb strings.Builder
	os := slices.Collect(maps.Keys(paths))
	slices.SortFunc(os, func(a, b origin) int {
		d := cmp.Compare(a.from, b.from)
		if d != 0 {
			return d
		}
		return cmp.Compare(a.line, b.line)
	})
	for _, o := range os {
		b := paths[o]
		sb.WriteString(fmt.Sprintf("BP[%2d L:%2d] %v\n", o.from, o.line, b))
	}
	return sb.String()
}

func newLinebreaker(parshape linebreak.ParShape, params *Parameters) *linebreaker {
	kp := &linebreaker{}
	kp.pathTable = newPathTable()
	kp.horizon = make(map[kinx]struct{})
	kp.paths = make(map[origin]bookkeeping)
	kp.parshape = parshape
	if params == nil {
		params = NewKPDefaultParameters()
	}
	kp.params = params
	return kp
}

// NewKPDefaultParameters creates line-breaking parameters similar to
// (but not identical) to TeX's.
func NewKPDefaultParameters() *Parameters {
	return &Parameters{
		Tolerance:            200,
		PreTolerance:         100,
		LinePenalty:          10,
		HyphenPenalty:        50,
		ExHyphenPenalty:      50,
		DoubleHyphenDemerits: 2000,
		FinalHyphenDemerits:  10000,
		EmergencyStretch:     dimen.DU(dimen.BP * 20),
		LeftSkip:             glue(0, 0, 0),
		RightSkip:            glue(0, 0, 50*dimen.BP),
		ParFillSkip:          glue(0, 0, dimen.Fill),
	}
}

func prepareLineBreaker(parshape linebreak.ParShape,
	params *Parameters) (*linebreaker, error) {
	//
	if parshape == nil {
		return nil, ErrNoParShape
	}
	kp := newLinebreaker(parshape, params)
	kp.root = -1                                 // virtual starting knot
	kp.horizon[kp.root] = struct{}{}             // root is now officially a breakpoint
	kp.paths[origin{kp.root, 0}] = bookkeeping{} // start of every path
	return kp, nil
}

// --- Breakpoints -----------------------------------------------------------

// A feasible breakpoint is uniquely identified by a text position (mark).
// A break position may be selectable for different line counts, and we
// retain all of them. Different line-count paths usually will have different costs.
// We will hold some bookkeeping information to reflect active segments.
type feasibleBreakpoint struct {
	mark  khipu.Mark             // location of this breakpoint
	books map[int32]*bookkeeping // bookkeeping per linecount
}

type bookkeeping struct {
	segment      WSS    // sum of widths from this breakpoint up to current knot
	totalcost    merits // sum of costs for segment up to this breakpoint
	worstbadness merits // highest line badness encountered along the path
	leadingTrim  WSS    // discardable width before the first retained line item
	trailingTrim WSS    // discardable width after the most recent retained line item
	seenContent  bool   // does this segment contain a non-discardable content item?
}

func (path bookkeeping) String() string {
	return fmt.Sprintf("P(%.2f TC:%d MB:%d)", path.segment.W.Points(), path.totalcost, path.worstbadness)
}

type lineItemClass uint8

const (
	LICContent lineItemClass = iota
	LICTrimDiscardable
	LICRetainedNeutral
)

func classifyLineItem(idx, end kinx, k khipu.KnotCore) lineItemClass {
	if idx == end && k.Penalty <= InfinityMerits {
		return LICRetainedNeutral
	}
	switch k.Kind {
	case khipu.KTGlue, khipu.KTKern:
		return LICTrimDiscardable
	case khipu.KTTextBox, khipu.KTDiscretionary:
		return LICContent
	default:
		return LICRetainedNeutral
	}
}

func (book *bookkeeping) appendItem(cls lineItemClass, w WSS) {
	book.segment = book.segment.Add(w)
	switch cls {
	case LICTrimDiscardable:
		if book.seenContent {
			book.trailingTrim = book.trailingTrim.Add(w)
		} else {
			book.leadingTrim = book.leadingTrim.Add(w)
		}
	case LICContent:
		book.seenContent = true
		book.trailingTrim = WSS{}
	case LICRetainedNeutral:
		// width stays in segment, but does not contribute to trim bookkeeping
	}
}

func (book bookkeeping) effectiveWidth(params *Parameters) WSS {
	segw := book.segment
	segw = segw.Subtract(book.leadingTrim)
	segw = segw.Subtract(book.trailingTrim)
	w := WSS{}.SetFromKnot(params.LeftSkip)
	segw = segw.Add(w)
	w = WSS{}.SetFromKnot(params.RightSkip)
	segw = segw.Add(w)
	return segw
}

type cost struct {
	badness  merits // line badness used for pass screening
	demerits merits // line demerits used for path ranking
}

func (c cost) String() string {
	return fmt.Sprintf("C(b:%d d:%d)", c.badness, c.demerits)
}

type provisionalMark int64 // provisional mark from an integer position

func (m provisionalMark) Position() int64  { return int64(m) }
func (m provisionalMark) Knot() khipu.Knot { return khipu.PenaltyItem(-10000) }

func (kp *linebreaker) updatePath(bp, idx kinx, k khipu.KnotCore) {
	wss := WSS{}.SetFromKnot(k)             // get dimensions of knot
	cls := classifyLineItem(idx, kp.end, k) // classify for line-edge trimming
	for st, path := range kp.paths {        // TODO find a more efficient data-structure
		if st.from == bp { // found a path ending in `to`
			tracer().Debugf("   --- path of %s/%v = %v", knotInxStr(bp), st, path)
			path.appendItem(cls, wss)
			tracer().Debugf("K&R: extending segment from %s to %v", knotInxStr(bp), path.segment)
			kp.paths[st] = path
		}
	}
}

func (kp *linebreaker) badnessLimit() merits {
	if kp != nil && kp.hyphenating {
		return kp.params.Tolerance
	}
	return kp.params.PreTolerance
}

// --- Segments ---------------------------------------------------------

func (kp *linebreaker) evalNewSegment(from, to kinx, line lineNo, c cost) {
	if bp := kp.pathTable.Breakpoint(to); bp == noinx {
		kp.pathTable.AddBP(to)
	}
	// now sure that `to` is a breakpoint
	preSeg, ok := kp.paths[origin{from, line - 1}]
	tracer().Debugf("K&P prev seg = %v", preSeg)
	assert(ok, "K&P internal error: cannot append segment to non-existent sub-path")
	evalCost := preSeg.totalcost + c.demerits
	pred, hasPred := kp.pathTable.Pred(to, line)
	seg, ok := kp.paths[origin{to, line}]
	path := seg // `path` will be the new path, if cheaper than `seg`
	if !hasPred {
		assert(!ok, "K&P internal error: path with non-existent predecessor state exists")
	} else if pred.total <= evalCost {
		assert(ok, "K&P internal error: path for predecessor state does not exist")
		return // existing path is cheaper => do nothing
	} else {
		tracer().Debugf("K&P remove sub-optimal seg %v", origin{to, line})
		delete(kp.paths, origin{to, line}) // remove `seg` from paths
	}
	kp.pathTable.SetPred(from, to, c.demerits, evalCost, line)
	path.totalcost = evalCost
	path.worstbadness = max(preSeg.worstbadness, c.badness)
	kp.paths[origin{to, line}] = path
	tracer().Debugf(" paths:\n%s", pathsAsString(kp.paths))
	tracer().Debugf("new segment %s ---(C:%d|L:%d)---> %s", knotInxStr(from), c.demerits,
		line, knotInxStr(to))
}

// === Algorithms ============================================================

type lineCost struct {
	bp      kinx
	line    lineNo
	cost    cost
	stretch dimen.DU // stretch / shrink
}

func (lc lineCost) String() string {
	c := lc.cost.String()
	return fmt.Sprintf("LC{%s line=%d %s %.2f}", knotInxStr(lc.bp),
		lc.line, c, lc.stretch.Points())
}

type lineDisposition uint8

const (
	lineInfeasible lineDisposition = iota
	lineScreenedOut
	lineAccepted
)

type lineEvaluation struct {
	bp          kinx
	line        lineNo
	stretch     dimen.DU
	disposition lineDisposition
	badness     merits
	demerits    merits
}

func (ev lineEvaluation) accepted() bool {
	return ev.disposition == lineAccepted
}

func (ev lineEvaluation) screenedOut() bool {
	return ev.disposition == lineScreenedOut
}

func (ev lineEvaluation) lineCost() lineCost {
	return lineCost{
		bp:      ev.bp,
		line:    ev.line,
		stretch: ev.stretch,
		cost: cost{
			badness:  ev.badness,
			demerits: ev.demerits,
		},
	}
}

func (kp *linebreaker) evaluateCandidates(bp kinx, penalty khipu.Penalty) ([]lineEvaluation, bool) {
	canReach := false
	// find all paths ending at `bp`
	var result []lineEvaluation
	for st, path := range kp.paths { // TODO find a more efficient data-structure
		if st.from == bp { // found a path ending in `to`
			linelen := kp.parshape.LineLength(int32(st.line + 1))
			segwss := path.effectiveWidth(kp.params)
			stsh := absD(linelen - segwss.W) // stretch or shrink of glue in line
			tracer().Debugf("    +---%.2f--->    | %.2f", segwss.W.Points(), linelen.Points())
			ev := lineEvaluation{
				bp:          bp,
				line:        st.line,
				stretch:     stsh,
				disposition: lineInfeasible,
			}
			if b, ok := calcBadness(segwss, linelen); ok {
				canReach = true
				ev.badness = b
				if b <= kp.badnessLimit() {
					ev.disposition = lineAccepted
					ev.demerits = calcDemerits(b, penalty, kp.params)
				} else {
					ev.disposition = lineScreenedOut
				}
			}
			result = append(result, ev)
		}
	}
	return result, canReach
}

// calcAdjustmentRatio computes the TeX-style adjustment ratio for one line.
// Positive ratios mean stretching, negative ratios mean shrinking.
// The ratio is only computable if there is some stretch or shrink capacity.
func calcAdjustmentRatio(segwss WSS, linelen dimen.DU) (float64, bool) {
	if segwss.W == linelen {
		return 0, true
	}
	if segwss.W < linelen {
		available := segwss.Max - segwss.W
		if available <= 0 {
			return 0, false
		}
		return float64(linelen-segwss.W) / float64(available), true
	}
	available := segwss.W - segwss.Min
	if available <= 0 {
		return 0, false
	}
	return -float64(segwss.W-linelen) / float64(available), true
}

func calcBadness(segwss WSS, linelen dimen.DU) (merits, bool) {
	ratio, ok := calcAdjustmentRatio(segwss, linelen)
	if !ok {
		return 0, false
	}
	r := max(0.0, absFloat64(ratio))
	badness := merits(min(100.0, r*r*r) * 100.0)
	return badness, true
}

// calcDemerits computes line demerits from a precomputed badness value.
// Forced breaks are handled in constructBreakpointGraph and therefore reach
// this function with a neutralized penalty.
func calcDemerits(badness merits, penalty khipu.Penalty, params *Parameters) merits {
	p := clampPenalty(merits(penalty))
	q := params.LinePenalty + badness
	d := q * q
	p2 := p * p
	if p > 0 {
		d += p2
	} else if p < 0 {
		d -= p2
	}
	tracer().Debugf("    calculating demerits for p=%d, b=%d: d=%d", p, badness, d)
	return d
}

func demeritsString(d linebreak.Merits) string {
	return fmt.Sprintf("%d", d)
}

type pathQuality struct {
	totalCost    merits
	worstBadness merits
}

// penaltyAt iterates over all penalties, starting at the current cursor mark, and
// collects penalties, searching for the most significant one.
// Will return
//
//	-10000, if present
//	max(p1, p2, ..., pn) otherwise
//
// Returns the most significant penalty. Advances the cursor over all adjacent penalties.
// After this, the cursor mark may not reflect the position of the significant penalty.
func penaltyAt(cursor linebreak.Cursor) (khipu.PenaltyItem, khipu.Mark) {
	if cursor.Knot().Type() != khipu.KTPenalty {
		return khipu.PenaltyItem(linebreak.InfinityDemerits), cursor.Mark()
	}
	penalty := cursor.Knot().(khipu.PenaltyItem)
	ignore := false // final penalty found, ignore all other penalties
	knot, ok := cursor.Peek()
	for ok {
		if knot.Type() == khipu.KTPenalty {
			cursor.Next() // advance to next penalty
			if ignore {
				break // just skip over adjacent penalties
			}
			p := knot.(khipu.PenaltyItem)
			if linebreak.Merits(p.Demerits()) <= linebreak.InfinityMerits { // -10000 must break (like in TeX)
				penalty = p
				ignore = true
			} else if p.Demerits() > penalty.Demerits() {
				penalty = p
			}
			knot, ok = cursor.Peek() // now check next knot
		} else {
			ok = false
		}
	}
	p := khipu.PenaltyItem(linebreak.CapDemerits(linebreak.Merits(penalty.Demerits())))
	return p, cursor.Mark()
}

// --- Main API ---------------------------------------------------------

// BreakParagraph determines optimal linebreaks for a paragraph, depending on
// a given set of linebreaking parameters and the desired shape of the paragraph.
//
// Paragraphs may be broken with different line counts. Only one of these will be
// optimal, and BreakParagraph will return that.
//
// For a function to get solutions with different linecounts, see FindBreakpoints.
func BreakParagraph(khipu *khipu.Khipu, parshape linebreak.ParShape, params *Parameters) (
	[]kinx, error) {
	//
	if params == nil {
		params = NewKPDefaultParameters()
	}
	breakpoints, quality, ok, err := breakParagraphPass(khipu, parshape, params, false)
	if err != nil {
		return nil, err
	}
	if ok && len(breakpoints) > 0 &&
		quality.worstBadness <= params.PreTolerance &&
		quality.totalCost < AwfulDemerits {
		return breakpoints, nil
	}
	if ok && quality.totalCost >= AwfulDemerits {
		tracer().Debugf("K&P: first-pass best path is overfull fallback (%d), retrying with hyphenation enabled",
			quality.totalCost)
	} else if ok && quality.worstBadness > params.PreTolerance {
		tracer().Debugf("K&P: first-pass best path exceeds PreTolerance (%d > %d)",
			quality.worstBadness, params.PreTolerance)
	} else if !ok {
		tracer().Debugf("K&P: first pass found no acceptable path, retrying with hyphenation enabled")
	}
	breakpoints, _, ok, err = breakParagraphPass(khipu, parshape, params, true)
	if err != nil {
		return nil, err
	}
	if !ok || len(breakpoints) == 0 {
		return nil, ErrNoBreakpoints
	}
	return breakpoints, nil
}

func breakParagraphPass(khp *khipu.Khipu, parshape linebreak.ParShape,
	params *Parameters, hyphenating bool) ([]kinx, pathQuality, bool, error) {
	kp, err := prepareLineBreaker(parshape, params)
	if err != nil {
		return nil, pathQuality{}, false, err
	}
	kp.hyphenating = hyphenating
	if err := kp.constructBreakpointGraph(khp, parshape, params); err != nil {
		tracer().Errorf("K&P: %w", err)
		return nil, pathQuality{}, false, err
	}
	breakpoints, quality, ok := kp.collectOptimalBreakpoints(kp.end)
	return breakpoints, quality, ok, nil
}

// constructBreakpointGraph is the central algorithm, akin to the paragraph breaking
// algorithm described by Knuth & Plass for the TeX typesetting system.
//
// The central data type is a feasible breakpoint (FB). An fb is a potential line breaking
// opportunity, which carries a certain cost. For all FBs considered, the cost is below a
// certain threshold (configured by the linebreaking-parameters). The task of the algorithm
// is to fit a sequence of FBs which produce the least cost overall.
//
// A khipu.Cursor moves over the knots in the input khipu, consisting of such things
// as text, glue, and penalties. Lines can potentially be broken at penalities.
// The algorithm maintains a set of active feasible linebreaks, called horizon. These
// FBs are inspected in turn and tested for a potential line between the FB and the
// location of the cursor. If such a line is not too costly, a new FB is constructed and
// appended to horizon. Other FBs, which can no longer be the start of any new potential
// line, are removed from horizon.
//
// The above operations contruct a DAG, starting from a single node representing the
// start of the paragraph, to a single node representing the end.
func (kp *linebreaker) constructBreakpointGraph(khipu *khipu.Khipu, parshape linebreak.ParShape,
	params *Parameters) error {
	//
	if len(khipu.W) == 0 {
		return ErrNoBreakpoints
	}
	kp.end = len(khipu.W) - 1
	//var last kinx // will hold last position within input khipu
	//var fb kinx // will hold feasible breakpoint from horizon
	//for cursor.Next() { // outer loop over input knots
	for last := range len(khipu.W) {
		k := khipu.KnotByIndex(last)
		//last = cursor.Mark() // we will need the last knot at the end of the loop
		//tracer().Debugf("_______________ %d/%v ___________________", last.Position(), last.Knot())
		tracer().Debugf("_______________ %d/%v ___________________", last, khipu.KnotByIndex(last))
		assert(len(kp.horizon) > 0, "K&P: no more active breakpoints, but input available")
		tracer().Debugf("horizon: %v", elements(kp.horizon))
		// if fb = kp.horizon.first(); fb == nil {
		// 	panic("no more active breakpoints, but input available") // TODO remove after debugging
		// }
		// --- main loop over active breakpoints in horizon ------------
		horizon := slices.Sorted(maps.Keys(kp.horizon))
		tracer().Debugf("horizon: %v", horizon)
		//for horiz_bp := range kp.horizon { // loop over active feasible breakpoints of horizon
		for _, horiz_bp := range horizon { // loop over active feasible breakpoints of horizon
			//for fb != nil { // loop over active feasible breakpoints of horizon
			//tracer().Debugf("      --- %s (in horizon) --> %v", knotInxStr(horiz_bp), khipu.KnotByIndex(last))
			tracer().Debugf("   --- %s (in horizon) --> candidate %v", knotInxStr(horiz_bp), knotString(last, khipu))
			kp.updatePath(horiz_bp, last, k)   // now WSS extends to new knot k
			if k.Penalty >= InfinityDemerits { // merits prohibit break
				tracer().Debugf("   --- break prohibited (p=%d)", k.Penalty)
				continue
			}
			costPenalty := k.Penalty
			if costPenalty <= InfinityMerits {
				costPenalty = 0
			}
			evals, stillreachable := kp.evaluateCandidates(horiz_bp, costPenalty)
			tracer().Debugf("   %s reachable with evals=%v", knotInxStr(last), evals)
			if stillreachable { // yes, position may have been reached in this iteration
				for _, ev := range evals {
					if ev.disposition == lineInfeasible {
						continue
					}
					c := ev.lineCost()
					tracer().Debugf("   check reachable segm line-costs %s", c.String())
					if k.Penalty <= InfinityMerits { // merits cause forced break
						if c.cost.badness > kp.badnessLimit() {
							tracer().Infof("K&P: znderfull box at line %d, b=%d, d=%d",
								c.line+1, c.cost.badness, c.cost.demerits)
						}
						kp.evalNewSegment(horiz_bp, last, c.line+1, c.cost)
						//newfb := kp.newFeasibleLine(fb, cursor.Mark(), cost.demerits, linecnt+1)
						//kp.horizon.Add(newfb) // make forced break member of horizon n+1
						kp.horizon[last] = struct{}{} // make forced break member of horizon n+1
					} else if ev.accepted() { // happy case: new breakpoint is feasible
						//
						kp.evalNewSegment(horiz_bp, last, c.line+1, c.cost)
						//newfb := kp.newFeasibleLine(fb, cursor.Mark(), cost.demerits, linecnt+1)
						//kp.horizon.Add(newfb) // make new breakpoint member of horizon n+1
						kp.horizon[last] = struct{}{} // make new breakpoint member of horizon n+1
					}
				}
			} else { // no longer reachable => check against draining of horizon
				if len(kp.horizon) <= 1 { // oops, low on options
					for _, ev := range evals {
						if ev.disposition != lineInfeasible {
							continue
						}
						c := ev.lineCost()
						tracer().Infof("Overfull box at line %d, cost=%d", c.line+1, AwfulDemerits)
						c.cost.demerits = AwfulDemerits
						kp.evalNewSegment(horiz_bp, last, c.line+1, c.cost)
						kp.horizon[last] = struct{}{}
					}
					// for linecnt := range costs {
					// 	tracer().Infof("Overfull box at line %d, cost=10000", linecnt+1)
					// 	newfb := kp.newFeasibleLine(fb, cursor.Mark(), linebreak.InfinityDemerits, linecnt+1)
					// 	kp.horizon.Add(newfb) // make new fb member of horizon n+1
					// 	if newfb.mark.Position() == fb.mark.Position() {
					// 		panic("THIS SHOULD NOT HAPPEN ?!?")
					// 	}
					// }
				}
				delete(kp.horizon, horiz_bp) // no longer valid in horizon
				//kp.horizon.Remove(fb) // no longer valid in horizon
			}

			//fb.UpdateSegmentBookkeeping(cursor.Mark())
			// Breakpoints are allowed at penalties only
			/*
				if cursor.Mark().Knot().Type() == khipu.KTPenalty { // TODO discretionaries
					var penalty khipu.PenaltyItem
					penalty, last = penaltyAt(cursor) // find correct p, if more than one
					costs, stillreachable := fb.calculateCostsTo(penalty, parshape, kp.params)
					if stillreachable { // yes, position may have been reached in this iteration
						for linecnt, cost := range costs { // check for every linecount alternative
							if linebreak.Merits(penalty.Demerits()) <= linebreak.InfinityMerits { // forced break
								if cost.badness > kp.params.Tolerance {
									tracer().Infof("Underfull box at line %d, b=%d, d=%d", linecnt+1, cost.badness, cost.demerits)
								}
								newfb := kp.newFeasibleLine(fb, cursor.Mark(), cost.demerits, linecnt+1)
								kp.horizon.Add(newfb) // make forced break member of horizon n+1
							} else if cost.badness < kp.params.Tolerance &&
								cost.demerits < linebreak.InfinityDemerits { // happy case: new breakpoint is feasible
								//
								newfb := kp.newFeasibleLine(fb, cursor.Mark(), cost.demerits, linecnt+1)
								kp.horizon.Add(newfb) // make new breakpoint member of horizon n+1
							}
						}
					} else { // no longer reachable => check against draining of horizon
						if kp.horizon.Size() <= 1 { // oops, low on options
							for linecnt := range costs {
								tracer().Infof("Overfull box at line %d, cost=10000", linecnt+1)
								newfb := kp.newFeasibleLine(fb, cursor.Mark(), linebreak.InfinityDemerits, linecnt+1)
								kp.horizon.Add(newfb) // make new fb member of horizon n+1
								if newfb.mark.Position() == fb.mark.Position() {
									panic("THIS SHOULD NOT HAPPEN ?!?")
								}
							}
						}
						kp.horizon.Remove(fb) // no longer valid in horizon
					}
				}
			*/
			//fb = kp.horizon.next()
		} // --- end of main loop over horizon ----------------------
	} // end of outer loop over input knots
	tracer().Infof("Collected %d potential breakpoints for paragraph", len(kp.feasBP))
	tracer().Infof("          %v", kp.feasBP)
	tracer().Infof("          predecessors:\n%v", kp.String())
	tracer().Infof("          paths:\n%s", pathsAsString(kp.paths))
	// fb = kp.findBreakpointAtMark(last)
	// if fb == nil {
	// for now panic, for debugging purposes
	//panic("last breakpoint not found") // khipu didn't end with penalty -10000
	// TODO add fb(-10000) and connect to last horizon
	// in this situation, input is drained but horizon is not ?!
	// }
	// kp.end = fb // remember last breakpoint of paragraph
	return nil
}

// collectOptimalBreakpoints walks the predecessor table backwards from the
// paragraph terminus and returns the single cheapest breakpoint sequence.
func (kp *linebreaker) collectOptimalBreakpoints(end kinx) ([]kinx, pathQuality, bool) {
	bestLine := lineNo(0)
	bestQuality := pathQuality{}
	found := false
	for st, path := range kp.paths {
		if st.from != end {
			continue
		}
		if !found || path.totalcost < bestQuality.totalCost {
			bestLine = st.line
			bestQuality = pathQuality{
				totalCost:    path.totalcost,
				worstBadness: path.worstbadness,
			}
			found = true
		}
	}
	if !found {
		return nil, pathQuality{}, false
	}
	breaks := make([]kinx, 0, bestLine)
	cur := end
	line := bestLine
	for cur != kp.root {
		breaks = append(breaks, cur)
		pred, ok := kp.pathTable.Pred(cur, line)
		if !ok {
			return nil, pathQuality{}, false
		}
		cur = pred.from
		line--
	}
	slices.Reverse(breaks)
	return breaks, bestQuality, true
}

// --- Helpers ----------------------------------------------------------

func absD(n dimen.DU) dimen.DU {
	if n < 0 {
		return -n
	}
	return n
}

func abs(n merits) merits {
	if n < 0 {
		return -n
	}
	return n
}

func absFloat64(n float64) float64 {
	if n < 0 {
		return -n
	}
	return n
}

func insert(s []int32, i int, n int32) []int32 {
	s = append(s, 0)
	copy(s[i+1:], s[i:])
	s[i] = n
	return s
}

func elements[T comparable](m map[T]struct{}) string {
	var sb strings.Builder
	sb.WriteRune('{')
	i := 0
	for k := range m {
		if i > 0 {
			sb.WriteRune(' ')
		}
		switch v := any(k).(type) {
		case kinx:
			sb.WriteString(knotInxStr(kinx(v)))
		default:
			sb.WriteString(fmt.Sprintf("%v", k))
		}
		//sb.WriteString(fmt.Sprintf("%v", k))
		i += 1
	}
	sb.WriteRune('}')
	return sb.String()
}

func knotInxStr(k kinx) string {
	if k < 0 {
		return "START"
	}
	return fmt.Sprintf("k-%d", k)
}

func knotString(n kinx, khp *khipu.Khipu) string {
	kstr := knotInxStr(n)
	knot := khp.KnotByIndex(n)
	return fmt.Sprintf("knot[%s]%s)", kstr, knot)

}
