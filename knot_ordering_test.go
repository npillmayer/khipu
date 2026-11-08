package khipu

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/uax/bidi"
)

func mappingKnot(from, to uint64) logicalKnot {
	return logicalKnot{core: KnotCore{Kind: KTTextBox}, source: runSourceRange{byteStart: from, byteEnd: to}}
}

func TestKnotOrderingSegments(t *testing.T) {
	ltr, rtl := bidi.LeftToRight, bidi.RightToLeft
	for _, tc := range []struct {
		name       string
		knots      []logicalKnot
		segments   []bidi.Segment
		want       []int
		unresolved []int
	}{
		{"LTR", []logicalKnot{mappingKnot(0, 2), mappingKnot(2, 4)}, []bidi.Segment{{From: 0, To: 4, Dir: ltr}}, []int{0, 1}, nil},
		{"RTL within segment", []logicalKnot{mappingKnot(0, 2), mappingKnot(2, 4)}, []bidi.Segment{{From: 0, To: 4, Dir: rtl}}, []int{1, 0}, nil},
		{"mixed", []logicalKnot{mappingKnot(0, 1), mappingKnot(1, 2), mappingKnot(2, 3), mappingKnot(3, 4)}, []bidi.Segment{{From: 0, To: 1, Dir: ltr}, {From: 1, To: 3, Dir: rtl}, {From: 3, To: 4, Dir: ltr}}, []int{0, 2, 1, 3}, nil},
		{"visual segment order", []logicalKnot{mappingKnot(0, 1), mappingKnot(1, 2), mappingKnot(2, 3), mappingKnot(3, 4)}, []bidi.Segment{{From: 2, To: 4, Dir: rtl}, {From: 0, To: 2, Dir: ltr}}, []int{3, 2, 0, 1}, nil},
		{"source gap", []logicalKnot{mappingKnot(0, 1), mappingKnot(3, 4)}, []bidi.Segment{{From: 3, To: 4, Dir: rtl}, {From: 0, To: 1, Dir: ltr}}, []int{1, 0}, nil},
		{"trimmed segment", []logicalKnot{mappingKnot(1, 2)}, []bidi.Segment{{From: 0, To: 1, Dir: ltr}, {From: 1, To: 2, Dir: ltr}, {From: 2, To: 4, Dir: rtl}}, []int{0}, nil},
		{"all trimmed", nil, []bidi.Segment{{From: 0, To: 4, Dir: rtl}}, nil, nil},
		{"no segments", nil, nil, nil, nil},
		{"generated", []logicalKnot{mappingKnot(0, 0), mappingKnot(0, 4), mappingKnot(4, 4)}, []bidi.Segment{{From: 0, To: 4, Dir: ltr}}, []int{1}, []int{0, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := logicalLine{knots: tc.knots, byteEnd: 4}
			before, segments := slices.Clone(line.knots), slices.Clone(tc.segments)
			got, err := line.mapVisualSegments(tc.segments)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got.visual, tc.want) || !slices.Equal(got.unresolved, tc.unresolved) {
				t.Fatalf("got %+v, want visual %v, unresolved %v", got, tc.want, tc.unresolved)
			}
			seen := make([]bool, len(line.knots))
			for _, list := range [][]int{got.visual, got.unresolved} {
				for _, i := range list {
					if seen[i] {
						t.Fatal("duplicate knot", i)
					}
					seen[i] = true
				}
			}
			if slices.Contains(seen, false) {
				t.Fatal("lost knot")
			}
			again, err := line.mapVisualSegments(tc.segments)
			if err != nil || !reflect.DeepEqual(got, again) || !reflect.DeepEqual(before, line.knots) || !reflect.DeepEqual(segments, tc.segments) {
				t.Fatal("mutation or unstable mapping", err)
			}
		})
	}
}

