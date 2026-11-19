package knuthplass

import (
	"fmt"
	"github.com/npillmayer/khipu/internal/search"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/schuko/tracing"
)

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

// badnessLimit returns the active screening threshold for the current pass.
// Pass 1 uses PreTolerance; passes 2 and 3 use Tolerance with discretionary
// breaking enabled.
func (kp *linebreaker) badnessLimit() merits {
	if kp != nil && kp.hyphenating {
		return kp.params.Tolerance
	}
	return kp.params.PreTolerance
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
		if err := search.WidthError(kp.layout.LineShape); err != nil {
			kp.arithmeticErr = err
			return nil
		}
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
