/*
Package knuthplass implements (in an early draft) a
line breaking algorithm described by D.E. Knuth and M.F. Plass.

Definite source of information is of course

	Computers & Typesetting, Vol. A & C.
	http://www-cs-faculty.stanford.edu/~knuth/abcde.html

An approachable summary may be found in

	http://defoe.sourceforge.net/folio/knuth-plass.html

# BSD License

# Copyright (c) Norbert Pillmayer

All rights reserved.

Please refer to the LICENSE file for details.
*/
package knuthplass

import (
	"github.com/npillmayer/schuko/tracing"
)

// tracer traces with key 'khipu.linebreak'.
func tracer() tracing.Trace {
	return tracing.Select("khipu.linebreak")
}