func TestKnotOrderingRejectsAmbiguity(t *testing.T) {
	ltr, rtl := bidi.LeftToRight, bidi.RightToLeft
	for _, tc := range []struct {
		name     string
		knots    []logicalKnot
		segments []bidi.Segment
	}{
		{"missing segment", []logicalKnot{mappingKnot(0, 2)}, nil},
		{"partially covered", []logicalKnot{mappingKnot(0, 2)}, []bidi.Segment{{From: 1, To: 2, Dir: ltr}}},
		{"direction boundary inside box", []logicalKnot{mappingKnot(0, 4)}, []bidi.Segment{{From: 0, To: 2, Dir: ltr}, {From: 2, To: 4, Dir: rtl}}},
		{"same direction boundary inside box", []logicalKnot{mappingKnot(0, 4)}, []bidi.Segment{{From: 0, To: 2, Dir: ltr}, {From: 2, To: 4, Dir: ltr}}},
		{"internal gap", []logicalKnot{mappingKnot(0, 4)}, []bidi.Segment{{From: 0, To: 1, Dir: ltr}, {From: 3, To: 4, Dir: ltr}}},
		{"duplicate segments", nil, []bidi.Segment{{From: 0, To: 2, Dir: ltr}, {From: 0, To: 2, Dir: ltr}}},
		{"overlapping segments", nil, []bidi.Segment{{From: 0, To: 3, Dir: ltr}, {From: 2, To: 4, Dir: ltr}}},
		{"empty segment", nil, []bidi.Segment{{From: 2, To: 2, Dir: ltr}}},
		{"reversed segment", nil, []bidi.Segment{{From: 3, To: 2, Dir: ltr}}},
		{"outside line", nil, []bidi.Segment{{From: 0, To: 5, Dir: ltr}}},
		{"invalid direction", nil, []bidi.Segment{{From: 0, To: 4, Dir: bidi.Direction(99)}}},
		{"reversed knot", []logicalKnot{mappingKnot(2, 1)}, nil},
		{"overlapping knots", []logicalKnot{mappingKnot(0, 3), mappingKnot(2, 4)}, []bidi.Segment{{From: 0, To: 4, Dir: ltr}}},
		{"unordered knots", []logicalKnot{mappingKnot(2, 4), mappingKnot(0, 2)}, []bidi.Segment{{From: 0, To: 4, Dir: ltr}}},
		{"knot outside line", []logicalKnot{mappingKnot(0, 5)}, []bidi.Segment{{From: 0, To: 4, Dir: ltr}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := logicalLine{knots: tc.knots, byteEnd: 4}
			before := slices.Clone(line.knots)
			if got, err := line.mapVisualSegments(tc.segments); got != nil || !errors.Is(err, ErrIllegalArguments) {
				t.Fatal(got, err)
			}
			if !reflect.DeepEqual(before, line.knots) {
				t.Fatal("failure changed input")
			}
		})
	}
	if got, err := (logicalLine{}).mapOrdering(nil); got != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal(got, err)
	}
}

// Real Orderings verify that the adapter traverses runs and their noncontiguous
// intervals in visual order, rather than merely sorting all knots by direction.
func TestKnotOrderingBidiTraversal(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"abc", []string{"a", "b", "c"}},
		{"אבג", []string{"ג", "ב", "א"}},
		{"aאבb", []string{"a", "ב", "א", "b"}},
		{"a\u2067אב\u2069b", []string{"a", "ב", "א", "b"}},
		{"a\u2067א\u2066xy\u2069ב\u2069z", []string{"a", "ב", "x", "y", "א", "z"}},
	} {
		t.Run(tc.text, func(t *testing.T) {
			line := logicalLine{byteEnd: uint64(len(tc.text))}
			for from, r := range tc.text {
				if r == '\u2067' || r == '\u2066' || r == '\u2069' {
					continue
				}
				line.knots = append(line.knots, mappingKnot(uint64(from), uint64(from+len(string(r)))))
			}
			order := bidi.ResolveParagraph(strings.NewReader(tc.text), nil).Reorder()
			got, err := line.mapOrdering(order)
			if err != nil {
				t.Fatal(err)
			}
			var words []string
			for _, i := range got.visual {
				s := line.knots[i].source
				words = append(words, tc.text[s.byteStart:s.byteEnd])
			}
			if !slices.Equal(words, tc.want) || len(got.unresolved) != 0 {
				t.Fatal(words, got)
			}
		})
	}
}

