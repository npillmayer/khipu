package khipu

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

func logicalTestParagraph(t *testing.T, text string) (*Khipukamayuq, *Khipu) {
	t.Helper()
	owner := NewKhipukamayuq(Config{})
	p := preparationParagraph(t, text)
	p.Offset = 100
	k, err := owner.EncodeParagraph(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	return owner, k
}

func TestLogicalLinesOrdinary(t *testing.T) {
	for _, tc := range []struct {
		text   string
		ends   []uint64
		widths []dimen.DU
	}{
		{"", []uint64{0}, []dimen.DU{0}},
		{"   ", []uint64{3}, []dimen.DU{0}},
		{"  aa  bb  ", []uint64{6, 10}, []dimen.DU{10 * dimen.PT, 10 * dimen.PT}},
		{"aa\nbb", []uint64{3, 5}, []dimen.DU{10 * dimen.PT, 10 * dimen.PT}},
		{"אב cd", []uint64{5, 7}, []dimen.DU{10 * dimen.PT, 10 * dimen.PT}},
	} {
		t.Run(tc.text, func(t *testing.T) {
			owner, k := logicalTestParagraph(t, tc.text)
			var breaks []int
			for j, end := range tc.ends {
				at := len(k.Kind) - 1
				if j < len(tc.ends)-1 {
					at = -1
					for i, source := range k.shaping.sources {
						if source.breakAfter.byteEnd == end && k.Penalty[i] < MaxPenalty {
							at = i
							break
						}
					}
				}
				breaks = append(breaks, at)
			}
			cores := make([]KnotCore, len(k.Kind))
			for i := range cores {
				cores[i] = k.KnotByIndex(i)
			}
			got, err := owner.materializeLogicalLines(context.Background(), k, breaks)
			if err != nil {
				t.Fatal(err)
			}
			var start uint64
			for i, line := range got.lines {
				if line.byteStart != start || line.byteEnd != tc.ends[i] || line.naturalWidth != tc.widths[i] {
					t.Fatalf("line %d: %+v", i, line)
				}
				start = line.byteEnd
				for _, item := range line.knots {
					if item.fragment != logicalOriginal || item.core != cores[item.origin] || item.core.IsDiscardable() {
						t.Fatalf("unexpected knot: %+v", item)
					}
					if item.core.Kind == KTTextBox && got.arena.boxes[item.box-1].source != item.source {
						t.Fatal("lost glyph association")
					}
				}
			}
			last := got.lines[len(got.lines)-1].knots
			if last[len(last)-1].origin != len(k.Kind)-1 {
				t.Fatal("lost terminal fill")
			}
			for i, core := range cores {
				if k.KnotByIndex(i) != core {
					t.Fatal("mutated elastic input")
				}
			}
		})
	}
}

func TestLogicalLineTrimmingNeutralKnots(t *testing.T) {
	content := KnotCore{Kind: KTTextBox, W: 10}
	discard := KnotCore{Kind: KTKern, Flags: KFDiscardable, W: 2}
	neutral := KnotCore{Kind: KTGlue, W: 3}
	for _, tc := range []struct {
		input []KnotCore
		want  []int
		width dimen.DU
	}{
		{[]KnotCore{discard, neutral, discard, content, discard, neutral, discard}, []int{1, 3, 5}, 16},
		{[]KnotCore{discard, neutral, discard}, []int{1}, 3},
		{[]KnotCore{content, discard, neutral, discard, content}, []int{0, 1, 2, 3, 4}, 27},
		{[]KnotCore{discard, discard}, nil, 0},
		{[]KnotCore{{Kind: KTTextBox, Flags: KFDiscardable, W: 10}, neutral}, []int{1}, 3},
	} {
		line := logicalLine{}
		for i, c := range tc.input {
			line.knots = append(line.knots, logicalKnot{core: c, origin: i})
		}
		if err := line.trimAndMeasure(); err != nil {
			t.Fatal(err)
		}
		var origins []int
		for _, item := range line.knots {
			origins = append(origins, item.origin)
		}
		if !slices.Equal(origins, tc.want) || line.naturalWidth != tc.width {
			t.Fatal(origins, line.naturalWidth)
		}
	}
}

func TestLogicalLinesSilentReplacements(t *testing.T) {
	k, pair := discretionaryStorageFixture(t)
	// Empty and nonempty-but-silent replacement input both retain their
	// original provenance and distinct arena references.
	pair.pre.input.text, pair.pre.input.scalarCount = "", 0
	pair.pre.glyphs, pair.pre.widths = nil, candidateWidths{}
	pair.post.glyphs, pair.post.widths = nil, candidateWidths{}
	c, err := k.publishDiscretionary(0, 1, 0, pair)
	if err != nil {
		t.Fatal(err)
	}
	k.SelectDiscretionary(0, DiscretionarySelection{0, 1})
	p, err := k.owner.materializeLogicalLines(context.Background(), k, []int{0, len(k.Kind) - 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.lines[0].knots) != 1 || p.lines[0].knots[0].box != c.preBox ||
		p.lines[1].knots[0].box != c.postBox || p.lines[0].naturalWidth != 0 || p.lines[1].naturalWidth != 0 ||
		p.lines[0].byteEnd != 3 || p.lines[1].byteStart != 3 || p.lines[1].byteEnd != 4 {
		t.Fatal("lost silent replacement or original coverage", p.lines)
	}
}

func TestLogicalLinesConsecutiveReplacements(t *testing.T) {
	owner := NewKhipukamayuq(Config{Hyphenator: &testhyphenator{want: []HyphenationFragment{{PreBreak: "-"}}}})
	k, err := owner.EncodeParagraph(t.Context(), preparationParagraph(t, "ab\u00adcd ef\u00adgh"))
	if err != nil {
		t.Fatal(err)
	}
	var breaks []int
	var chosen []DiscretionaryCandidate
	for i := range k.Kind {
		if k.HasExplicitDiscretionary(i) {
			c, err := owner.DiscretionaryCandidates(k, i)
			if err != nil || len(c) != 1 {
				t.Fatal(c, err)
			}
			k.SelectDiscretionary(i, DiscretionarySelection{i, c[0].Variant})
			breaks = append(breaks, i)
			chosen = append(chosen, c[0])
		}
	}
	breaks = append(breaks, len(k.Kind)-1)
	p, err := owner.materializeLogicalLines(context.Background(), k, breaks)
	if err != nil {
		t.Fatal(err)
	}
	if len(chosen) != 2 || len(p.lines) != 3 || len(p.lines[1].knots) != 3 ||
		p.lines[1].knots[0].box != chosen[0].postBox || p.lines[1].knots[1].core.Kind != KTGlue ||
		p.lines[1].knots[2].box != chosen[1].preBox || p.lines[2].knots[0].box != chosen[1].postBox ||
		p.lines[1].naturalWidth != 30*dimen.PT {
		t.Fatal("incorrect consecutive carry", p.lines)
	}
}

func TestLogicalLinesGlyphlessSourceGap(t *testing.T) {
	a, g := candidateFixture(t, "a\u2067אב\u2069z")
	a.boundaries[3].penalty = 0
	knots := accumulateTestGlyphs(t, a, g, []contracts.ShapedGlyph{{Cluster: 0, XAdvance: 500}, {Cluster: 5, XAdvance: 500}})
	owner := NewKhipukamayuq(Config{})
	k, err := owner.publishCandidateParagraph(&candidateParagraph{a.para, a.runs, a.boundaries, knots})
	if err != nil {
		t.Fatal(err)
	}
	p, err := owner.materializeLogicalLines(context.Background(), k, []int{0, len(k.Kind) - 1})
	if err != nil {
		t.Fatal(err)
	}
	if p.lines[0].byteEnd != 8 || p.lines[0].knots[0].source.byteEnd != 1 || p.lines[1].byteStart != 8 || p.lines[1].byteEnd != 12 {
		t.Fatal("replaced line coverage with visible source span", p.lines)
	}
}

func logicalTestDiscretionary(t *testing.T) (*Khipukamayuq, *Khipu, DiscretionaryCandidate) {
	t.Helper()
	owner := NewKhipukamayuq(Config{Hyphenator: &testhyphenator{want: []HyphenationFragment{
		{Variant: 4, PreBreak: "LONG-", PostBreak: "lair", SourceSplit: SourceSplit{3, true}},
	}}})
	p := preparationParagraph(t, "éclair")
	p.Offset = 100
	k, err := owner.EncodeParagraph(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := owner.DiscretionaryCandidates(k, 0)
	if err != nil || len(candidates) != 1 {
		t.Fatal(candidates, err)
	}
	k.SelectDiscretionary(0, DiscretionarySelection{Source: 0, Variant: 4})
	return owner, k, candidates[0]
}

func TestLogicalLinesReplacementAndLifetime(t *testing.T) {
	owner, k, candidate := logicalTestDiscretionary(t)
	owner.env.streaming.NewShaper = func() (contracts.StreamingShaper, error) { t.Fatal("materialization reshaped input"); return nil, nil }
	selections := maps.Clone(k.SelectedDiscretionaries)
	glyphs := slices.Clone(k.shaping.glyphs)
	breaks := []int{0, len(k.Kind) - 1}
	got, err := owner.materializeLogicalLines(context.Background(), k, breaks)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selections, k.SelectedDiscretionaries) || !reflect.DeepEqual(glyphs, k.shaping.glyphs) {
		t.Fatal("materialization mutated input")
	}
	if len(got.lines) != 2 || len(got.lines[0].knots) != 1 || len(got.lines[1].knots) != 2 {
		t.Fatal("wrong replacement/carry structure")
	}
	a, b := got.lines[0].knots[0], got.lines[1].knots[0]
	if a.box != candidate.preBox || b.box != candidate.postBox || a.core != candidate.PreBreak || b.core != candidate.PostBreak ||
		a.fragment != logicalPreBreak || b.fragment != logicalPostBreak || a.variant != 4 || b.variant != 4 {
		t.Fatal(a, b)
	}
	if got.lines[0].byteEnd != 3 || got.lines[1].byteStart != 3 || got.lines[1].byteEnd != 7 ||
		got.lines[0].naturalWidth != 25*dimen.PT || got.lines[1].naturalWidth != 20*dimen.PT {
		t.Fatal("used replacement spelling as source or width")
	}
	// A result retains immutable arena storage, not a dependency on mutable
	// selections, slice headers or the owner's current paragraph attachment.
	breaks[0] = 99
	clear(k.SelectedDiscretionaries)
	k.DiscretionaryCandidates[0][0].PreBreak.W = 99
	k.shaping.glyphs = slices.Grow(k.shaping.glyphs, len(k.shaping.glyphs)+100)
	unwireKhipu(k)
	if got.lines[0].breakpoint != 0 || got.lines[0].knots[0].core != candidate.PreBreak ||
		!reflect.DeepEqual(got.arena.glyphs, glyphs) || got.arena.boxes[a.box-1].replacement.text != "LONG-" ||
		got.paragraph.Raw().String() != "éclair" {
		t.Fatal("snapshot lost its storage")
	}
}

func TestLogicalLinesRejectInvalidInput(t *testing.T) {
	for _, mode := range []string{"empty path", "partial", "duplicate", "outside", "prohibited", "missing plane", "missing source", "missing glyphs", "wrong owner", "discardable trailer", "bad trailer", "forced skipped", "stale selection", "overflow"} {
		t.Run(mode, func(t *testing.T) {
			owner, k := logicalTestParagraph(t, "aa bb")
			breaks := []int{len(k.Kind) - 1}
			switch mode {
			case "empty path":
				breaks = nil
			case "partial":
				breaks = []int{1}
			case "duplicate":
				breaks = []int{1, 1, len(k.Kind) - 1}
			case "outside":
				breaks = []int{-1, len(k.Kind) - 1}
			case "prohibited":
				breaks = []int{0, len(k.Kind) - 1}
			case "missing plane":
				k.W = nil
			case "missing source":
				k.shaping.sources = nil
			case "missing glyphs":
				k.boxRefs[0] = 0
			case "wrong owner":
				owner = NewKhipukamayuq(Config{})
			case "discardable trailer":
				k.Flags[len(k.Kind)-1] = KFDiscardable
			case "bad trailer":
				k.Penalty[len(k.Kind)-1] = 0
			case "forced skipped":
				k.Penalty[1] = MaxMerit
			case "stale selection":
				k.SelectDiscretionary(1, DiscretionarySelection{Source: 0, Variant: 1})
			case "overflow":
				for _, i := range []int{0, 2} {
					k.W[i] = dimen.Fil/2 + 1
					k.MaxW[i] = k.W[i]
				}
			}
			if got, err := owner.materializeLogicalLines(context.Background(), k, breaks); got != nil || !errors.Is(err, ErrIllegalArguments) {
				t.Fatal(got, err)
			}
		})
	}
	if got, err := (*Khipukamayuq)(nil).materializeLogicalLines(context.Background(), nil, nil); got != nil || !errors.Is(err, ErrVoidKhipukamayuq) {
		t.Fatal(got, err)
	}
	if got, err := NewKhipukamayuq(Config{}).materializeLogicalLines(context.Background(), nil, nil); got != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal(got, err)
	}
}

func TestLogicalLinesRejectBrokenSelection(t *testing.T) {
	for _, mode := range []string{"unknown variant", "duplicate variant", "unknown split", "missing pre", "missing post", "wrong provenance", "wrong context", "wrong glyph range", "missing choice", "off-path choice", "moved source"} {
		t.Run(mode, func(t *testing.T) {
			owner, k, c := logicalTestDiscretionary(t)
			breaks := []int{0, len(k.Kind) - 1}
			switch mode {
			case "unknown variant":
				k.SelectedDiscretionaries[0] = DiscretionarySelection{0, 99}
			case "duplicate variant":
				k.AddDiscretionaryCandidate(0, c)
			case "unknown split":
				k.DiscretionaryCandidates[0][0].SourceSplit.Valid = false
			case "missing pre":
				k.DiscretionaryCandidates[0][0].preBox = 0
			case "missing post":
				k.DiscretionaryCandidates[0][0].postBox = 0
			case "wrong provenance":
				k.shaping.boxes[c.postBox-1].source.byteStart++
			case "wrong context":
				k.shaping.boxes[c.postBox-1].context = -1
			case "wrong glyph range":
				k.shaping.boxes[c.postBox-1].glyphs.end = len(k.shaping.glyphs) + 1
			case "missing choice":
				clear(k.SelectedDiscretionaries)
			case "off-path choice":
				breaks = []int{len(k.Kind) - 1}
			case "moved source":
				k.SelectedDiscretionaries[0] = DiscretionarySelection{1, 4}
			}
			if got, err := owner.materializeLogicalLines(context.Background(), k, breaks); got != nil || !errors.Is(err, ErrIllegalArguments) {
				t.Fatal(got, err)
			}
		})
	}
}
