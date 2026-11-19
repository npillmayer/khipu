package knuthplass

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/internal/search"
)

// BreakParagraph publishes the minimum-cost complete path from the first
// successful KP pass. The immutable result captures widths, skips, selected
// replacements, effective policy and diagnostics. Seeded KP remains unsupported.
func BreakParagraph(r khipu.BreakRequest, params *Parameters) (*khipu.Tikray, error) {
	if e := r.Validate(); e != nil {
		return nil, e
	}
	if r.Prefix != nil {
		return nil, khipu.ErrUnsupportedPolicy
	}
	if params == nil {
		params = NewDefaultParameters()
	}
	local := *params
	local.discretionaryProvider = search.Provider{Discovery: r.Discovery}
	widths := search.NewWidths(r.Geometry.Shape, nil)
	m := search.New(r.Binding, r.Discovery)
	result, e := selectParagraphBreaks(m, widths.Layout(r.Geometry), &local)
	if widths.Err != nil {
		return nil, widths.Err
	}
	if e != nil {
		return nil, e
	}
	recordSelectedDiscretionaries(m, result.breaks)
	decisions, e := widths.Decisions(r, projectBreakRefs(result.breaks), m.SelectedDiscretionaries, nil)
	if e != nil {
		return nil, e
	}
	policy := khipu.PolicyRecord{Algorithm: khipu.KnuthPlassAlgorithm, KnuthPlass: khipu.KnuthPlassSettings{PreTolerance: int64(local.PreTolerance), Tolerance: int64(local.Tolerance), LinePenalty: int64(local.LinePenalty), FirstHyphenDemerits: int64(local.FirstHyphenDemerits), FinalHyphenDemerits: int64(local.FinalHyphenDemerits), EmergencyStretch: max(0, local.EmergencyStretch)}}
	return khipu.NewTikray(r, decisions, policy, khipu.BreakDiagnostics{HasScore: true, Score: int64(result.quality.totalCost), HasPass: true, Pass: int(result.pass.id)})
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
