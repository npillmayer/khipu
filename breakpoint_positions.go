package khipu

import (
	"fmt"
	"sort"
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
// Ordinary breaks use retained boundary provenance, which need not equal
// Pos+Len. Selected alternatives require a valid cached SourceSplit; their
// replacement lengths never determine source positions. Manually constructed
// knots without retained source provenance cannot be mapped. Missing or inconsistent
// provenance returns an error wrapping ErrIllegalArguments and no partial result.
func (k *Khipu) BreakpointPositions(breakpoints []int) ([]uint64, error) {
	if k == nil || k.paragraph == nil || k.shaping.sources == nil || len(k.shaping.sources) != len(k.Kind) {
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
	if _, _, err := k.knotSourceBytes(at); err != nil {
		return 0, err
	}
	// Glyphless gaps and suppressed markers may separate the boundary from
	// the knot's source span; Pos+Len is not a substitute for this endpoint.
	return k.shaping.sources[at].breakAfter.byteEnd, nil
}

// knotSourceBytes reads retained original spans, never replacement lengths.
func (k *Khipu) knotSourceBytes(at int) (uint64, uint64, error) {
	if at < 0 || at >= len(k.shaping.sources) {
		return 0, 0, fmt.Errorf("%w: missing knot provenance", ErrIllegalArguments)
	}
	source := k.shaping.sources[at].source
	start, end := source.byteStart, source.byteEnd
	if end < start || end > k.paragraph.Raw().Len() {
		return 0, 0, fmt.Errorf("%w: knot source span out of bounds", ErrIllegalArguments)
	}
	return start, end, nil
}

// isSourceByteBoundary checks retained scalar endpoints without flattening the cord.
func (k *Khipu) isSourceByteBoundary(pos uint64) bool {
	length := k.paragraph.Raw().Len()
	if pos > length {
		return false
	}
	if pos == 0 || pos == length {
		return true
	}
	b := k.shaping.boundaries
	i := sort.Search(len(b), func(i int) bool { return b[i].byteEnd >= pos })
	return i < len(b) && b[i].byteEnd == pos
}
