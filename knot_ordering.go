package khipu

import (
	"fmt"
	"slices"
	"sort"

	"github.com/npillmayer/uax/bidi"
)

// knotOrdering is a partial permutation of logicalLine.knots, not a visual line.
// visual contains source-backed knots, including selected replacement fragments,
// in physical left-to-right order. unresolved contains original zero-source
// material in logical order. Every input knot occurs in exactly one list on
// success. A caller must resolve zero-source attachment before publishing a
// complete visual line.
//
// Indices move whole knots, keeping cores, box references and glyph arrays
// untouched. They are local slice indices, not original Khipu knot indices.
type knotOrdering struct {
	visual     []int
	unresolved []int
}

// mapOrdering translates bidi's visual segment traversal into a knot permutation.
// Ordering creation (including L1) belongs to lineOrderings; this method neither
// consumes the Ordering nor changes the logical line. It does not place generated
// fill, allocate widths, or reverse glyphs in boxes. Selected fragments use their
// original source provenance, never the length or clusters of replacement input.
func (line logicalLine) mapOrdering(order *bidi.Ordering) (*knotOrdering, error) {
	if order == nil {
		return nil, fmt.Errorf("%w: missing line ordering", ErrIllegalArguments)
	}
	var segments []bidi.Segment
	for _, run := range order.Runs {
		segments = append(segments, slices.Collect(run.Segments(run.IsOpposite(bidi.LeftToRight)))...)
	}
	return line.mapVisualSegments(segments)
}

// mapVisualSegments accepts segments already in physical left-to-right order.
// Source gaps are legal: trimming and omitted controls need not produce knots.
// However, each source-backed knot, including a selected fragment, must fit
// wholly in one segment. Silent fragments still participate through provenance.
// A genuinely zero-source fragment cannot identify its segment; reject it rather
// than guess a direction from a neighboring box. Original zero-source material
// remains unresolved for the separate generated-material placement stage.
// Even adjacent same-direction segments are not merged here: a boundary inside
// a knot needs an explicit compatibility policy, not an arbitrary anchor choice.
// This conservative mapper rejects such input rather than splitting a box
// or emitting it twice. It applies the same indivisibility rule to glue/kerns.
//
// A separate source-sorted index allows binary lookup without changing visual
// traversal. Cost is O(S log S + K log S), with O(S + K) temporary storage for
// S segments and K knots. No source text, glyphs or arena records are copied.
func (line logicalLine) mapVisualSegments(segments []bidi.Segment) (*knotOrdering, error) {
	bad := func(reason string) (*knotOrdering, error) {
		return nil, fmt.Errorf("%w: knot ordering: %s", ErrIllegalArguments, reason)
	}
	if line.byteStart > line.byteEnd {
		return bad("reversed line source interval")
	}
	bySource := make([]int, len(segments))
	for i, s := range segments {
		if s.From >= s.To || s.From < line.byteStart || s.To > line.byteEnd ||
			(s.Dir != bidi.LeftToRight && s.Dir != bidi.RightToLeft) {
			return bad(fmt.Sprintf("invalid segment %d", i))
		}
		bySource[i] = i
	}
	sort.Slice(bySource, func(i, j int) bool { return segments[bySource[i]].From < segments[bySource[j]].From })
	for i := 1; i < len(bySource); i++ {
		if segments[bySource[i-1]].To > segments[bySource[i]].From {
			return bad("overlapping source segments")
		}
	}
	buckets := make([][]int, len(segments))
	result := &knotOrdering{}
	previousEnd := line.byteStart
	for i, knot := range line.knots {
		from, to := knot.source.byteStart, knot.source.byteEnd
		if from > to || from < previousEnd || to > line.byteEnd {
			return bad(fmt.Sprintf("invalid or unordered source span for knot %d", i))
		}
		previousEnd = to
		switch knot.fragment {
		case logicalPreBreak, logicalPostBreak:
			if knot.core.Kind != KTTextBox ||
				(knot.fragment == logicalPreBreak && to != line.byteEnd) ||
				(knot.fragment == logicalPostBreak && from != line.byteStart) {
				return bad(fmt.Sprintf("invalid fragment placement for knot %d", i))
			}
			if from == to {
				return bad(fmt.Sprintf("zero-source fragment %d requires an explicit attachment rule", i))
			}
		case logicalOriginal:
		default:
			return bad(fmt.Sprintf("unknown fragment role for knot %d", i))
		}
		if from == to {
			result.unresolved = append(result.unresolved, i)
			continue
		}
		at := sort.Search(len(bySource), func(j int) bool { return segments[bySource[j]].To > from })
		if at == len(bySource) {
			return bad(fmt.Sprintf("unmapped knot %d", i))
		}
		segmentIndex := bySource[at]
		s := segments[segmentIndex]
		if from < s.From || to > s.To {
			return bad(fmt.Sprintf("knot %d crosses a segment boundary or source gap", i))
		}
		buckets[segmentIndex] = append(buckets[segmentIndex], i)
	}
	for i, s := range segments {
		// Segment iteration orders intervals, not the knots within an interval.
		// Reverse those knots for RTL, but leave each shaped box intact.
		if s.Dir == bidi.RightToLeft {
			slices.Reverse(buckets[i])
		}
		result.visual = append(result.visual, buckets[i]...)
	}
	return result, nil
}
