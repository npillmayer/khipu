package khipu

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/uax/bidi"
)

func mappingFragment(from, to uint64, role logicalFragment) logicalKnot {
	k := mappingKnot(from, to)
	k.fragment, k.variant, k.box = role, 7, 11
	return k
}

func TestKnotOrderingFragmentDirections(t *testing.T) {
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		line := logicalLine{byteEnd: 6, knots: []logicalKnot{
			mappingFragment(0, 2, logicalPostBreak), mappingKnot(2, 4), mappingFragment(4, 6, logicalPreBreak),
		}}
		before := slices.Clone(line.knots)
		got, err := line.mapVisualSegments([]bidi.Segment{{From: 0, To: 6, Dir: dir}})
		want := []int{0, 1, 2}
		if dir == bidi.RightToLeft {
			want = []int{2, 1, 0}
		}
		if err != nil || !slices.Equal(got.visual, want) || len(got.unresolved) != 0 || !reflect.DeepEqual(before, line.knots) {
			t.Fatal(dir, got, err)
		}
	}
	// A pre-break fragment's logical line-end position is not necessarily its
	// physical right edge. Its source segment, not the fragment role, decides.
	line := logicalLine{byteEnd: 6, knots: []logicalKnot{
		mappingKnot(0, 2), mappingKnot(2, 4), mappingFragment(4, 6, logicalPreBreak),
	}}
	got, err := line.mapVisualSegments([]bidi.Segment{
		{From: 0, To: 2, Dir: bidi.LeftToRight}, {From: 2, To: 6, Dir: bidi.RightToLeft},
	})
	if err != nil || !slices.Equal(got.visual, []int{0, 2, 1}) {
		t.Fatal(got, err)
	}
}

func TestKnotOrderingFragmentFailures(t *testing.T) {
	for _, mode := range []string{"crosses directions", "same-direction boundary", "gap", "unmapped", "zero pre", "zero post", "pre not at end", "post not at start", "not textbox"} {
		t.Run(mode, func(t *testing.T) {
			line := logicalLine{byteEnd: 4, knots: []logicalKnot{mappingFragment(0, 4, logicalPreBreak)}}
			segments := []bidi.Segment{{From: 0, To: 4, Dir: bidi.LeftToRight}}
			switch mode {
			case "crosses directions", "same-direction boundary":
				segments[0].To = 2
				dir := bidi.RightToLeft
				if mode == "same-direction boundary" {
					dir = bidi.LeftToRight
				}
				segments = append(segments, bidi.Segment{From: 2, To: 4, Dir: dir})
			case "gap":
				segments[0].To = 1
				segments = append(segments, bidi.Segment{From: 3, To: 4, Dir: bidi.LeftToRight})
			case "unmapped":
				segments = nil
			case "zero pre":
				line.knots = []logicalKnot{mappingKnot(0, 4), mappingFragment(4, 4, logicalPreBreak)}
			case "zero post":
				line.knots = []logicalKnot{mappingFragment(0, 0, logicalPostBreak), mappingKnot(0, 4)}
			case "pre not at end":
				line.knots[0].source.byteEnd = 3
			case "post not at start":
				line.knots[0] = mappingFragment(1, 4, logicalPostBreak)
			case "not textbox":
				line.knots[0].core.Kind = KTGlue
			}
			before := slices.Clone(line.knots)
			if got, err := line.mapVisualSegments(segments); got != nil || !errors.Is(err, ErrIllegalArguments) {
				t.Fatal(got, err)
			}
			if !reflect.DeepEqual(before, line.knots) {
				t.Fatal("failure mutated fragments")
			}
		})
	}
}

// Exercise publication and materialization, not merely fabricated source spans.
// Empty replacement input and nonempty input producing no glyphs both retain
// nonempty provenance and must still be emitted by the permutation.
func TestKnotOrderingSilentFragments(t *testing.T) {
	k, pair := discretionaryStorageFixture(t)
	pair.pre.input.text, pair.pre.input.scalarCount = "", 0
	pair.pre.glyphs, pair.pre.widths = nil, candidateWidths{}
	pair.post.glyphs, pair.post.widths = nil, candidateWidths{}
	c, err := k.publishDiscretionary(0, 1, 0, pair)
	if err != nil {
		t.Fatal(err)
	}
	k.SelectDiscretionary(0, DiscretionarySelection{0, 1})
	p, err := k.owner.materializeLogicalLines(k, []int{0, len(k.Kind) - 1})
	if err != nil {
		t.Fatal(err)
	}
	orders, err := p.lineOrderings()
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []shapedBoxID{c.preBox, c.postBox} {
		got, err := p.lines[i].mapOrdering(orders[i])
		if err != nil || !slices.Equal(got.visual, []int{0}) || len(got.unresolved) != i || p.lines[i].knots[0].box != id {
			t.Fatal("lost silent fragment", got, err)
		}
	}
	visual, err := p.visualLines()
	if err != nil {
		t.Fatal(err)
	}
	if visual.lines[0].knots[0].box != c.preBox || visual.lines[1].knots[0].box != c.postBox ||
		visual.lines[0].naturalWidth != 0 || visual.lines[1].naturalWidth != 0 {
		t.Fatal("visual construction lost silent replacement", visual.lines)
	}
}

