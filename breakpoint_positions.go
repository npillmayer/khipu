package khipu

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

// BreakpointPositions maps line-ending knot indices to exclusive UTF-8 byte
// endpoints in the original paragraph. Positions are paragraph-relative, not
// adjusted by paragraph.Offset. Whitespace and omitted controls remain part of
// source coverage, irrespective of line-edge trimming. The final fill maps to
// the paragraph end, including for an empty paragraph.
//
// Call this with the breakpoint list and selections from the same linebreaking
// run: rerunning a breaker replaces SelectedDiscretionaries. Indices must be
// strictly increasing; partial and empty lists are allowed. This translates
// decisions, but does not check whether the supplied path is feasible or optimal.
// It neither discovers candidates nor changes knots, selections or bidi state.
//
// Ordinary streaming breaks use retained boundary provenance, which need not
// equal Pos+Len. Selected alternatives require a valid cached SourceSplit; their
// replacement lengths never determine source positions. Ordinary metric-encoded
// knots are supported through their original Pos/Len spans, but legacy metric
// discretionaries without SourceSplit cannot be mapped. Missing or inconsistent
// provenance returns an error wrapping ErrIllegalArguments and no partial result.
func (k *Khipu) BreakpointPositions(breakpoints []int) ([]uint64, error) {
	if k == nil || k.paragraph == nil {
		return nil, fmt.Errorf("%w: breakpoint positions require an encoded paragraph", ErrIllegalArguments)
	}
	positions := make([]uint64, len(breakpoints))
	previous := -1
	for i, at := range breakpoints {
		if at <= previous || at < 0 || at >= len(k.Kind) {
			return nil, fmt.Errorf("%w: breakpoint indices must be increasing and in bounds (knot %d)", ErrIllegalArguments, at)
		}
		pos, err := k.breakpointPosition(at)
		if err != nil {
			return nil, fmt.Errorf("breakpoint knot %d: %w", at, err)
		}
		if !k.isSourceByteBoundary(pos) || (i > 0 && pos < positions[i-1]) {
			return nil, fmt.Errorf("%w: invalid or decreasing source boundary at knot %d", ErrIllegalArguments, at)
		}
		positions[i], previous = pos, at
	}
	return positions, nil
}

// breakpointPosition separates selected replacement provenance from the ordinary
// boundary. A malformed selection must not silently fall back to a plain break.
func (k *Khipu) breakpointPosition(at int) (uint64, error) {
	if choice, selected := k.SelectedDiscretionaryAt(at); selected {
		if choice.Source < 0 || choice.Source >= len(k.Kind) || k.Kind[choice.Source] != KTTextBox || choice.Variant == 0 {
			return 0, fmt.Errorf("%w: invalid discretionary selection", ErrIllegalArguments)
		}
		var split SourceSplit
		matches := 0
		for _, candidate := range k.DiscretionariesAt(choice.Source) {
			if candidate.Variant == choice.Variant {
				split = candidate.SourceSplit
				matches++
			}
		}
		if matches != 1 || !split.Valid {
			return 0, fmt.Errorf("%w: selected discretionary needs one cached variant with known SourceSplit", ErrIllegalArguments)
		}
		start, end, err := k.knotSourceBytes(choice.Source)
		if err != nil {
			return 0, err
		}
		if split.ByteOffset <= start || split.ByteOffset >= end {
			return 0, fmt.Errorf("%w: discretionary split outside source interior", ErrIllegalArguments)
		}
		return split.ByteOffset, nil
	}
	_, end, err := k.knotSourceBytes(at)
	if err != nil {
		return 0, err
	}
	if k.shaping.sources != nil {
		// A boundary in a glyphless gap may lie beyond the knot's source span;
		// a suppressed marker suffix may instead extend the span past the break.
		return k.shaping.sources[at].breakAfter.byteEnd, nil
	}
	return end, nil
}

// knotSourceBytes reads original spans only. Streaming provenance takes
// precedence; missing entries must not be concealed by a metric-plane fallback.
func (k *Khipu) knotSourceBytes(at int) (uint64, uint64, error) {
	var start, end uint64
	if k.shaping.sources != nil {
		if at >= len(k.shaping.sources) {
			return 0, 0, fmt.Errorf("%w: missing knot provenance", ErrIllegalArguments)
		}
		source := k.shaping.sources[at].source
		start, end = source.byteStart, source.byteEnd
	} else {
		if at >= len(k.Pos) || at >= len(k.Len) || k.Pos[at] < k.paragraph.Offset {
			return 0, 0, fmt.Errorf("%w: missing or invalid knot source span", ErrIllegalArguments)
		}
		start = k.Pos[at] - k.paragraph.Offset
		end = start + uint64(k.Len[at])
	}
	if end < start || end > k.paragraph.Raw().Len() {
		return 0, 0, fmt.Errorf("%w: knot source span out of bounds", ErrIllegalArguments)
	}
	return start, end, nil
}

// isSourceByteBoundary avoids flattening the cord: streaming paragraphs retain
// every scalar endpoint; the metric path needs only the byte at the proposed
// split to rule out a UTF-8 continuation byte. Encoders already validated UTF-8.
func (k *Khipu) isSourceByteBoundary(pos uint64) bool {
	length := k.paragraph.Raw().Len()
	if pos > length {
		return false
	}
	if pos == 0 || pos == length {
		return true
	}
	if k.shaping.sources != nil {
		b := k.shaping.boundaries
		i := sort.Search(len(b), func(i int) bool { return b[i].byteEnd >= pos })
		return i < len(b) && b[i].byteEnd == pos
	}
	for _, b := range k.paragraph.Raw().ByteRangeBounded(pos, pos+1) {
		return utf8.RuneStart(b)
	}
	return false
}