func TestKnotOrderingMaterializedLine(t *testing.T) {
	para := preparationParagraph(t, "Go אבגד  end")
	if _, err := para.Style(intersectionStyle("split RTL box"), 7, 11); err != nil {
		t.Fatal(err)
	}
	k := eligibilityParagraph(t, para)
	p := l1Materialized(t, k, 13)
	orders, err := p.lineOrderings()
	if err != nil {
		t.Fatal(err)
	}
	before := slices.Clone(p.lines[0].knots)
	glyphs := slices.Clone(p.arena.glyphs)
	got, err := p.lines[0].mapOrdering(orders[0])
	if err != nil {
		t.Fatal(err)
	}
	var starts []uint64
	for _, i := range got.visual {
		starts = append(starts, p.lines[0].knots[i].source.byteStart)
	}
	if !slices.Equal(starts, []uint64{0, 2, 7, 3}) || len(got.unresolved) != 0 {
		t.Fatal(starts, got)
	}
	if !reflect.DeepEqual(before, p.lines[0].knots) || !reflect.DeepEqual(glyphs, p.arena.glyphs) {
		t.Fatal("changed knots or glyph storage")
	}
	last, err := p.lines[1].mapOrdering(orders[1])
	if err != nil || len(last.unresolved) != 1 || last.unresolved[0] != len(p.lines[1].knots)-1 {
		t.Fatal("final fill must await attachment", last, err)
	}
}

func TestKnotOrderingPlacesDiscretionaries(t *testing.T) {
	owner, k, _ := logicalTestDiscretionary(t)
	p, err := owner.materializeLogicalLines(k, []int{0, len(k.Kind) - 1})
	if err != nil {
		t.Fatal(err)
	}
	orders, err := p.lineOrderings()
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range p.lines {
		got, err := line.mapOrdering(orders[i])
		if err != nil || !slices.Equal(got.visual, []int{0}) || len(got.unresolved) != i {
			t.Fatal("replacement must be placed; only final fill remains unresolved", got, err)
		}
	}
}

func TestKnotOrderingLocalCoordinatesAndNeutralKnots(t *testing.T) {
	line := logicalLine{byteStart: 10, byteEnd: 14, knots: []logicalKnot{
		mappingKnot(10, 11), mappingKnot(11, 12), mappingKnot(12, 13), mappingKnot(13, 14),
	}}
	line.knots[1].core.Kind = KTGlue
	line.knots[2].core.Kind = KTKern
	for i := range line.knots {
		line.knots[i].core.Pos = 100 + line.knots[i].source.byteStart
		line.knots[i].origin = 42 + i
	}
	segments := []bidi.Segment{{From: 10, To: 14, Dir: bidi.RightToLeft}}
	got, err := line.mapVisualSegments(segments)
	if err != nil || !slices.Equal(got.visual, []int{3, 2, 1, 0}) {
		t.Fatal("must use local indices and paragraph-relative source spans", got, err)
	}
	line.knots[0].source.byteStart = 9
	if got, err := line.mapVisualSegments(segments); got != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal("accepted source before line", got, err)
	}
	line.knots[0].source.byteStart = 10
	line.knots[0].fragment = logicalFragment(99)
	if got, err := line.mapVisualSegments(segments); got != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal("accepted unknown role", got, err)
	}
	if got, err := (logicalLine{byteStart: 2, byteEnd: 1}).mapVisualSegments(nil); got != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal("accepted reversed line", got, err)
	}
}
