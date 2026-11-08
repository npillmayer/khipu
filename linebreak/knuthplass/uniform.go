package knuthplass

import (
	"reflect"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/linebreak"
)

// knownUniformShape recognizes only the immutable built-in rectangular shape.
// No width probing can prove future uniformity. Custom shapes (even wrappers
// around rectangles) deliberately retain separate line-count states. Recognition
// happens once per pass, not in the knot-processing hot path.
func knownUniformShape(shape khipu.LineShape) bool {
	return reflect.TypeOf(shape) == reflect.TypeOf(linebreak.RectangularParShape(0))
}

// compactUniformBoundary runs AFTER all proposals at an ordinary boundary have
// been committed. States here start with identical empty segment bookkeeping;
// all have completed at least one line, so first-line extras no longer apply.
// At the same plain breakpoint their future geometry and scoring are identical.
// Lowest accumulated cost dominates; equal costs retain the lower line count,
// matching terminal collection. Keep actual line numbers and predecessor history.
//
// This deliberately does not compact discretionary buckets: retroactive discovery
// can propose improved predecessors there after descendants already exist. Nor
// may it run midway through discovery/commit, whose snapshots still reference
// source states. An ordinary boundary is never proposed again after its knot;
// retirement tombstones therefore cannot block a later improvement to it.
func (kp *linebreaker) compactUniformBoundary(to BreakRef) {
	if to.IsDiscretionary() || to == kp.root {
		return
	}
	lines := kp.paths[to]
	if len(lines) < 2 {
		return
	}
	var best lineNo
	found := false
	for line, book := range lines {
		if line < 1 {
			return // never unify first-line scoring with later lines
		}
		if !found || book.totalcost < lines[best].totalcost ||
			book.totalcost == lines[best].totalcost && line < best {
			best, found = line, true
		}
	}
	for line := range lines {
		if line != best {
			kp.retireState(origin{to, line})
			kp.uniformRemoved++
		}
	}
}
