package khipu

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/uax/bidi"
)

func TestTrailingGlueL1Policy(t *testing.T) {
	item := func(kind KnotType, from, to uint64) logicalKnot {
		return logicalKnot{core: KnotCore{Kind: kind}, source: runSourceRange{byteStart: from, byteEnd: to}}
	}
	box := item(KTTextBox, 0, 3)
	glue := item(KTGlue, 3, 5)
	trailer := item(KTGlue, 5, 5)
	trailer.origin = 99
	replacement := item(KTTextBox, 5, 5)
	replacement.fragment = logicalPreBreak // silent text is still a barrier
	for _, tc := range []struct {
		name       string
		start, end uint64
		items      []logicalKnot
		want       sourceByteInterval
	}{
		{"plain", 0, 5, []logicalKnot{box, glue}, sourceByteInterval{3, 5}},
		{"final fill", 0, 5, []logicalKnot{box, glue, trailer}, sourceByteInterval{3, 5}},
		{"only fill", 5, 5, []logicalKnot{trailer}, sourceByteInterval{}},
		{"no glue", 0, 3, []logicalKnot{box}, sourceByteInterval{}},
		{"no knots", 0, 5, nil, sourceByteInterval{}},
		{"adjacent glues", 0, 5, []logicalKnot{box, item(KTGlue, 3, 4), item(KTGlue, 4, 5)}, sourceByteInterval{3, 5}},
		{"clip both endpoints", 2, 4, []logicalKnot{item(KTGlue, 0, 6)}, sourceByteInterval{2, 4}},
		{"trailing gap", 0, 6, []logicalKnot{box, glue}, sourceByteInterval{}},
		{"intervening gap", 0, 7, []logicalKnot{box, glue, item(KTGlue, 6, 7)}, sourceByteInterval{6, 7}},
		{"zero source glue", 0, 5, []logicalKnot{box, glue, item(KTGlue, 5, 5)}, sourceByteInterval{}},
		{"kern barrier", 0, 5, []logicalKnot{box, glue, item(KTKern, 5, 5)}, sourceByteInterval{}},
		{"silent replacement barrier", 0, 5, []logicalKnot{box, glue, replacement}, sourceByteInterval{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := logicalLine{knots: tc.items, byteStart: tc.start, byteEnd: tc.end}
			if got := line.trailingGlueL1(99); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			// Flags and elasticity do not decide eligibility for this heuristic.
			for i := range line.knots {
				line.knots[i].core.Flags = KFDiscardable
			}
			if got := line.trailingGlueL1(99); got != tc.want {
				t.Fatal("flags changed L1", got)
			}
		})
	}
}

