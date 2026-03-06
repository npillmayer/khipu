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

// InfinityDemerits is the worst demerit value possible.
const infinityDemerits merits = 10000
