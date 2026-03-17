/*
Package firstfit implements a straightforward line-breaking algorithm where
lines are broken at the first suitable breakpoint.

The current implementation operates directly on the new SOA-style `khipu.Khipu`
and returns breakpoint indices (`[]kinx`). It keeps only the simple local
bookkeeping required by first-fit:

- accumulate one current line
- remember the most recent feasible breakpoint
- commit that breakpoint once a later candidate would overflow the line

Compared to `knuthplass`, this package intentionally stays simple:

- no global path comparison
- no graph / predecessor table
- no discretionary support yet
*/
package firstfit

import "github.com/npillmayer/schuko/tracing"

// tracer traces with key 'khipu.linebreak'.
func tracer() tracing.Trace {
	return tracing.Select("khipu.linebreak")
}