func l1Materialized(t *testing.T, k *Khipu, ends ...uint64) *logicalParagraph {
	t.Helper()
	var breaks []int
	for _, end := range ends {
		at := -1
		for i, s := range k.shaping.sources {
			if s.breakAfter.byteEnd == end && k.Penalty[i] < MaxPenalty {
				at = i
				break
			}
		}
		if at < 0 {
			t.Fatalf("no breakpoint at %d", end)
		}
		breaks = append(breaks, at)
	}
	breaks = append(breaks, len(k.Kind)-1)
	p, err := k.owner.materializeLogicalLines(k, breaks)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func orderingSegments(order *bidi.Ordering) []bidi.Segment {
	var result []bidi.Segment
	for _, run := range order.Runs {
		for s := range run.Segments(run.IsOpposite(bidi.LeftToRight)) {
			result = append(result, s)
		}
	}
	return result
}

func TestLineOrderingsApplyTrailingGlueL1(t *testing.T) {
	for _, tc := range []struct {
		text      string
		dir       bidi.Direction
		from, end uint64
	}{
		{"Go אבג  דה", bidi.LeftToRight, 9, 11},
		{"אב abc  def", bidi.RightToLeft, 8, 10},
	} {
		t.Run(tc.dir.String(), func(t *testing.T) {
			text := styled.TextFromString(tc.text)
			para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), tc.dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			para.Offset = 100 // L1 coordinates must not include this offset.
			k, err := NewKhipukamayuq(Config{}).EncodeParagraph(t.Context(), para)
			if err != nil {
				t.Fatal(err)
			}
			p := l1Materialized(t, k, tc.end)
			if p.lines[0].l1Glue != (sourceByteInterval{tc.from, tc.end}) || p.lines[1].l1Glue != (sourceByteInterval{}) {
				t.Fatal(p.lines)
			}
			for _, item := range p.lines[0].knots {
				if item.source.byteStart >= tc.from {
					t.Fatal("trailing glue was not trimmed")
				}
			}
			before := para.BidiLevels().Clone()
			cores := slices.Clone(p.lines[0].knots)
			orders, err := p.lineOrderings()
			if err != nil || len(orders) != 2 {
				t.Fatal(orders, err)
			}
			found := false
			for _, s := range orderingSegments(orders[0]) {
				if s.From <= tc.from && s.To >= tc.end && s.Dir == tc.dir {
					found = true
				}
			}
			if !found {
				t.Fatal("trailing glue did not reset to paragraph direction", orderingSegments(orders[0]))
			}
			if !reflect.DeepEqual(before, para.BidiLevels()) || !reflect.DeepEqual(cores, p.lines[0].knots) {
				t.Fatal("mutated input")
			}
			again, err := p.lineOrderings()
			if err != nil || !reflect.DeepEqual(orders, again) {
				t.Fatal("ordering not repeatable", err)
			}
			// Show this is a real reset, not merely a check of unchanged output.
			unreset, _ := para.BidiLevels().Clone().Split(tc.end, false)
			for s := range unreset.LogicalSegments() {
				if s.From <= tc.from && s.To >= tc.end && s.Dir == tc.dir {
					t.Fatal("fixture's spaces already had paragraph direction")
				}
			}
		})
	}
}

func TestLineOrderingsFinalAndHeuristicLimits(t *testing.T) {
	for _, tc := range []struct {
		text string
		want sourceByteInterval
	}{
		{"", sourceByteInterval{}},
		{"   ", sourceByteInterval{0, 3}},
		{"abc", sourceByteInterval{}},
		{"abc  ", sourceByteInterval{3, 5}},
		{"a\u2002", sourceByteInterval{1, 4}},      // multibyte glue
		{"a\u00a0", sourceByteInterval{1, 3}},      // accepted over-reset: glue, not bidi WS
		{"a\tb", sourceByteInterval{}},             // internal separator is deliberately missed
		{"a \u00ad", sourceByteInterval{1, 4}},     // suppressed marker included in glue provenance
		{"a\u2067אב \u2069", sourceByteInterval{}}, // trailing control gap is not guessed
	} {
		t.Run(tc.text, func(t *testing.T) {
			_, k := logicalTestParagraph(t, tc.text)
			p := l1Materialized(t, k)
			if p.lines[0].l1Glue != tc.want {
				t.Fatalf("got %+v, want %+v", p.lines[0].l1Glue, tc.want)
			}
			if orders, err := p.lineOrderings(); err != nil || len(orders) != 1 {
				t.Fatal(orders, err)
			}
		})
	}
}

func TestLineOrderingsSplitStyleGlueAndClipMarker(t *testing.T) {
	para := preparationParagraph(t, "Go אבג  דה")
	if _, err := para.Style(intersectionStyle("space"), 10, 11); err != nil {
		t.Fatal(err)
	}
	k := eligibilityParagraph(t, para)
	p := l1Materialized(t, k, 11)
	if p.lines[0].l1Glue != (sourceByteInterval{9, 11}) {
		t.Fatal(p.lines[0].l1Glue)
	}
	count := 0
	for i, s := range k.shaping.sources {
		if k.Kind[i] == KTGlue && s.source.byteStart >= 9 && s.source.byteEnd <= 11 {
			count++
		}
	}
	if count != 2 {
		t.Fatal("fixture did not split glue at style boundary", count)
	}
	if _, err := p.lineOrderings(); err != nil {
		t.Fatal(err)
	}
	_, k = logicalTestParagraph(t, "a \u00adb")
	p = l1Materialized(t, k, 2)
	if p.lines[0].l1Glue != (sourceByteInterval{1, 2}) {
		t.Fatal("L1 crossed line into marker", p.lines[0].l1Glue)
	}
	if _, err := p.lineOrderings(); err != nil {
		t.Fatal(err)
	}
}

