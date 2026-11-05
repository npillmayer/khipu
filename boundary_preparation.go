package khipu

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/uax/uax14"
)

// sourceBoundary describes the position after one original source scalar.
// Endpoints are exclusive and paragraph-relative, including controls omitted
// from shaping. Entries include prohibited breaks. These are source penalties,
// not permission to split a shaped cluster or replacements for the terminal
// paragraph-fill knot. There is no synthetic EOF entry or initial-position entry.
type sourceBoundary struct {
	byteEnd   uint64
	scalarEnd uint64
	penalty   Penalty
}

// prepareParagraphBoundaries analyzes the intact original paragraph before
// shaping begins. Style and bidi boundaries never reset Unicode analysis.
// Returned penalties are finalized and may be shared as immutable metadata.
// This preparation-only path does not change the active metric encoder.
func prepareParagraphBoundaries(para *styled.Paragraph) ([]sourceBoundary, error) {
	if para == nil {
		return nil, fmt.Errorf("%w: boundary preparation requires a paragraph", contracts.ErrIllegalArguments)
	}
	return readParagraphBoundaries(para.Reader())
}

// readParagraphBoundaries retains endpoints and accumulated penalties, not a
// copy of the source text. UAX#14 can revise earlier positions, so normalization
// must wait until EOF contributions have been applied. On failure no partial
// table is returned. The reader seam also permits injected I/O failures in tests.
func readParagraphBoundaries(source io.Reader) ([]sourceBoundary, error) {
	if nilRunService(source) {
		return nil, fmt.Errorf("%w: missing boundary source", contracts.ErrIllegalArguments)
	}
	reader := bufio.NewReader(source)
	breaker := uax14.NewLineWrap()
	var result []sourceBoundary
	var totals []int64
	feed := func(r rune) {
		totals = append(totals, 0)
		class := breaker.CodePointClassFor(r)
		breaker.StartRulesFor(r, class)
		breaker.ProceedWithRune(r, class)
		for back, p := range breaker.Penalties() {
			if at := len(totals) - 1 - back; at >= 0 {
				totals[at] += int64(p)
			}
		}
	}
	var bytes, scalars uint64
	for {
		r, size, err := reader.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("khipu: reading boundary source: %w", err)
		}
		if r == utf8.RuneError && size == 1 {
			return nil, fmt.Errorf("%w: invalid UTF-8 boundary source", contracts.ErrIllegalArguments)
		}
		if bytes > math.MaxUint64-uint64(size) || scalars == math.MaxUint64 {
			return nil, fmt.Errorf("%w: boundary coordinate overflow", contracts.ErrIllegalArguments)
		}
		bytes += uint64(size)
		scalars++
		result = append(result, sourceBoundary{byteEnd: bytes, scalarEnd: scalars})
		// NUL is the breaker's EOF marker. Preserve source coordinates but
		// substitute U+FFFD for classification, matching the metric encoder.
		if r == 0 {
			r = utf8.RuneError
		}
		feed(r)
	}
	feed(0)
	for i := range result {
		result[i].penalty = normPenalty(totals[i])
	}
	return result, nil
}
