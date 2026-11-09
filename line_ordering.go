package khipu

import (
	"context"
	"fmt"

	"github.com/npillmayer/uax/bidi"
)

// trailingGlueL1 implements a deliberately approximate L1 policy using only
// already classified knots. Call on the untrimmed logical line: the trailing
// glue usually disappears during trimAndMeasure, but its source still needs
// bidi treatment. Width, elasticity and KFDiscardable are not L1 criteria.
//
// Only a contiguous source-glue suffix qualifies. The validated paragraph
// trailer has no source and is ignored; other zero-source items, textboxes
// (including silent replacements), kerns and unexplained source gaps stop the
// scan. Clip to the line's exact source coverage, since a suppressed marker may
// extend a glue span past its breakpoint. Do not infer characters in gaps.
//
// This is NOT full UAX#9 L1: glue may include non-WS characters such as NBSP or
// suppressed soft hyphens; conversely, omitted isolate controls, whitespace
// embedded in text clusters and internal separators may be missed. We accept
// that trade-off rather than inspect original characters again during layout.
func (line logicalLine) trailingGlueL1(trailer int) sourceByteInterval {
	from := line.byteEnd
	for i := len(line.knots) - 1; i >= 0; i-- {
		item := line.knots[i]
		if item.fragment == logicalOriginal && item.origin == trailer {
			continue
		}
		if item.fragment != logicalOriginal || item.core.Kind != KTGlue {
			break
		}
		start := max(item.source.byteStart, line.byteStart)
		end := min(item.source.byteEnd, line.byteEnd)
		if start >= end || end != from {
			break
		}
		from = start
	}
	if from == line.byteEnd {
		return sourceByteInterval{}
	}
	return sourceByteInterval{from, line.byteEnd}
}

// lineOrderings prepares one bidi Ordering per logical line, but does not yet
// permute knots or place generated fill/skips. All positions remain relative to
// the original paragraph, including source which was trimmed from visible lines.
//
// Clone once per attempt, then Split -> ApplyL1 -> Reorder. Neither the original
// paragraph resolution nor materialized knots are mutated, even on failure.
// The result is private until every line succeeds. Source, styles and arena data
// must remain stable under logicalParagraph's ownership contract.
func (p *logicalParagraph) lineOrderings(ctx context.Context) ([]*bidi.Ordering, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bad := func() ([]*bidi.Ordering, error) {
		return nil, fmt.Errorf("%w: inconsistent logical-line bidi input", ErrIllegalArguments)
	}
	if p == nil || p.paragraph == nil || p.paragraph.BidiLevels() == nil || len(p.lines) == 0 {
		return bad()
	}
	length := p.paragraph.Raw().Len()
	var start uint64
	for _, line := range p.lines {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if line.byteStart != start || line.byteEnd < start || line.byteEnd > length {
			return bad()
		}
		if r := line.l1Glue; r != (sourceByteInterval{}) &&
			(r.from < start || r.from >= r.to || r.to != line.byteEnd) {
			return bad()
		}
		start = line.byteEnd
	}
	if start != length {
		return bad()
	}
	remaining := p.paragraph.BidiLevels().Clone()
	orders := make([]*bidi.Ordering, 0, len(p.lines))
	for i, line := range p.lines {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		levels := remaining
		if i < len(p.lines)-1 {
			levels, remaining = remaining.Split(line.byteEnd, false)
		}
		if r := line.l1Glue; r != (sourceByteInterval{}) {
			if err := levels.ApplyL1(r.from, r.to); err != nil {
				return nil, fmt.Errorf("khipu: line %d glue L1 reset: %w", i+1, err)
			}
		}
		orders = append(orders, levels.Reorder())
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}