func TestKnotOrderingConsecutiveFragments(t *testing.T) {
	for _, text := range []string{"abcd efgh", "אבגד הוזח"} {
		t.Run(text, func(t *testing.T) {
			owner := NewKhipukamayuq(Config{})
			para := preparationParagraph(t, text)
			para.Offset = 100
			k, err := owner.EncodeParagraph(t.Context(), para)
			if err != nil {
				t.Fatal(err)
			}
			var breaks []int
			for i := range k.Kind {
				if k.Kind[i] != KTTextBox {
					continue
				}
				// Publish selected alternatives directly: this test concerns
				// placement, not language-specific discovery of opportunities.
				source := k.shaping.sources[i].source
				original := text[source.byteStart:source.byteEnd]
				split := len(string([]rune(original)[:2]))
				input, err := prepareDiscretionaryInput(original, source, HyphenationFragment{
					PreBreak: original[:split] + "-", PostBreak: original[split:], SourceSplit: SourceSplit{uint64(split), true},
				})
				if err != nil {
					t.Fatal(err)
				}
				params := k.shaping.contexts[k.shaping.boxes[k.boxRefs[i]-1].context].params
				pair, err := shapeDiscretionaryInput(t.Context(), input, params, params, func() (contracts.StreamingShaper, error) {
					return helpers.NewSyntheticStreamingShaper(0)
				}, contracts.ShapeBufferOptions{})
				if err != nil {
					t.Fatal(err)
				}
				c, err := k.publishDiscretionary(i, 1, 0, pair)
				if err != nil {
					t.Fatal(err)
				}
				k.SelectDiscretionary(i, DiscretionarySelection{i, c.Variant})
				breaks = append(breaks, i)
			}
			breaks = append(breaks, len(k.Kind)-1)
			if len(breaks) != 3 {
				t.Fatal("fixture needs consecutive replacements", breaks)
			}
			owner.env.streaming.NewShaper = func() (contracts.StreamingShaper, error) {
				t.Fatal("ordering must not reshape replacements")
				return nil, nil
			}
			p, err := owner.materializeLogicalLines(k, breaks)
			if err != nil {
				t.Fatal(err)
			}
			orders, err := p.lineOrderings()
			if err != nil {
				t.Fatal(err)
			}
			glyphs := slices.Clone(p.arena.glyphs)
			contexts := slices.Clone(p.arena.contexts)
			boxes := slices.Clone(p.arena.boxes)
			visual, err := p.visualLines()
			if err != nil {
				t.Fatal(err)
			}
			for i, line := range p.lines {
				knots := slices.Clone(line.knots)
				got, err := line.mapOrdering(orders[i])
				if err != nil {
					t.Fatal(err)
				}
				want := []int{0}
				if i == 1 {
					want = []int{0, 1, 2}
					if text[0] != 'a' {
						want = []int{2, 1, 0}
					}
				}
				if !slices.Equal(got.visual, want) || (i < 2 && len(got.unresolved) != 0) ||
					(i == 2 && !slices.Equal(got.unresolved, []int{1})) {
					t.Fatal("incorrect fragment placement", i, got)
				}
				complete := slices.Clone(want)
				if i == 2 { // these fixtures use LTR paragraph embedding
					complete = append(complete, 1)
				}
				if len(visual.lines[i].knots) != len(complete) || visual.lines[i].naturalWidth != line.naturalWidth {
					t.Fatal("visual line lost material or width", i)
				}
				for j, at := range complete {
					if visual.lines[i].knots[j] != line.knots[at] {
						t.Fatal("visual construction changed fragment", i, j)
					}
				}
				again, err := line.mapOrdering(orders[i])
				if err != nil || !reflect.DeepEqual(got, again) || !reflect.DeepEqual(knots, line.knots) {
					t.Fatal("unstable or destructive mapping", err)
				}
			}
			if !reflect.DeepEqual(glyphs, p.arena.glyphs) || !reflect.DeepEqual(contexts, p.arena.contexts) || !reflect.DeepEqual(boxes, p.arena.boxes) {
				t.Fatal("changed retained replacement data")
			}
		})
	}
}

// Storage accepts endpoint splits, but breakpoint validation requires an interior
// split. Current materialized fragments therefore have nonempty provenance;
// the mapper's separate zero-source guard protects against future extensions.
func TestKnotOrderingEndpointRejectedBeforeMapping(t *testing.T) {
	for _, split := range []uint64{0, 4} {
		k, _ := discretionaryStorageFixture(t)
		original := "aéz"
		input, err := prepareDiscretionaryInput(original, k.shaping.sources[0].source, HyphenationFragment{
			PreBreak: original[:split], PostBreak: original[split:], SourceSplit: SourceSplit{split, true},
		})
		if err != nil {
			t.Fatal(err)
		}
		params := k.shaping.contexts[0].params
		pair, err := shapeDiscretionaryInput(t.Context(), input, params, params, func() (contracts.StreamingShaper, error) {
			return helpers.NewSyntheticStreamingShaper(0)
		}, contracts.ShapeBufferOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := k.publishDiscretionary(0, 1, 0, pair); err != nil {
			t.Fatal(err)
		}
		k.SelectDiscretionary(0, DiscretionarySelection{0, 1})
		p, err := k.owner.materializeLogicalLines(k, []int{0, len(k.Kind) - 1})
		if p != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal("endpoint split must fail before mapping", p, err)
		}
	}
}
