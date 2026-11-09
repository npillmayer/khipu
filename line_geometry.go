package khipu

import (
	"context"
	"fmt"
	"math"
	"slices"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
)

// geometryParagraph is elastic visual content with complete line geometry, not
// frozen output. It owns line/knot snapshots, retaining immutable arena/source
// data under the visualParagraph lifetime contract. The shape is not retained.
type geometryParagraph struct {
	paragraph *styled.Paragraph
	arena     shapingArena
	lines     []geometryLine
}

// geometryLine's physical sequence is leftSkip, content.knots, rightSkip in
// both LTR and RTL paragraphs. Explicit edge slots avoid fabricated source
// positions or knot identities. Skips are not subject to trimming or reordering;
// their role, not Kind/Flags/Penalty, determines their treatment as edge glue.
// The exact supplied cores are retained, but only W/MinW/MaxW affect geometry.
//
// content.naturalWidth excludes side skips and already counts final ParFillSkip.
// naturalWidth includes both skips once; delta = targetWidth - naturalWidth.
// Positive delta requests stretching, negative delta shrinking. No width has
// been changed and no assertion of sufficient capacity is made at this stage.
type geometryLine struct {
	content                   visualLine
	leftSkip, rightSkip       KnotCore
	targetWidth, naturalWidth dimen.DU
	delta                     dimen.DU
}

// materializeLineGeometry uses the SAME shape and skip values as the completed
// breaker run. Callers must keep them stable: breakpoint indices do not encode
// geometry, so we cannot detect a different but structurally valid parameter set.
// No breaker defaults are substituted. The result is private until every stage
// succeeds; failure does not change the elastic khipu or selected discretionaries.
func (kq *Khipukamayuq) materializeLineGeometry(ctx context.Context, k *Khipu, breaks []int, params Params) (*geometryParagraph, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, err := kq.materializeVisualLines(ctx, k, breaks)
	if err != nil {
		return nil, err
	}
	return p.prepareGeometry(ctx, params)
}

// prepareGeometry snapshots one target per line in logical paragraph order,
// independent of each line's visual direction. A caller-supplied shape must be
// stable and side-effect free; calls already made cannot be undone on failure.
// The result never retains the shape or aliases mutable line/knot slices.
//
// Finite natural widths and targets must stay below dimen.Fil. Fill orders in
// MaxW are retained per item, never numerically summed as capacities. Oversized
// totals and inconsistent width triples fail rather than saturate as breaker
// summaries do. Negative finite skip widths are allowed if the total remains
// nonnegative. Fit checks and allocation belong to the next stage.
func (p *visualParagraph) prepareGeometry(ctx context.Context, params Params) (*geometryParagraph, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bad := func(reason string) (*geometryParagraph, error) {
		return nil, fmt.Errorf("%w: line geometry: %s", ErrIllegalArguments, reason)
	}
	if p == nil || p.paragraph == nil || len(p.lines) == 0 || int64(len(p.lines)) > math.MaxInt32 {
		return bad("missing or invalid visual paragraph/shape")
	}
	if err := params.ValidateGeometry(); err != nil {
		return nil, err
	}
	shape, leftSkip, rightSkip := params.LineShape, params.LeftSkip, params.RightSkip
	result := &geometryParagraph{paragraph: p.paragraph, arena: p.arena, lines: make([]geometryLine, len(p.lines))}
	for i, line := range p.lines {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		target := shape.LineLength(int32(i + 1))
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if target < 0 || target >= dimen.Fil {
			return bad(fmt.Sprintf("line %d has invalid target width", i+1))
		}
		var width int64
		for _, knot := range line.knots {
			if !validGeometryWidths(knot.core) {
				return bad(fmt.Sprintf("line %d has invalid knot widths", i+1))
			}
			w := int64(knot.core.W)
			if (w > 0 && width > math.MaxInt64-w) || (w < 0 && width < math.MinInt64-w) {
				return bad(fmt.Sprintf("line %d natural-width sum overflow", i+1))
			}
			width += w
		}
		if width < 0 || width >= int64(dimen.Fil) || width != int64(line.naturalWidth) {
			return bad(fmt.Sprintf("line %d has inconsistent natural width", i+1))
		}
		width += int64(leftSkip.W) + int64(rightSkip.W)
		if width < 0 || width >= int64(dimen.Fil) {
			return bad(fmt.Sprintf("line %d total natural width out of range", i+1))
		}
		// Both endpoints are nonnegative finite dimensions, so their signed
		// difference fits in DU. Use widened arithmetic before narrowing.
		line.knots = slices.Clone(line.knots)
		result.lines[i] = geometryLine{content: line, leftSkip: leftSkip, rightSkip: rightSkip,
			targetWidth: target, naturalWidth: dimen.DU(width), delta: dimen.DU(int64(target) - width)}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// validGeometryWidths admits finite signed bounds and the three positive fill
// orders as maxima. Infinity is not a fill order; infinite natural widths and
// infinite shrink bounds have no supported geometry contract.
func validGeometryWidths(k KnotCore) bool {
	return k.MinW > -dimen.Fil && k.MinW < dimen.Fil && k.W > -dimen.Fil && k.W < dimen.Fil &&
		k.MinW <= k.W && k.W <= k.MaxW && k.MaxW <= dimen.Filll
}
