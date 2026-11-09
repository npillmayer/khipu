/*
Package firstfit implements greedy line breaking: keep the latest fitting
checkpoint and commit it when a later candidate overflows.

The current implementation operates directly on the SOA `khipu.Khipu`
and returns breakpoint indices (`[]kinx`). It keeps only the simple local
bookkeeping required by first-fit:

- accumulate one current line
- remember the most recent feasible breakpoint
- commit that breakpoint once a later candidate would overflow the line

Compared to `knuthplass`, this package intentionally stays simple:

- no global path comparison
- no graph / predecessor table
- local discretionary rescue when an overflowing textbox can improve the line
- no second pass or global discretionary search

Feasibility uses trimmed minimum width, including side skips, not natural width.
The ragged-right fil skip absorbs positive slack but supplies no shrink. Content
may shrink only within its supplied bounds; fixed-width content therefore fits
at natural width automatically. Freezing, not breaking, assigns final widths.

Discretionary rescue rejects prohibited pre-break penalties, requires minimum-
width fit and an inclusive MinHyphenGain, then ranks by natural width with penalty
as a tie-break. The default zero gain threshold permits zero improvement; it is
not a limit on hyphen frequency. Automatic discovery requires an earlier ordinary
checkpoint, while retained explicit markers can also rescue a first word.

NewDefaultParameters returns independent algorithm defaults; nil parameters use
the same profile. Layout is resolved separately with DefaultLayout. Discretionary
selections are staged until the complete call succeeds. Errors preserve previous
selections, although provider-owned candidate caches may have been populated.
Providers must not modify the published selection map themselves.
*/
package firstfit

import "github.com/npillmayer/schuko/tracing"

// tracer traces with key 'khipu.linebreak'.
func tracer() tracing.Trace {
	return tracing.Select("khipu.linebreak")
}
