package knuthplass

import (
	"fmt"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/schuko/tracing"
)

type passID uint8

const (
	passPreTolerance passID = iota + 1
	passTolerance
	passEmergency
)

// passConfig separates discretionary discovery from permission to recover with
// fallback. Pass 2 is final only when emergency stretch is disabled. Every pass
// starts with fresh working states, predecessor history and evaluation caches;
// paragraph-owned discretionary discovery caches remain available across passes.
type passConfig struct {
	id               passID
	hyphenating      bool
	final            bool
	emergencyStretch dimen.DU
}

// passResult retains the scoring context with the winning path. Emergency
// quality is optimization quality under virtual capacity, not physical quality
// for freezing. complete=false with no error is an ordinary exhausted search;
// provider and validation errors must never be treated as reasons to retry.
type passResult struct {
	pass     passConfig
	breaks   []BreakRef
	quality  pathQuality
	complete bool
}

// paragraphPasses validates the public emergency setting before any search,
// even if pass 1 could succeed. Nonpositive values disable the extra pass.
func paragraphPasses(params *Parameters) ([]passConfig, error) {
	e := params.EmergencyStretch
	if e >= dimen.Fil {
		return nil, fmt.Errorf("%w: EmergencyStretch must be a finite dimension below Fil", khipu.ErrIllegalArguments)
	}
	passes := []passConfig{
		{id: passPreTolerance},
		{id: passTolerance, hyphenating: true, final: e <= 0},
	}
	if e > 0 {
		passes = append(passes, passConfig{id: passEmergency, hyphenating: true, final: true, emergencyStretch: e})
	}
	return passes, nil
}

// selectParagraphBreaks returns the first complete pass, without publishing any
// selections. Costs from different passes are not comparable. Non-final passes
// enforce their tolerance at every boundary; only the final pass may recover
// with fallback. Publication belongs to the successful public-call boundary.
func selectParagraphBreaks(khp *khipu.Khipu, layout khipu.Params, params *Parameters) (passResult, error) {
	if params == nil {
		params = NewDefaultParameters()
	}
	passes, err := paragraphPasses(params)
	if err != nil {
		return passResult{}, err
	}
	var result passResult
	for _, pass := range passes {
		result, err = breakParagraphPassRefs(khp, layout, params, pass)
		if err != nil {
			return result, err
		}
		if result.complete {
			if trace := tracer(); trace.GetTraceLevel() >= tracing.LevelDebug {
				trace.Debugf("K&P: selected pass %d, emergency stretch=%v, worst badness=%d, cost=%d",
					pass.id, pass.emergencyStretch, result.quality.worstBadness, result.quality.totalCost)
			}
			return result, nil
		}
		if trace := tracer(); trace.GetTraceLevel() >= tracing.LevelDebug {
			trace.Debugf("K&P: pass %d found no complete path", pass.id)
		}
	}
	return result, ErrNoBreakpoints
}

// breakParagraphPassRefs runs exactly the supplied pass. Geometry is validated
// before scanning; no partial path or discretionary selection is published.
// The classifier's emergency-adjusted badness also drives lazy discovery in
// pass 3. Overfull rescue and explicit discretionary requests remain independent
// of that badness threshold.
func breakParagraphPassRefs(khp *khipu.Khipu, layout khipu.Params,
	params *Parameters, pass passConfig) (passResult, error) {
	result := passResult{pass: pass}
	kp, err := prepareLineBreaker(layout, params)
	if err != nil {
		return result, err
	}
	kp.hyphenating, kp.final, kp.emergencyStretch = pass.hyphenating, pass.final, pass.emergencyStretch
	if err := kp.constructBreakpointGraph(khp); err != nil {
		// Reporting belongs to the caller; return the original provider/input
		// error without logging it again at this internal pass boundary.
		return result, err
	}
	result.breaks, result.quality, result.complete = kp.collectOptimalBreakRefs(kp.end)
	result.complete = result.complete && len(result.breaks) > 0
	return result, nil
}
