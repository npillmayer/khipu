package khipu

import (
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
)

// visualParagraph owns new line and knot slices in physical left-to-right order.
// It shares immutable source and published arena records under logicalParagraph's
// lifetime contract. Neither glyph sequences within boxes nor elastic widths
// change. This is not frozen output, and external left/right skips are absent.
type visualParagraph struct {
	paragraph *styled.Paragraph
	arena     shapingArena
	lines     []visualLine
}

// visualLine deliberately differs from logicalLine: its knots must not be fed
// back into logical trimming or source-order mapping. Source coverage and the
// ending breakpoint still describe the original logical line, not visual edges.
type visualLine struct {
	knots              []logicalKnot
	breakpoint         int
	byteStart, byteEnd uint64
	naturalWidth       dimen.DU
}

// materializeVisualLines connects a complete breaker decision to visual content.
// It inherits materializeLogicalLines' ownership and selection-validity contract.
// No shaping, discretionary discovery, side-skip insertion or width allocation
// occurs here. Errors return no partial paragraph and do not change the input.
func (kq *Khipukamayuq) materializeVisualLines(k *Khipu, breakpoints []int) (*visualParagraph, error) {
	p, err := kq.materializeLogicalLines(k, breakpoints)
	if err != nil {
		return nil, err
	}
	return p.visualLines()
}

// visualLines builds on the logical snapshot without consuming it. lineOrderings
// clones the retained bidi resolution before Split/ApplyL1/Reorder. Placement
// reads the authoritative paragraph direction, not an individual run's direction
// or an independently configured owner default. Only complete success publishes
// the result, so even a failure on a later line leaves the snapshot retryable.
func (p *logicalParagraph) visualLines() (*visualParagraph, error) {
	orders, err := p.lineOrderings()
	if err != nil {
		return nil, err
	}
	direction := p.paragraph.BidiLevels().ParagraphDirection()
	result := &visualParagraph{paragraph: p.paragraph, arena: p.arena, lines: make([]visualLine, len(p.lines))}
	for i, line := range p.lines {
		partial, err := line.mapOrdering(orders[i])
		if err != nil {
			return nil, fmt.Errorf("khipu: visual line %d mapping: %w", i+1, err)
		}
		indices, err := line.placeGenerated(partial, direction)
		if err != nil {
			return nil, fmt.Errorf("khipu: visual line %d placement: %w", i+1, err)
		}
		visual := visualLine{breakpoint: line.breakpoint, byteStart: line.byteStart, byteEnd: line.byteEnd,
			naturalWidth: line.naturalWidth, knots: make([]logicalKnot, len(indices))}
		for j, at := range indices {
			visual.knots[j] = line.knots[at]
		}
		result.lines[i] = visual
	}
	return result, nil
}
