package knuthplass

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

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
//
// This synchronous call has no context argument. Cancellation passed to encoding
// or freezing does not cancel this search or its provider calls. The standard
// owner provider shapes lazy replacements with context.Background().
func BreakParagraph(khp *khipu.Khipu, layout khipu.Params, params *Parameters) (
	[]int, error) {
	result, err := selectParagraphBreaks(khp, layout, params)
	if err != nil {
		return nil, err
	}
	recordSelectedDiscretionaries(khp, result.breaks)
	return projectBreakRefs(result.breaks), nil
}

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
