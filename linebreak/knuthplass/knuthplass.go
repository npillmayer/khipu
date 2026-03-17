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
	horizon     map[BreakRef]struct{}  // horizon of possible linebreaks
	paths       map[origin]bookkeeping // path(-partials) up to horizon
	seeds       map[origin]bookkeeping // immutable seed state for rescanning a segment
	params      *Parameters            // typesetting parameters relevant for line-breaking
	parshape    linebreak.ParShape     // target shape of the paragraph
	hyphenating bool                   // second pass may relax limits and use discretionaries
	root        BreakRef               // "break" at start of paragraph
	end         BreakRef               // "break" at end of paragraph
}

func pathsAsString(paths map[origin]bookkeeping) string {
	var sb strings.Builder
	os := slices.Collect(maps.Keys(paths))
	slices.SortFunc(os, func(a, b origin) int {
		d := compareBreakRef(a.from, b.from)
		if d != 0 {
			return d
		}
		return cmp.Compare(a.line, b.line)
	})
	for _, o := range os {
		b := paths[o]
		sb.WriteString(fmt.Sprintf("BP[%s L:%2d] %v\n", o.from.String(), o.line, b))
	}
	return sb.String()
}

func newLinebreaker(parshape linebreak.ParShape, params *Parameters) *linebreaker {
	kp := &linebreaker{}
	kp.pathTable = newPathTable()
	kp.horizon = make(map[BreakRef]struct{})
	kp.paths = make(map[origin]bookkeeping)
	kp.seeds = make(map[origin]bookkeeping)
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
		FirstHyphenDemerits:  0,
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
	kp.root = PlainBreak(noinx)                  // virtual starting knot
	kp.horizon[kp.root] = struct{}{}             // root is now officially a breakpoint
	kp.paths[origin{kp.root, 0}] = bookkeeping{} // start of every path
	kp.seeds[origin{kp.root, 0}] = bookkeeping{}
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

// classifyLineItem derives line-edge trimming behavior entirely from knot
// flags and visible-content kind. The paragraph-final ParFillSkip node must be
// guaranteed to arrive without KFDiscardable set; otherwise it would be
// trimmed like ordinary interword space.
func classifyLineItem(k khipu.KnotCore) lineItemClass {
	if k.IsDiscardable() {
		return LICTrimDiscardable
	}
	switch k.Kind {
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

func (kp *linebreaker) updatePath(bp BreakRef, idx kinx, k khipu.KnotCore) {
	if bp.IsDiscretionary() && idx <= bp.At {
		return
	}
	wss := WSS{}.SetFromKnot(k)      // get dimensions of knot
	cls := classifyLineItem(k)       // classify for line-edge trimming
	for st, path := range kp.paths { // TODO find a more efficient data-structure
		if st.from == bp { // found a path ending in `to`
			tracer().Debugf("   --- path of %s/%v = %v", bp.String(), st, path)
			path.appendItem(cls, wss)
			tracer().Debugf("K&R: extending segment from %s to %v", bp.String(), path.segment)
			kp.paths[st] = path
		}
	}
}

// badnessLimit returns the active screening threshold for the current pass.
// Pass 1 uses PreTolerance; pass 2 relaxes this to Tolerance once discretionary
// breaking is enabled.
func (kp *linebreaker) badnessLimit() merits {
	if kp != nil && kp.hyphenating {
		return kp.params.Tolerance
	}
	return kp.params.PreTolerance
}

// terminalHyphenDemerits applies the "last line" discretionary extra at the
// only point where it can be known cheaply: when a transition reaches the final
// forced break of the paragraph. In that situation, the predecessor break is the
// break that created the last line.
func (kp *linebreaker) terminalHyphenDemerits(from, to BreakRef) merits {
	if kp == nil || kp.params == nil {
		return 0
	}
	if to == kp.end && from.IsDiscretionary() {
		return kp.params.FinalHyphenDemerits
	}
	return 0
}

// --- Segments ---------------------------------------------------------

func (kp *linebreaker) evalNewSegment(from, to BreakRef, line lineNo, c cost) {
	kp.evalNewSegmentSeed(from, to, line, c, bookkeeping{})
}

func (kp *linebreaker) evalNewSegmentSeed(from, to BreakRef, line lineNo, c cost, seed bookkeeping) {
	if bp := kp.pathTable.Breakpoint(to); bp == NoBreakRef {
		kp.pathTable.AddBP(to)
	}
	// now sure that `to` is a breakpoint
	preSeg, ok := kp.paths[origin{from, line - 1}]
	tracer().Debugf("K&P prev seg = %v", preSeg)
	assert(ok, "K&P internal error: cannot append segment to non-existent sub-path")
	evalCost := preSeg.totalcost + c.demerits
	pred, hasPred := kp.pathTable.Pred(to, line)
	_, ok = kp.paths[origin{to, line}]
	path := seed.segmentState() // `path` will be the new path, if cheaper than `seg`
	if !hasPred {
		assert(!ok, "K&P internal error: path with non-existent predecessor state exists")
	} else if pred.total <= evalCost {
		assert(ok, "K&P internal error: path for predecessor state does not exist")
		return // existing path is cheaper => do nothing
	} else {
		tracer().Debugf("K&P remove sub-optimal seg %v", origin{to, line})
		delete(kp.paths, origin{to, line}) // remove `seg` from paths
		delete(kp.seeds, origin{to, line})
	}
	kp.pathTable.SetPred(from, to, c.demerits, evalCost, line)
	path.totalcost = evalCost
	path.worstbadness = max(preSeg.worstbadness, c.badness)
	kp.paths[origin{to, line}] = path
	kp.seeds[origin{to, line}] = seed.segmentState()
	tracer().Debugf(" paths:\n%s", pathsAsString(kp.paths))
	tracer().Debugf("new segment %s ---(C:%d|L:%d)---> %s", from.String(), c.demerits,
		line, to.String())
}

// === Algorithms ============================================================

type lineCost struct {
	bp      BreakRef
	line    lineNo
	cost    cost
	stretch dimen.DU // stretch / shrink
}

func (lc lineCost) String() string {
	c := lc.cost.String()
	return fmt.Sprintf("LC{%s line=%d %s %.2f}", lc.bp.String(),
		lc.line, c, lc.stretch.Points())
}

type lineDisposition uint8

const (
	// lineInfeasible means the segment cannot be adjusted to the target line at all:
	// there is not enough stretch or shrink capacity to compute an adjustment ratio.
	lineInfeasible lineDisposition = iota
	// lineScreenedOut means the segment is geometrically feasible, but its badness
	// exceeds the active threshold of the current pass.
	lineScreenedOut
	// lineAccepted means the segment survives pass screening and therefore receives
	// a demerit score for path ranking.
	lineAccepted
)

// lineEvaluation is the complete local evaluation of one candidate line from one
// active breakpoint state. It keeps the screening inputs (ratio, badness) and,
// for accepted lines only, the ranking value (demerits).
type lineEvaluation struct {
	bp          BreakRef
	line        lineNo
	ratio       float64
	stretch     dimen.DU
	disposition lineDisposition
	badness     merits
	demerits    merits
}

type breakCandidate struct {
	eval   lineEvaluation
	target BreakRef
	seed   bookkeeping
}

func (ev lineEvaluation) accepted() bool {
	return ev.disposition == lineAccepted
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

func (b bookkeeping) segmentState() bookkeeping {
	return bookkeeping{
		segment:      b.segment,
		leadingTrim:  b.leadingTrim,
		trailingTrim: b.trailingTrim,
		seenContent:  b.seenContent,
	}
}

func (bc breakCandidate) lineCost() lineCost {
	return lineCost{
		bp:      bc.target,
		line:    bc.eval.line,
		stretch: bc.eval.stretch,
		cost: cost{
			badness:  bc.eval.badness,
			demerits: bc.eval.demerits,
		},
	}
}

// evaluateCandidates performs the TeX-like local line scoring step for one active
// breakpoint.
//
// The procedure is intentionally split into two conceptual phases:
// 1. screening
//   - can an adjustment ratio be computed at all?
//   - if so, is the badness within the active pass limit?
//
// 2. ranking
//   - only accepted candidates receive demerits
//
// This separation matters because pass 2 hyphenation hooks into screened-out
// but still geometrically meaningful candidates.
func (kp *linebreaker) evaluateCandidates(bp BreakRef, penalty khipu.Penalty) ([]lineEvaluation, bool) {
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
			if ratio, ok := calcAdjustmentRatio(segwss, linelen); ok {
				canReach = true
				ev.ratio = ratio
				b, _ := calcBadness(segwss, linelen)
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
//
// In TeX terms:
// - short lines divide by available stretch
// - long lines divide by available shrink
//
// This function is pure geometry. It does not decide whether the line is
// acceptable in the current pass.
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

// calcBadness turns an adjustment ratio into a TeX-style badness value.
//
// The current implementation follows the classic cubic form:
//
//	badness = 100 * |ratio|^3
//
// with the same practical cap used elsewhere in the rewrite. Like
// calcAdjustmentRatio, this function is part of pass screening, not of path
// ranking.
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
//
// The shape follows the TeX idea:
// - q = linePenalty + badness
// - start with q^2
// - positive penalties add p^2
// - negative penalties subtract p^2
//
// The result is a ranking value among already accepted candidates; screening by
// badness has happened earlier.
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

type pathQuality struct {
	totalCost    merits // sum of demerits along the winning path
	worstBadness merits // highest local line badness on the winning path
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
	breakrefs, quality, ok, err := breakParagraphPassRefs(khipu, parshape, params, false)
	if err != nil {
		return nil, err
	}
	if ok && len(breakrefs) > 0 &&
		quality.worstBadness <= params.PreTolerance &&
		quality.totalCost < AwfulDemerits {
		recordSelectedDiscretionaries(khipu, breakrefs)
		return projectBreakRefs(breakrefs), nil
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
	breakrefs, _, ok, err = breakParagraphPassRefs(khipu, parshape, params, true)
	if err != nil {
		return nil, err
	}
	if !ok || len(breakrefs) == 0 {
		return nil, ErrNoBreakpoints
	}
	recordSelectedDiscretionaries(khipu, breakrefs)
	return projectBreakRefs(breakrefs), nil
}

func breakParagraphPass(khp *khipu.Khipu, parshape linebreak.ParShape,
	params *Parameters, hyphenating bool) ([]kinx, pathQuality, bool, error) {
	breakrefs, quality, ok, err := breakParagraphPassRefs(khp, parshape, params, hyphenating)
	return projectBreakRefs(breakrefs), quality, ok, err
}

func breakParagraphPassRefs(khp *khipu.Khipu, parshape linebreak.ParShape,
	params *Parameters, hyphenating bool) ([]BreakRef, pathQuality, bool, error) {
	kp, err := prepareLineBreaker(parshape, params)
	if err != nil {
		return nil, pathQuality{}, false, err
	}
	kp.hyphenating = hyphenating
	if err := kp.constructBreakpointGraph(khp); err != nil {
		tracer().Errorf("K&P: %w", err)
		return nil, pathQuality{}, false, err
	}
	breakrefs, quality, ok := kp.collectOptimalBreakRefs(kp.end)
	return breakrefs, quality, ok, nil
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
func (kp *linebreaker) constructBreakpointGraph(khipu *khipu.Khipu) error {
	//
	if len(khipu.W) == 0 {
		return ErrNoBreakpoints
	}
	kp.end = PlainBreak(len(khipu.W) - 1)
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
		horizon := slices.Collect(maps.Keys(kp.horizon))
		slices.SortFunc(horizon, compareBreakRef)
		tracer().Debugf("horizon: %v", horizon)
		//for horiz_bp := range kp.horizon { // loop over active feasible breakpoints of horizon
		for _, horiz_bp := range horizon { // loop over active feasible breakpoints of horizon
			//for fb != nil { // loop over active feasible breakpoints of horizon
			//tracer().Debugf("      --- %s (in horizon) --> %v", knotInxStr(horiz_bp), khipu.KnotByIndex(last))
			tracer().Debugf("   --- %s (in horizon) --> candidate %v", horiz_bp.String(), knotString(last, khipu))
			if horiz_bp.IsDiscretionary() && last <= horiz_bp.At {
				// Discretionary breakpoints become active only after the scan has
				// advanced past their source textbox. Until then they represent a
				// not-yet-reached logical break inside that textbox.
				continue
			}
			kp.updatePath(horiz_bp, last, k) // now WSS extends to new knot k
			currentBreak := PlainBreak(last)
			if kp.hyphenating {
				// If the current knot itself is a textbox with cached
				// discretionaries, it may create additional logical breakpoints
				// inside that knot.
				discretionaryBreaks := kp.evaluateDiscretionaryCandidates(khipu, horiz_bp, last, last)
				for _, cand := range discretionaryBreaks {
					c := cand.lineCost()
					tracer().Debugf("   check discretionary segm line-costs %s", c.String())
					kp.evalNewSegmentSeed(horiz_bp, cand.target, c.line+1, c.cost, cand.seed)
					kp.horizon[cand.target] = struct{}{}
				}
			}
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
						c.cost.demerits += kp.terminalHyphenDemerits(horiz_bp, currentBreak)
						kp.evalNewSegment(horiz_bp, currentBreak, c.line+1, c.cost)
						//newfb := kp.newFeasibleLine(fb, cursor.Mark(), cost.demerits, linecnt+1)
						//kp.horizon.Add(newfb) // make forced break member of horizon n+1
						kp.horizon[currentBreak] = struct{}{} // make forced break member of horizon n+1
					} else if kp.hyphenating && kp.wantsHyphenation(ev) {
						if err := kp.requestPass2Discretionaries(khipu, last, ev); err != nil {
							return err
						}
						if candidate, ok := hyphenationCandidateForPass2(khipu, last, ev); ok {
							// A bad pass-2 line may become acceptable if one of the
							// textbox-local discretionary variants is inserted into
							// this same segment.
							discretionaryBreaks := kp.evaluateDiscretionaryCandidates(khipu, horiz_bp, last, candidate)
							for _, cand := range discretionaryBreaks {
								c := cand.lineCost()
								tracer().Debugf("   check requested discretionary segm line-costs %s", c.String())
								kp.evalNewSegmentSeed(horiz_bp, cand.target, c.line+1, c.cost, cand.seed)
								kp.horizon[cand.target] = struct{}{}
							}
						}
						if ev.accepted() { // current graph still uses the unhyphenated path
							kp.evalNewSegment(horiz_bp, currentBreak, c.line+1, c.cost)
							kp.horizon[currentBreak] = struct{}{}
						}
					} else if ev.accepted() { // happy case: new breakpoint is feasible
						//
						kp.evalNewSegment(horiz_bp, currentBreak, c.line+1, c.cost)
						//newfb := kp.newFeasibleLine(fb, cursor.Mark(), cost.demerits, linecnt+1)
						//kp.horizon.Add(newfb) // make new breakpoint member of horizon n+1
						kp.horizon[currentBreak] = struct{}{} // make new breakpoint member of horizon n+1
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
						kp.evalNewSegment(horiz_bp, currentBreak, c.line+1, c.cost)
						kp.horizon[currentBreak] = struct{}{}
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

		} // --- end of main loop over horizon ----------------------
	} // end of outer loop over input knots
	tracer().Infof("Collected %d potential breakpoints for paragraph", len(kp.feasBP))
	tracer().Infof("          %v", kp.feasBP)
	tracer().Infof("          predecessors:\n%v", kp.String())
	tracer().Infof("          paths:\n%s", pathsAsString(kp.paths))
	// kp.end = fb // remember last breakpoint of paragraph
	return nil
}

// collectOptimalBreakRefs walks the predecessor table backwards from the
// paragraph terminus and returns the single cheapest breakpoint sequence.
func (kp *linebreaker) collectOptimalBreakRefs(end BreakRef) ([]BreakRef, pathQuality, bool) {
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
	breaks := make([]BreakRef, 0, bestLine)
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

func (kp *linebreaker) collectOptimalBreakpoints(end BreakRef) ([]kinx, pathQuality, bool) {
	breakrefs, quality, ok := kp.collectOptimalBreakRefs(end)
	return projectBreakRefs(breakrefs), quality, ok
}

func projectBreakRefs(breakrefs []BreakRef) []kinx {
	if len(breakrefs) == 0 {
		return nil
	}
	breakpoints := make([]kinx, 0, len(breakrefs))
	for _, br := range breakrefs {
		breakpoints = append(breakpoints, br.At)
	}
	return breakpoints
}

// recordSelectedDiscretionaries persists only the discretionary choices from
// the winning path back into the Khipu. Ordinary breakpoints continue to be
// represented by the returned []kinx; SelectedDiscretionaries stores the extra
// variant information needed for later paragraph processing stages.
func recordSelectedDiscretionaries(khp *khipu.Khipu, breakrefs []BreakRef) {
	if khp == nil {
		return
	}
	if khp.SelectedDiscretionaries != nil {
		// The final Khipu should reflect only the winning path of the current
		// linebreaking run; stale selections from earlier runs must not survive.
		clear(khp.SelectedDiscretionaries)
	}
	for _, br := range breakrefs {
		if !br.IsDiscretionary() {
			continue
		}
		khp.SelectDiscretionary(br.At, khipu.DiscretionarySelection{
			Source:  br.At,
			Variant: br.Variant,
		})
	}
}

// --- Helpers ----------------------------------------------------------

func absD(n dimen.DU) dimen.DU {
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
		case BreakRef:
			sb.WriteString(BreakRef(v).String())
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
