package khipu

import (
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
)

// logicalParagraph is an internal, unfrozen snapshot of one complete break
// decision. Cores and line slices are owned; arena slice headers retain immutable
// published shaping records, even after arena growth or unwiring the input.
// Source text, styles, font services and published arena records must stay
// immutable. No bidi state is consumed, glyphs reordered or widths adjusted.
type logicalParagraph struct {
	paragraph *styled.Paragraph
	arena     shapingArena
	lines     []logicalLine
}

type logicalLine struct {
	knots              []logicalKnot
	breakpoint         int
	byteStart, byteEnd uint64             // untrimmed original paragraph coverage
	naturalWidth       dimen.DU           // excludes external left/right skips
	l1Glue             sourceByteInterval // trailing source glue, recorded BEFORE trimming
}

// sourceByteInterval is half-open and paragraph-relative. Its zero value means
// no interval. It describes original UTF-8 input, never generated glyph bytes.
type sourceByteInterval struct{ from, to uint64 }

type logicalFragment uint8

const (
	logicalOriginal logicalFragment = iota
	logicalPreBreak
	logicalPostBreak
)

// logicalKnot keeps geometry separate from original provenance and shaping
// input. box indexes the result's retained arena, not an unowned global table.
// Both replacement halves identify the original knot and selected variant;
// their glyph coordinates belong to their stored replacement input instead.
type logicalKnot struct {
	core          KnotCore
	source        runSourceRange
	box           shapedBoxID
	origin        int
	variant       uint16
	fragment      logicalFragment
	paragraphFill bool // identity of the validated generated trailer, not inferred from dimensions
}

// materializeLogicalLines resolves a complete result from either breaker. It
// never discovers candidates: a selected variant must already have stored
// fragments. Validation and construction are private and atomic; on failure
// nothing is published or changed on k. Concurrent owner operations are not
// supported. Call before another breaker replaces the selections. Without a
// breaker generation token, a stale but structurally identical result cannot
// be distinguished from the current one.
//
// The result preserves elasticity for later allocation; it is not FrozenKhipu.
// Targets, side skips, fit/optimality checks and visual ordering are out of scope.
func (kq *Khipukamayuq) materializeLogicalLines(k *Khipu, breakpoints []int) (*logicalParagraph, error) {
	if kq == nil {
		return nil, ErrVoidKhipukamayuq
	}
	bad := func() (*logicalParagraph, error) {
		return nil, fmt.Errorf("%w: inconsistent logical-line input", ErrIllegalArguments)
	}
	if k == nil || k.owner != kq || k.paragraph == nil {
		return bad()
	}
	n := len(k.Kind)
	for _, length := range []int{len(k.W), len(k.MinW), len(k.MaxW), len(k.Pos), len(k.Len), len(k.Penalty), len(k.Flags), len(k.boxRefs), len(k.shaping.sources)} {
		if length != n {
			return bad()
		}
	}
	if n == 0 || len(breakpoints) == 0 || breakpoints[len(breakpoints)-1] != n-1 {
		return bad()
	}
	trailer := k.KnotByIndex(n - 1)
	end := k.paragraph.Raw().Len()
	if trailer.Kind != KTGlue || trailer.IsDiscardable() || trailer.Penalty > MaxMerit ||
		trailer.Len != 0 || trailer.Pos != k.paragraph.Offset+end ||
		k.shaping.sources[n-1].source.byteStart != end || k.shaping.sources[n-1].source.byteEnd != end {
		return bad()
	}
	ends, err := k.BreakpointPositions(breakpoints)
	if err != nil {
		return nil, err
	}
	if ends[len(ends)-1] != end {
		return bad()
	}
	result := &logicalParagraph{paragraph: k.paragraph, arena: k.shaping}
	var carry *logicalKnot
	var start uint64
	previous, selected := -1, 0
	for lineIndex, at := range breakpoints {
		line := logicalLine{breakpoint: at, byteStart: start, byteEnd: ends[lineIndex]}
		if carry != nil {
			line.knots = append(line.knots, *carry)
			carry = nil
		}
		for i := previous + 1; i <= at; i++ {
			// Forced breaks cannot be skipped, including by a replacement.
			if i < at && k.Penalty[i] <= MaxMerit {
				return bad()
			}
			if choice, ok := k.SelectedDiscretionaries[i]; ok {
				if i != at || i == n-1 || choice.Source != i || k.Penalty[i] <= MaxMerit {
					return bad()
				}
				// BreakpointPositions already checked unique variant and SourceSplit.
				for _, candidate := range k.DiscretionariesAt(i) {
					if candidate.Variant != choice.Variant {
						continue
					}
					pre, err := k.logicalReplacement(i, candidate, true)
					if err != nil {
						return nil, err
					}
					post, err := k.logicalReplacement(i, candidate, false)
					if err != nil {
						return nil, err
					}
					if pre.source.scalarEnd != post.source.scalarStart {
						return bad()
					}
					line.knots = append(line.knots, pre)
					carry = &post
				}
				selected++
				continue
			}
			if i == at && k.Penalty[i] >= MaxPenalty {
				return bad()
			}
			item := logicalKnot{core: k.KnotByIndex(i), source: k.shaping.sources[i].source, box: k.boxRefs[i], origin: i}
			item.paragraphFill = i == n-1
			if err := k.validateLogicalKnot(item); err != nil {
				return nil, err
			}
			line.knots = append(line.knots, item)
		}
		line.l1Glue = line.trailingGlueL1(n - 1)
		if err := line.trimAndMeasure(); err != nil {
			return nil, err
		}
		result.lines = append(result.lines, line)
		previous, start = at, line.byteEnd
	}
	if selected != len(k.SelectedDiscretionaries) || carry != nil {
		return bad() // includes stale/out-of-bounds selection keys
	}
	return result, nil
}

