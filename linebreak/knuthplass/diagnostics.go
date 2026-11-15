package knuthplass

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/npillmayer/schuko/tracing"
)

func pathsAsString(paths map[BreakRef]map[lineNo]bookkeeping) string {
	var sb strings.Builder
	var os []origin
	for bp, lines := range paths {
		for line := range lines {
			os = append(os, origin{bp, line})
		}
	}
	slices.SortFunc(os, func(a, b origin) int {
		d := compareBreakRef(a.from, b.from)
		if d != 0 {
			return d
		}
		return cmp.Compare(a.line, b.line)
	})
	for _, o := range os {
		b := paths[o.from][o.line]
		sb.WriteString(fmt.Sprintf("BP[%s L:%2d] %v\n", o.from.String(), o.line, b))
	}
	return sb.String()
}

// tracer traces with key 'khipu.linebreak'.
func tracer() tracing.Trace {
	return tracing.Select("khipu.linebreak")
}