func TestLineOrderingsDiscretionaryAndEmptyLines(t *testing.T) {
	owner, k, _ := logicalTestDiscretionary(t)
	p, err := owner.materializeLogicalLines(k, []int{0, len(k.Kind) - 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range p.lines {
		if line.l1Glue != (sourceByteInterval{}) {
			t.Fatal("reset replacement source")
		}
	}
	if _, err := p.lineOrderings(); err != nil {
		t.Fatal(err)
	}
	_, k = logicalTestParagraph(t, "  aa ")
	p = l1Materialized(t, k, 2)
	if len(p.lines[0].knots) != 0 || p.lines[0].l1Glue != (sourceByteInterval{0, 2}) {
		t.Fatal("lost empty line's L1")
	}
	if orders, err := p.lineOrderings(); err != nil || len(orders) != 2 {
		t.Fatal(orders, err)
	}
}

func TestLineOrderingsSuccessiveSplitsInsideIsolate(t *testing.T) {
	_, k := logicalTestParagraph(t, "A \u2067אב  גד  הו\u2069 Z ")
	p := l1Materialized(t, k, 11, 17)
	want := []sourceByteInterval{{9, 11}, {15, 17}, {26, 27}}
	before := k.paragraph.BidiLevels().Clone()
	orders, err := p.lineOrderings()
	if err != nil || len(orders) != len(want) {
		t.Fatal(orders, err)
	}
	for i, interval := range want {
		if p.lines[i].l1Glue != interval {
			t.Fatalf("line %d: got %+v, want %+v", i+1, p.lines[i].l1Glue, interval)
		}
		found := false
		for _, segment := range orderingSegments(orders[i]) {
			if segment.From <= interval.from && segment.To >= interval.to && segment.Dir == bidi.LeftToRight {
				found = true
			}
		}
		if !found {
			t.Fatalf("line %d: glue not reset to paragraph direction: %+v", i+1, orderingSegments(orders[i]))
		}
	}
	again, err := p.lineOrderings()
	if err != nil || !reflect.DeepEqual(orders, again) || !reflect.DeepEqual(before, k.paragraph.BidiLevels()) {
		t.Fatal("successive isolate splits consumed input", err)
	}
}

func TestLineOrderingsRejectInvalidInput(t *testing.T) {
	for _, mode := range []string{"nil", "no paragraph", "no lines", "source gap", "reversed line", "beyond end", "partial", "L1 before line", "L1 beyond line", "L1 reversed", "L1 not suffix"} {
		t.Run(mode, func(t *testing.T) {
			_, k := logicalTestParagraph(t, "aa bb ")
			p := l1Materialized(t, k, 3)
			before := k.paragraph.BidiLevels().Clone()
			switch mode {
			case "nil":
				p = nil
			case "no paragraph":
				p.paragraph = nil
			case "no lines":
				p.lines = nil
			case "source gap":
				p.lines[1].byteStart++
			case "reversed line":
				p.lines[1].byteEnd = 2
			case "beyond end":
				p.lines[1].byteEnd = 100
			case "partial":
				p.lines = p.lines[:1]
			case "L1 before line":
				p.lines[1].l1Glue = sourceByteInterval{0, 6}
			case "L1 beyond line":
				p.lines[1].l1Glue = sourceByteInterval{5, 7}
			case "L1 reversed":
				p.lines[1].l1Glue = sourceByteInterval{6, 5}
			case "L1 not suffix":
				p.lines[1].l1Glue = sourceByteInterval{3, 4}
			}
			if got, err := p.lineOrderings(); got != nil || !errors.Is(err, ErrIllegalArguments) {
				t.Fatal(got, err)
			}
			if !reflect.DeepEqual(before, k.paragraph.BidiLevels()) {
				t.Fatal("failed ordering consumed original")
			}
		})
	}
}
