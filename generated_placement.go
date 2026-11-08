package khipu

import (
	"fmt"
	"slices"

	"github.com/npillmayer/uax/bidi"
)

// placeGenerated completes a partial knot ordering without moving or changing
// the knots themselves. The only supported generated item is the validated
// paragraph trailer, marked during materialization. Its physical edge follows
// the PARAGRAPH direction, not the last text run's direction: right for LTR,
// left for RTL. Its W/MinW/MaxW, including finite or zero fill, are untouched.
//
// paragraphDir must be the direction used to resolve the source paragraph.
// visualLines supplies it from ResolvedLevels.ParagraphDirection(). Do not infer
// it from text runs, DirectionAt, or the owner's independent Params.BidiDir.
//
// Unknown zero-source knots and zero-source fragments have no attachment
// contract and fail atomically. LeftSkip/RightSkip are external breaker inputs,
// not materialized knots; this stage neither creates nor places them. The
// returned indices own their slice; input orderings and logical lines remain
// unchanged, including on failure. This is ordering, not width allocation.
func (line logicalLine) placeGenerated(order *knotOrdering, paragraphDir bidi.Direction) ([]int, error) {
	bad := func(reason string) ([]int, error) {
		return nil, fmt.Errorf("%w: generated placement: %s", ErrIllegalArguments, reason)
	}
	if order == nil || (paragraphDir != bidi.LeftToRight && paragraphDir != bidi.RightToLeft) {
		return bad("missing ordering or invalid paragraph direction")
	}
	seen := make([]bool, len(line.knots))
	fill := -1
	for group, indices := range [][]int{order.visual, order.unresolved} {
		for _, i := range indices {
			if i < 0 || i >= len(seen) || seen[i] {
				return bad("invalid or duplicate knot index")
			}
			seen[i] = true
			k := line.knots[i]
			if group == 0 {
				if k.paragraphFill || k.source.byteStart >= k.source.byteEnd {
					return bad("source-less knot already in visual mapping")
				}
				continue
			}
			if !k.paragraphFill || fill >= 0 || i != len(line.knots)-1 || k.origin != line.breakpoint ||
				k.fragment != logicalOriginal || k.box != 0 || k.core.Kind != KTGlue ||
				k.core.IsDiscardable() || k.core.Penalty > MaxMerit || k.core.Len != 0 ||
				k.source.byteStart != line.byteEnd || k.source.byteEnd != line.byteEnd {
				return bad("unrecognized or invalid generated material")
			}
			fill = i
		}
	}
	if slices.Contains(seen, false) {
		return bad("incomplete knot mapping")
	}
	result := make([]int, 0, len(line.knots))
	if fill >= 0 && paragraphDir == bidi.RightToLeft {
		result = append(result, fill)
	}
	result = append(result, order.visual...)
	if fill >= 0 && paragraphDir == bidi.LeftToRight {
		result = append(result, fill)
	}
	return result, nil
}
