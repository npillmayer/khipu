package knuthplass

import (
	"cmp"
	"fmt"
	"github.com/npillmayer/khipu/linebreak"
	"slices"
	"strings"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/schuko/tracing"
)

// linebreaker is an internal entity for K&P-linebreaking.
// Working-state buckets let width updates and scoring visit only the requested
// breakpoint's line counts. Retired working states are deleted; the separate
// predecessor table retains the history needed for reconstruction.
// A full BreakRef is the key so discretionary variants never share a bucket.
type linebreaker struct {
	arithmeticErr  error
	compactUniform bool         // automatic for built-in rectangles; tests may disable
	uniformRemoved int          // diagnostic count of dominated ordinary working states
	metrics        *workMetrics // optional diagnostics; nil in public breaking calls
	*pathTable
	horizon     map[BreakRef]struct{}               // horizon of possible linebreaks
	paths       map[BreakRef]map[lineNo]bookkeeping // working states grouped by breakpoint
	seeds       map[origin]bookkeeping              // immutable seed state for rescanning a segment
	params      *Parameters                         // typesetting parameters relevant for line-breaking
	layout      khipu.Params                        // resolved geometry, shared with freezing
	hyphenating bool                                // passes 2 and 3 use Tolerance and discretionaries
	final       bool                                // only the final pass may use fallback transitions
	retired     map[origin]struct{}                 // tombstones prevent retroactive resurrection
	prune       bool                                // original/cached widths permit monotone retirement
	minimumSum  int64                               // bound for minimum-width accumulation before trimming
	root        BreakRef                            // "break" at start of paragraph
	end         BreakRef                            // "break" at end of paragraph
	// emergencyStretch is scoring-only and constant for a pass. Only pass 3
	// supplies virtual capacity; real knot and fragment widths never change.
	emergencyStretch dimen.DU

	// Scratch storage is owned by this single-pass breaker, never pooled or
	// shared across paragraphs. Each result survives until the next call to
	// its producer. Passes 2 and 3 consume ordinary evaluations while requesting
	// discretionaries, so those buffers must be independent. Elements contain
	// no references to paragraph storage and need no clearing when shortened.
	evaluationScratch    []lineEvaluation
	discretionaryScratch []breakCandidate
	horizonScratch       []BreakRef
	retirementScratch    []origin
	boundaryEvals        boundaryEvaluations
}

func pathsAsString(paths map[BreakRef]map[lineNo]bookkeeping) string {
	var sb strings.Builder
	var os []origin
	for bp, lines := range paths {
		for line := range lines {
			os = append(os, origin{bp, line})
		}
	}
	slices.SortFunc(os, func(a, b origin) int {
		d := compareBreakRef(a.from, b.from)
		if d != 0 {
			return d
		}
		return cmp.Compare(a.line, b.line)
	})
	for _, o := range os {
		b := paths[o.from][o.line]
		sb.WriteString(fmt.Sprintf("BP[%s L:%2d] %v\n", o.from.String(), o.line, b))
	}
	return sb.String()
}

func newLinebreaker(layout khipu.Params, params *Parameters) *linebreaker {
	kp := &linebreaker{}
	kp.pathTable = newPathTable()
	kp.horizon = make(map[BreakRef]struct{})
	kp.paths = make(map[BreakRef]map[lineNo]bookkeeping)
	kp.seeds = make(map[origin]bookkeeping)
	kp.retired = make(map[origin]struct{})
	kp.layout = layout
	kp.compactUniform = knownUniformShape(layout.LineShape)
	if params == nil {
		params = NewDefaultParameters()
	}
	kp.params = params
	return kp
}

// DefaultLayout preserves the finite-stretch right skip of the K&P profile.
// Resolve it before breaking, apply caller overrides, and pass the same Params
// to freezing. Nil algorithm parameters never substitute geometry defaults.
func DefaultLayout(shape khipu.LineShape) khipu.Params {
	layout := khipu.DefaultParams()
	layout.LineShape = shape
	layout.LeftSkip = glue(0, 0, 0)
	layout.RightSkip = glue(0, 0, 50*dimen.BP)
	return layout
}

func prepareLineBreaker(layout khipu.Params,
	params *Parameters) (*linebreaker, error) {
	//
	if layout.LineShape == nil {
		return nil, ErrNoParShape
	}
	if err := layout.ValidateGeometry(); err != nil {
		return nil, err
	}
	kp := newLinebreaker(layout, params)
	kp.root = PlainBreak(noinx)                       // virtual starting knot
	kp.horizon[kp.root] = struct{}{}                  // root is now officially a breakpoint
	kp.paths[kp.root] = map[lineNo]bookkeeping{0: {}} // start of every path
	kp.seeds[origin{kp.root, 0}] = bookkeeping{}
	return kp, nil
}

