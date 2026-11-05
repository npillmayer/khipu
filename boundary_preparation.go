package khipu

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/uax"
	"github.com/npillmayer/uax/uax14"
	"github.com/npillmayer/uax/uax29"
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
	// wordBoundary is a default UAX#29 knot-partition hint, independent of
	// the UAX#14 penalty. It neither permits a line break nor splits a glyph
	// cluster. Language-specific dictionary segmentation is not provided here.
	wordBoundary bool
	// softHyphen records an explicit opportunity even when it lies inside a
	// shaped cluster. Streaming no-break preparation suppresses its ordinary
	// penalty; a later discretionary provider must supply replacement fragments.
	softHyphen bool
}

// prepareParagraphBoundaries analyzes the intact original paragraph before
// shaping begins. Style and bidi boundaries never reset Unicode analysis.
// Returned penalties and word hints are finalized immutable metadata. The two
// analyses stay independent: word hints never contribute to line demerits.
// This preparation-only path does not change the active metric encoder.
func prepareParagraphBoundaries(para *styled.Paragraph) ([]sourceBoundary, error) {
	if para == nil {
		return nil, fmt.Errorf("%w: boundary preparation requires a paragraph", contracts.ErrIllegalArguments)
	}
	return readParagraphBoundaries(para.Reader())
}

// readParagraphBoundaries retains endpoints and accumulated penalties, not a
// copy of the source text. Both breakers can revise earlier positions, so finalization
// must wait until EOF contributions have been applied. On failure no partial
// table is returned. The reader seam also permits injected I/O failures in tests.
func readParagraphBoundaries(source io.Reader) ([]sourceBoundary, error) {
	if nilRunService(source) {
		return nil, fmt.Errorf("%w: missing boundary source", contracts.ErrIllegalArguments)
	}
	reader := bufio.NewReader(source)
	breaker := uax14.NewLineWrap()
	words := uax29.NewWordBreaker(1)
	var result []sourceBoundary
	var totals []int64
	var wordTotals []int64
	var mandatory []bool
	previousCR := false
	accumulate := func(b uax.UnicodeBreaker, r rune, sums []int64) []int64 {
		sums = append(sums, 0)
		class := b.CodePointClassFor(r)
		b.StartRulesFor(r, class)
		b.ProceedWithRune(r, class)
		for back, p := range b.Penalties() {
			if at := len(sums) - 1 - back; at >= 0 {
				sums[at] += int64(p)
			}
		}
		return sums
	}
	feed := func(r rune) {
		// Both engines may revise preceding positions. Never mix their
		// contributions: a word boundary must not change line feasibility.
		totals = accumulate(breaker, r, totals)
		wordTotals = accumulate(words, r, wordTotals)
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
		result = append(result, sourceBoundary{byteEnd: bytes, scalarEnd: scalars, softHyphen: r == '\u00ad'})
		// NUL is the breaker's EOF marker. Preserve source coordinates but
		// substitute U+FFFD for classification, matching the metric encoder.
		if r == 0 {
			r = utf8.RuneError
		}
		// Preserve LB4/LB5 precedence explicitly. The upstream additive rule
		// contributions can otherwise cancel a required break before another
		// hard separator (e.g. LF LF). CR LF is one separator, not two.
		class := uax14.UAX14Class(breaker.CodePointClassFor(r))
		if previousCR && class != uax14.LFClass {
			mandatory[len(mandatory)-1] = true
		}
		mandatory = append(mandatory, class == uax14.BKClass || class == uax14.LFClass || class == uax14.NLClass)
		previousCR = class == uax14.CRClass
		feed(r)
	}
	if previousCR {
		mandatory[len(mandatory)-1] = true
	}
	feed(0)
	for i := range result {
		result[i].penalty = normPenalty(totals[i])
		if mandatory[i] {
			result[i].penalty = MaxMerit
		}
		result[i].wordBoundary = wordTotals[i] < int64(uax.InfinitePenalty)
	}
	return result, nil
}
