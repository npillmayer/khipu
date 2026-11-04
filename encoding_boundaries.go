package khipu

import (
	"unicode/utf8"

	"github.com/npillmayer/uax/uax14"
)

// paragraphBoundaries evaluates UAX#14 over the complete original paragraph,
// before intersecting with style and cluster boundaries. Driving the breaker
// directly avoids the segmenter's mixed byte/rune bounds and synthetic EOF text.
// Penalties() reports contributions backwards from the most recently fed rune.
func paragraphBoundaries(source string) []Penalty {
	result := make([]Penalty, len(source)+1)
	for i := range result {
		result[i] = MaxPenalty
	}
	ends := make([]int, 0, utf8.RuneCountInString(source)+1)
	totals := make([]int64, 0, cap(ends))
	breaker := uax14.NewLineWrap()
	feed := func(r rune, end int) {
		ends = append(ends, end)
		totals = append(totals, 0)
		class := breaker.CodePointClassFor(r)
		breaker.StartRulesFor(r, class)
		breaker.ProceedWithRune(r, class)
		for back, p := range breaker.Penalties() {
			at := len(totals) - 1 - back
			if at >= 0 {
				totals[at] += int64(p)
			}
		}
	}
	for at, r := range source {
		size := utf8.RuneLen(r)
		// The breaker reserves NUL as its EOF marker. Treat a source NUL as a
		// replacement character for boundary classification only.
		if r == 0 {
			r = utf8.RuneError
		}
		feed(r, at+size)
	}
	feed(0, len(source))
	for i, end := range ends[:len(ends)-1] {
		result[end] = normPenalty(totals[i])
	}
	return result
}