// logicalReplacement resolves already published glyphs; replacement spelling
// must never be used to infer original input spans or a successor-line offset.
func (k *Khipu) logicalReplacement(at int, c DiscretionaryCandidate, pre bool) (logicalKnot, error) {
	item := logicalKnot{core: c.PostBreak, box: c.postBox, origin: at, variant: c.Variant, fragment: logicalPostBreak}
	if pre {
		item.core, item.box, item.fragment = c.PreBreak, c.preBox, logicalPreBreak
	}
	if item.box <= 0 || int(item.box) > len(k.shaping.boxes) {
		return logicalKnot{}, fmt.Errorf("%w: selected fragment has no shaping data", ErrIllegalArguments)
	}
	item.source = k.shaping.boxes[item.box-1].source
	original := k.shaping.sources[at].source
	if (pre && (item.source.byteStart != original.byteStart || item.source.scalarStart != original.scalarStart || item.source.byteEnd != c.SourceSplit.ByteOffset)) ||
		(!pre && (item.source.byteStart != c.SourceSplit.ByteOffset || item.source.byteEnd != original.byteEnd || item.source.scalarEnd != original.scalarEnd)) {
		return logicalKnot{}, fmt.Errorf("%w: inconsistent fragment provenance", ErrIllegalArguments)
	}
	return item, k.validateLogicalKnot(item)
}

// validateLogicalKnot checks associations at this phase boundary, not the full
// shaping protocol already checked at publication. Silent replacement boxes are
// valid and retain a box reference even when their glyph range is empty.
func (k *Khipu) validateLogicalKnot(item logicalKnot) error {
	bad := func() error { return fmt.Errorf("%w: invalid logical knot at %d", ErrIllegalArguments, item.origin) }
	s, c := item.source, item.core
	if s.byteStart > s.byteEnd || s.byteEnd > k.paragraph.Raw().Len() || s.scalarStart > s.scalarEnd ||
		c.Pos < k.paragraph.Offset || c.Pos-k.paragraph.Offset != s.byteStart || uint64(c.Len) != s.byteEnd-s.byteStart ||
		c.MinW > c.W || c.W > c.MaxW || c.W >= dimen.Fil || c.W <= -dimen.Fil {
		return bad()
	}
	if c.Kind != KTTextBox {
		if item.box != 0 || item.fragment != logicalOriginal {
			return bad()
		}
		return nil
	}
	if item.box <= 0 || int(item.box) > len(k.shaping.boxes) {
		return bad()
	}
	b := k.shaping.boxes[item.box-1]
	validRange := func(r arenaRange, size int) bool { return r.start >= 0 && r.start <= r.end && r.end <= size }
	if b.source != s || b.context < 0 || b.context >= len(k.shaping.contexts) ||
		!validRange(b.glyphs, len(k.shaping.glyphs)) || !validRange(b.clusters, len(k.shaping.clusters)) ||
		(b.replacement != nil) != (item.fragment != logicalOriginal) {
		return bad()
	}
	if b.replacement == nil && (b.glyphs.start == b.glyphs.end || b.clusters.start == b.clusters.end) {
		return bad()
	}
	if b.replacement != nil && b.replacement.provenance != s {
		return bad()
	}
	return nil
}

// trimAndMeasure mirrors both breakers: only a non-discardable textbox ends
// leading trim or clears trailing trim. Neutral retained knots (including final
// fill) survive but do not shelter adjacent discardables. An all-neutral line
// therefore keeps neutrals and discards every flagged knot. No Unicode guesses
// or new first-line policy belong here.
func (line *logicalLine) trimAndMeasure() error {
	first, last := len(line.knots), -1
	for i, item := range line.knots {
		if item.core.Kind == KTTextBox && !item.core.IsDiscardable() {
			first, last = min(first, i), i
		}
	}
	kept := line.knots[:0]
	var width int64
	for i, item := range line.knots {
		if item.core.IsDiscardable() && (i < first || i > last) {
			continue
		}
		kept = append(kept, item)
		width += int64(item.core.W)
	}
	if width < 0 || width >= int64(dimen.Fil) {
		return fmt.Errorf("%w: logical line width out of range", ErrIllegalArguments)
	}
	line.knots, line.naturalWidth = kept, dimen.DU(width)
	return nil
}
