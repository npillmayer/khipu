package knuthplass

// workMetrics measures one pass, not the public multi-pass pipeline. Benchmarks
// opt in before processing; normal callers leave the pointer nil. No callbacks,
// atomics or map scans are added to the hot path.
//
// live/peak count working (BreakRef, line number) states, including the root and
// successors inserted before retirement. Replacing a predecessor is not a new
// state; historical predecessors and retirement tombstones do not count.
//
// ordinary/discretionary count actual local classifications, accepted or not.
// Repeated boundary scoring counts again; each discretionary variant counts
// once per evaluated origin. Provider requests, rejected preconditions, seed
// replay knots, and fallback ranking are not additional classifications.
// ordinaryReused counts classifications avoided by boundary-local cache hits;
// it is not included in actual evaluation totals.
type workMetrics struct {
	live, peak              int
	ordinary, discretionary uint64
	ordinaryReused          uint64
}