// --- Breakpoints -----------------------------------------------------------

type bookkeeping struct {
	segment      linebreak.WSS // sum of widths from this breakpoint up to current knot
	totalcost    merits        // sum of costs for segment up to this breakpoint
	worstbadness merits        // highest line badness encountered along the path
	leadingTrim  linebreak.WSS // discardable width before the first retained line item
	trailingTrim linebreak.WSS // discardable width after the most recent retained line item
	seenContent  bool          // does this segment contain a non-discardable content item?
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
	if k.IsContent() {
		return LICContent
	}
	return LICRetainedNeutral
}

func (book *bookkeeping) appendItem(cls lineItemClass, w linebreak.WSS) {
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
		book.trailingTrim = linebreak.WSS{}
	case LICRetainedNeutral:
		// width stays in segment, but does not contribute to trim bookkeeping
	}
}

func (book bookkeeping) effectiveWidth(params khipu.Params) linebreak.WSS {
	segw := book.segment
	segw = segw.Subtract(book.leadingTrim)
	segw = segw.Subtract(book.trailingTrim)
	w := linebreak.WSS{}.SetFromKnot(params.LeftSkip)
	segw = segw.Add(w)
	w = linebreak.WSS{}.SetFromKnot(params.RightSkip)
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

func (kp *linebreaker) updatePath(bp BreakRef, idx kinx, k khipu.KnotCore) {
	if bp.IsDiscretionary() && idx <= bp.At {
		return
	}
	kp.boundaryEvals.invalidate(bp)
	wss := linebreak.WSS{}.SetFromKnot(k) // get dimensions of knot
	cls := classifyLineItem(k)            // classify for line-edge trimming
	trace := tracer()
	debug := trace.GetTraceLevel() >= tracing.LevelDebug
	lines := kp.paths[bp]
	for line, path := range lines {
		if debug {
			trace.Debugf("   --- path of %s/%v = %v", bp.String(), origin{bp, line}, path)
		}
		path.appendItem(cls, wss)
		if err := path.segment.Err(); err != nil {
			kp.arithmeticErr = err
		}
		if debug {
			trace.Debugf("K&R: extending segment from %s to %v", bp.String(), path.segment)
		}
		lines[line] = path
	}
}

// badnessLimit returns the active screening threshold for the current pass.
// Pass 1 uses PreTolerance; passes 2 and 3 use Tolerance with discretionary
// breaking enabled.
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

func (kp *linebreaker) evalNewSegment(from, to BreakRef, line lineNo, c cost) bool {
	return kp.evalNewSegmentSeed(from, to, line, c, bookkeeping{})
}

// evalNewSegmentSeed reports whether the target has a live working state,
// including when an existing predecessor wins. Callers may activate only live
// targets. Retired targets keep their historical predecessor unchanged.
func (kp *linebreaker) evalNewSegmentSeed(from, to BreakRef, line lineNo, c cost, seed bookkeeping) bool {
	if _, retired := kp.retired[origin{to, line}]; retired {
		return false
	}
	trace := tracer()
	debug := trace.GetTraceLevel() >= tracing.LevelDebug
	if bp := kp.pathTable.Breakpoint(to); bp == NoBreakRef {
		kp.pathTable.AddBP(to)
	}
	// now sure that `to` is a breakpoint
	preSeg, ok := kp.paths[from][line-1]
	if debug {
		trace.Debugf("K&P prev seg = %v", preSeg)
	}
	assert(ok, "K&P internal error: cannot append segment to non-existent sub-path")
	evalCost := preSeg.totalcost + c.demerits
	pred, hasPred := kp.pathTable.Pred(to, line)
	lines := kp.paths[to]
	_, ok = lines[line]
	path := seed.segmentState() // `path` will be the new path, if cheaper than `seg`
	if !hasPred {
		assert(!ok, "K&P internal error: path with non-existent predecessor state exists")
	} else if !betterPredecessor(from, evalCost, pred) {
		assert(ok, "K&P internal error: path for predecessor state does not exist")
		return true // existing path is cheaper or wins the canonical tie
	} else {
		if debug {
			trace.Debugf("K&P remove sub-optimal seg %v", origin{to, line})
		}
		delete(lines, line) // remove the superseded working state
		delete(kp.seeds, origin{to, line})
	}
	kp.pathTable.SetPred(from, to, c.demerits, evalCost, line)
	path.totalcost = evalCost
	path.worstbadness = max(preSeg.worstbadness, c.badness)
	if lines == nil {
		lines = make(map[lineNo]bookkeeping)
		kp.paths[to] = lines
	}
	lines[line] = path
	kp.boundaryEvals.invalidate(to)
	if kp.metrics != nil && !ok {
		kp.metrics.live++
		kp.metrics.peak = max(kp.metrics.peak, kp.metrics.live)
	}
	kp.seeds[origin{to, line}] = seed.segmentState()
	// Debugf cannot defer argument evaluation. In particular, sorting and
	// formatting the entire state table must never happen with debug disabled.
	if debug {
		trace.Debugf(" paths:\n%s", pathsAsString(kp.paths))
		trace.Debugf("new segment %s ---(C:%d|L:%d)---> %s", from.String(), c.demerits,
			line, to.String())
	}
	return true
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
	// lineOverfull means the effective minimum width exceeds the target. No
	// tolerance can make this an acceptable ordinary or discretionary line.
	lineOverfull lineDisposition = iota
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
	eval    lineEvaluation
	target  BreakRef
	seed    bookkeeping
	initial bookkeeping // post-break fragment alone, without the consumed suffix
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
//   - does the line fit within its shrink capacity?
//   - if so, is the badness within the active pass limit?
//
// 2. ranking
//   - only accepted candidates receive demerits
//
// Ordinary and discretionary lines use the same local classifier. Retirement
// is a later boundary-wide decision, after all rescue transitions are considered.
//
// The returned slice borrows breaker-owned storage until the next call to
// evaluateCandidates. Scoring completes before callers mutate path state;
// discretionary evaluation uses separate storage and does not invalidate it.
// During a boundary transaction, unchanged buckets reuse their local results.
// Both normal scratch and cached results are read-only to callers.
func (kp *linebreaker) evaluateCandidates(bp BreakRef, penalty khipu.Penalty) []lineEvaluation {
	if cached, ok := kp.boundaryEvals.get(bp, penalty); ok {
		if kp.metrics != nil {
			kp.metrics.ordinaryReused += uint64(len(cached))
		}
		return cached
	}
	trace := tracer()
	debug := trace.GetTraceLevel() >= tracing.LevelDebug
	// find all paths ending at `bp`
	result := kp.evaluationScratch[:0]
	for line, path := range kp.paths[bp] {
		if kp.metrics != nil {
			kp.metrics.ordinary++
		}
		linelen := kp.layout.LineShape.LineLength(int32(line + 1))
		segwss := path.effectiveWidth(kp.layout)
		if err := segwss.Err(); err != nil {
			kp.arithmeticErr = err
		}
		if debug {
			trace.Debugf("    +---%.2f--->    | %.2f", segwss.W.Points(), linelen.Points())
		}
		ev := classifyLine(segwss, linelen, kp.badnessLimit(), kp.emergencyStretch)
		ev.bp, ev.line = bp, line
		if ev.accepted() {
			ev.demerits = calcDemerits(ev.badness, penalty, kp.params)
		}
		result = append(result, ev)
	}
	kp.evaluationScratch = result
	kp.boundaryEvals.put(bp, penalty, result)
	return result
}

// calcAdjustmentRatio computes the TeX-style adjustment ratio for one line.
// Positive ratios mean stretching, negative ratios mean shrinking.
// Except at exact fit, the ratio requires stretch or shrink capacity in the
// relevant direction. The classifier supplies signed infinity when it is absent.
//
// In TeX terms:
// - short lines divide by available stretch
// - long lines divide by available shrink
//
// This function is pure geometry. It does not decide whether the line is
// acceptable in the current pass.
func calcAdjustmentRatio(segwss linebreak.WSS, linelen dimen.DU) (float64, bool) {
	if segwss.W == linelen {
		return 0, true
	}
	if segwss.W < linelen {
		available := int64(segwss.Max) - int64(segwss.W)
		if available <= 0 {
			return 0, false
		}
		return float64(int64(linelen)-int64(segwss.W)) / float64(available), true
	}
	available := int64(segwss.W) - int64(segwss.Min)
	if available <= 0 {
		return 0, false
	}
	return -float64(int64(segwss.W)-int64(linelen)) / float64(available), true
}

// calcBadness turns an adjustment ratio into a TeX-style badness value.
//
// The current implementation follows the classic cubic form:
//
//	badness = 100 * |ratio|^3
//
// with the same practical cap used elsewhere in the rewrite. Like
// calcAdjustmentRatio, this function is part of pass screening, not of path
// ranking. This low-level helper reports the cubic value only; classifyLine
// additionally rejects excessive shrink even when that value is below tolerance.
func calcBadness(segwss linebreak.WSS, linelen dimen.DU) (merits, bool) {
	ratio, ok := calcAdjustmentRatio(segwss, linelen)
	if !ok {
		return 0, false
	}
	return badnessForRatio(ratio), true
}

// calcDemerits computes line demerits from a precomputed badness value.
// Forced breaks are handled in buildBreakpointPaths and therefore reach
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
	if trace := tracer(); trace.GetTraceLevel() >= tracing.LevelDebug {
		trace.Debugf("    calculating demerits for p=%d, b=%d: d=%d", p, badness, d)
	}
	return d
}

type pathQuality struct {
	totalCost    merits // sum of demerits along the winning path
	worstBadness merits // highest local badness under this pass's scoring assumptions
}

// --- Main API ---------------------------------------------------------

// BreakParagraph returns the minimum-cost complete path among the candidates
// explored in the first successful pass. Equal-cost terminal states prefer
// fewer lines; costs from different passes are not compared.
//
// Results are zero-based indices of line-ending knots, including the final
// trailer. SelectedDiscretionaries is replaced only on success. Errors preserve
// earlier selections, although discovery may have populated candidate caches.
// Nil params selects
// NewDefaultParameters. Only the single best path is returned; alternative
// line-count solutions are not enumerated. Layout must be resolved explicitly
// (for example with DefaultLayout) and reused unchanged for freezing.
// Positive finite EmergencyStretch enables a third pass after two unsuccessful
// non-final passes. Its virtual capacity affects scoring, not physical widths:
// an emergency result can still exceed the real capacity available for freezing.
func BreakParagraph(khp *khipu.Khipu, layout khipu.Params, params *Parameters) (
	[]kinx, error) {
	result, err := selectParagraphBreaks(khp, layout, params)
	if err != nil {
		return nil, err
	}
	recordSelectedDiscretionaries(khp, result.breaks)
	return projectBreakRefs(result.breaks), nil
}

// snapshotHorizon returns a sorted snapshot, valid until its next call. The
// live horizon can change while one phase consumes it. Only after that phase
// finishes may the next phase refresh it, including boundary commit at the same
// knot. Newly discovered successors then participate without advancing twice.
func (kp *linebreaker) snapshotHorizon() []BreakRef {
	kp.horizonScratch = kp.horizonScratch[:0]
	for bp := range kp.horizon {
		kp.horizonScratch = append(kp.horizonScratch, bp)
	}
	slices.SortFunc(kp.horizonScratch, compareBreakRef)
	return kp.horizonScratch
}

// buildBreakpointPaths scans SOA knot indices and maintains a horizon of
// active BreakRefs. For each permitted boundary, it evaluates transitions from
// active origins and records the cheapest predecessor for each line count.
// Forced boundaries retire earlier origins; discretionary variants seed the
// following line with their post-break fragment. Collection follows the
// predecessor table to recover the single cheapest complete path.
func (kp *linebreaker) buildBreakpointPaths(khp *khipu.Khipu) error {
	if khp == nil || len(khp.W) == 0 {
		return ErrNoBreakpoints
	}
	kp.end = PlainBreak(len(khp.W) - 1)
	kp.prepareRetirement(khp)
	trace := tracer()
	for last := range len(khp.W) {
		live, err := kp.processKnot(khp, last)
		if kp.arithmeticErr != nil {
			return kp.arithmeticErr
		}
		if err != nil || !live {
			return err
		}
	}
	if trace.GetTraceLevel() >= tracing.LevelInfo {
		trace.Infof("Collected %d potential breakpoints for paragraph", len(kp.feasBP))
		trace.Infof("          %v", kp.feasBP)
		trace.Infof("          predecessors:\n%v", kp.String())
		trace.Infof("          paths:\n%s", pathsAsString(kp.paths))
	}
	return nil
}

// collectOptimalBreakRefs walks the predecessor table backwards from the
// paragraph terminus and returns the single cheapest breakpoint sequence.
// Equal total costs prefer fewer lines, independent of map iteration order.
func (kp *linebreaker) collectOptimalBreakRefs(end BreakRef) ([]BreakRef, pathQuality, bool) {
	bestLine := lineNo(0)
	bestQuality := pathQuality{}
	found := false
	for line, path := range kp.paths[end] {
		if !found || path.totalcost < bestQuality.totalCost ||
			(path.totalcost == bestQuality.totalCost && line < bestLine) {
			bestLine = line
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
