// Package knuthplass selects paragraph line breaks by minimum accumulated
// demerits among the candidates explored by a Knuth-Plass-style search.
// It consumes indexed elastic khipu.Khipu knots; it neither shapes text nor
// assigns final knot widths.
//
// # Input and result ownership
//
// BreakParagraph takes resolved paragraph geometry in khipu.Params and separate
// algorithm Parameters. Use DefaultLayout to obtain the package's layout profile,
// then reuse the resolved geometry for freezing. The input must end with a
// non-discardable paragraph-fill knot carrying InfinityMerits. Discardability
// comes from knot flags, not from an inferred glue or kern rule.
//
// The result contains zero-based line-ending knot indices, including the final
// trailer. BreakRef distinguishes ordinary breaks from discretionary variants
// during search. Only a successful call replaces Khipu.SelectedDiscretionaries
// with the winning variants; a failed call leaves previous selections intact.
// Discovery may still populate paragraph-owned candidate caches. Providers own
// shaping and candidate penalties; the breaker reads PreBreak.Penalty and seeds
// the following line with PostBreak.
//
// # Passes and scoring
//
// Pass 1 uses PreTolerance without discretionary discovery. If it cannot complete
// a path, pass 2 uses Tolerance and permits lazy discovery. Positive finite
// EmergencyStretch enables pass 3, which uses the same tolerance with additional
// virtual stretch for scoring and discovery only. The first complete pass wins;
// costs from different passes are not compared. Only the final configured pass
// may use fallback transitions. Provider errors abort rather than start a retry.
// Emergency stretch does not change physical widths or guarantee that freezing
// can achieve the requested line length.
//
// # Search-state lifecycle
//
// Each pass owns fresh working states keyed by (BreakRef, completed line count).
// The horizon groups active origins by breakpoint. At each knot, processKnot
// first advances existing states, then discovers discretionary successors, then
// commits the boundary using a fresh horizon snapshot. This prevents newly
// inserted states from accumulating the same knot twice and lets them reach a
// forced boundary immediately.
//
// Working bookkeeping holds costs and leading/trailing width trims. Separate
// immutable seeds support discretionary rescans. Overfull states are retired
// after rescue when minimum-width bounds make pruning safe; forced boundaries
// retire earlier origins so no path can bypass them. Retirement removes working
// data, not predecessor history. Tombstones prevent retroactive resurrection.
// The predecessor table keeps the best transition for each destination state.
// Collection walks that table backward from the cheapest terminal state and
// reverses the result. Equal-cost choices use deterministic tie breakers.
//
// Ordinary and discretionary evaluations use distinct pass-local scratch
// buffers. Boundary-local cached evaluations are invalidated when their working
// bucket changes; they do not cache accumulated predecessor costs. These storage
// lifetimes matter when adding nested discovery or new state transitions.
//
// # Implementation map and limits
//
// passes.go orchestrates retries; boundary.go advances, commits and retires
// states; geometry.go classifies line geometry; knuthplass.go manages working
// bookkeeping, demerits and collection; path.go stores predecessor history;
// discretionary.go discovers and evaluates replacements; evaluation_cache.go
// owns boundary-local reuse. The historical constructBreakpointGraph name now
// refers to predecessor-table construction, not a general graph. Helpers named
// Pass2 also serve emergency-pass discovery.
//
// This is not a complete TeX implementation: fitness classes and consecutive
// discretionary demerits are not modeled, and lazy discovery does not enumerate
// every possible hyphenation. Exact symbolic fill-order arithmetic is also not
// modeled by the breaker's numerical width summaries.
package knuthplass

import "github.com/npillmayer/schuko/tracing"

// tracer traces with key 'khipu.linebreak'.
func tracer() tracing.Trace {
	return tracing.Select("khipu.linebreak")
}

func assert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}

// infinityDemerits is the capped line badness, not a bound on accumulated costs.
const infinityDemerits merits = 10000
