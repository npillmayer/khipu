package khipu

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

// Check the publication boundary, not merely the allocator: all planes must
// align, metadata must survive, and the final physical line must conserve width.
func checkFrozenGeometry(t *testing.T, got *FrozenKhipu, geometry *geometryParagraph) {
	t.Helper()
	n := len(got.w)
	if len(got.pos) != n || len(got.len) != n || len(got.kind) != n || len(got.flags) != n || len(got.origins) != n || len(got.lines) != len(geometry.lines) {
		t.Fatal("unaligned frozen planes")
	}
	end := 0
	for i, line := range got.lines {
		original := geometry.lines[i].content
		if line.content.start != end || line.content.end-line.content.start != len(original.knots) ||
			line.byteStart != original.byteStart || line.byteEnd != original.byteEnd || line.width != geometry.lines[i].targetWidth {
			t.Fatal("lost line structure", i, line)
		}
		end = line.content.end
		sum := int64(line.leftSkip) + int64(line.rightSkip)
		for j, knot := range original.knots {
			at := line.content.start + j
			sum += int64(got.w[at])
			want := frozenOrigin{knot.source, knot.box, knot.origin, knot.variant, knot.fragment, knot.paragraphFill}
			if got.pos[at] != knot.core.Pos || got.len[at] != knot.core.Len || got.kind[at] != knot.core.Kind || got.flags[at] != knot.core.Flags || got.origins[at] != want {
				t.Fatal("lost knot identity", at)
			}
			if got.w[at] < knot.core.MinW || got.w[at] > knot.core.MaxW || (knot.core.Kind != KTGlue && got.w[at] != knot.core.W) {
				t.Fatal("unrealizable final width", at, got.w[at])
			}
		}
		if sum != int64(line.width) {
			t.Fatal("final width does not equal inclusive target", sum, line.width)
		}
	}
	if end != n || !reflect.DeepEqual(got.shaping.glyphs, geometry.arena.glyphs) ||
		!reflect.DeepEqual(got.shaping.boxes, geometry.arena.boxes) || !reflect.DeepEqual(got.shaping.contexts, geometry.arena.contexts) ||
		!reflect.DeepEqual(got.shaping.clusters, geometry.arena.clusters) {
		t.Fatal("lost shaping storage")
	}
}

func TestFreezeFiniteGlue(t *testing.T) {
	for _, delta := range []dimen.DU{0, dimen.PT, -dimen.PT} {
		owner, k := logicalTestParagraph(t, "aa bb")
		// A fixed terminal makes finite interword capacity observable.
		k.MaxW[len(k.Kind)-1] = 0
		shape := geometryShapeFunc(func(int32) dimen.DU { return 25*dimen.PT + delta })
		g, err := owner.materializeLineGeometry(context.Background(), k, []int{len(k.Kind) - 1}, Params{LineShape: shape, LeftSkip: KnotCore{}, RightSkip: KnotCore{}})
		if err != nil {
			t.Fatal(err)
		}
		before := slices.Clone(g.lines[0].content.knots)
		got, err := g.freeze(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		checkFrozenGeometry(t, got, g)
		if got.w[1] != 5*dimen.PT+delta || !reflect.DeepEqual(before, g.lines[0].content.knots) {
			t.Fatal("wrong interword allocation or changed input", got.w)
		}
		got.w[0]++
		got.origins[0].origin++
		again, err := g.freeze(context.Background())
		if err != nil || again.w[0] != k.W[0] || again.origins[0].origin != 0 {
			t.Fatal("output shares mutable planes", err)
		}
	}
}

func TestFreezeSideSkipsAndFill(t *testing.T) {
	// Side skips are glue by role even when supplied with a textbox Kind.
	left, right := allocationItem(dimen.PT, 0, 10*dimen.PT), allocationItem(2*dimen.PT, 0, 10*dimen.PT)
	left.Kind = KTTextBox
	g, err := geometryFixture(t).prepareGeometry(context.Background(), Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 20 * dimen.PT }), LeftSkip: left, RightSkip: right})
	if err != nil {
		t.Fatal(err)
	}
	got, err := g.freeze(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	checkFrozenGeometry(t, got, g)
	if got.lines[0].leftSkip <= left.W || got.lines[0].rightSkip <= right.W || got.lines[1].leftSkip != left.W || got.lines[1].rightSkip != right.W || got.w[len(got.w)-1] != 7*dimen.PT {
		t.Fatal("wrong skip/fill allocation", got)
	}
}

func TestFreezeRejectsElasticNonGlue(t *testing.T) {
	for _, kind := range []KnotType{KTTextBox, KTKern, KTUserDefined} {
		for _, shrink := range []bool{false, true} {
			p := geometryFixture(t)
			core := &p.lines[1].knots[0].core // final line also has fill
			core.Kind = kind
			if shrink {
				core.MinW--
			} else {
				core.MaxW++
			}
			g, err := p.prepareGeometry(context.Background(), Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 10 * dimen.PT }), LeftSkip: KnotCore{}, RightSkip: KnotCore{}})
			if err != nil {
				t.Fatal(err)
			}
			if got, err := g.freeze(context.Background()); got != nil || !errors.Is(err, ErrUnsupportedAdjustment) || !strings.Contains(err.Error(), "line 2 knot 0") {
				t.Fatal("accepted declared non-glue capacity on exact-fit line", got, err)
			}
			g.lines[1].targetWidth++
			g.lines[1].delta++
			if got, err := g.freeze(context.Background()); got != nil || !errors.Is(err, ErrUnsupportedAdjustment) {
				t.Fatal("fill concealed unsupported capacity", got, err)
			}
			g.lines[1].content.knots[0].core.MinW = core.W
			g.lines[1].content.knots[0].core.MaxW = core.W
			got, err := g.freeze(context.Background())
			if err != nil {
				t.Fatal("fixed material rejected", err)
			}
			checkFrozenGeometry(t, got, g)
		}
	}
}

func TestFreezeFailureAndRetry(t *testing.T) {
	p := geometryFixture(t)
	p.lines[1].knots[len(p.lines[1].knots)-1].core.MaxW = 0
	g, err := p.prepareGeometry(context.Background(), Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 10 * dimen.PT }), LeftSkip: KnotCore{}, RightSkip: KnotCore{}})
	if err != nil {
		t.Fatal(err)
	}
	g.lines[1].targetWidth++
	want := ErrIllegalArguments
	before := slices.Clone(g.lines[1].content.knots)
	if got, err := g.freeze(context.Background()); got != nil || !errors.Is(err, want) || !strings.Contains(err.Error(), "line 2") {
		t.Fatal("published a prefix or lost cause", got, err)
	}
	if !reflect.DeepEqual(before, g.lines[1].content.knots) {
		t.Fatal("failure changed input")
	}
	g.lines[1].targetWidth--
	g.lines[1].delta = 0
	got, err := g.freeze(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	checkFrozenGeometry(t, got, g)

	if got, err := (*geometryParagraph)(nil).freeze(context.Background()); got != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal(got, err)
	}
	if got, err := (*Khipukamayuq)(nil).Freeze(context.Background(), nil, nil, Params{LineShape: nil, LeftSkip: KnotCore{}, RightSkip: KnotCore{}}); got != nil || !errors.Is(err, ErrVoidKhipukamayuq) {
		t.Fatal(got, err)
	}
}

func TestFreezeSelectedReplacementLifetime(t *testing.T) {
	owner, k, candidate := logicalTestDiscretionary(t)
	owner.env.streaming.NewShaper = func() (contracts.StreamingShaper, error) { t.Fatal("freeze reshaped input"); return nil, nil }
	selections := maps.Clone(k.SelectedDiscretionaries)
	glyphs := slices.Clone(k.shaping.glyphs)
	widths := slices.Clone(k.W)
	shape := geometryShapeFunc(func(int32) dimen.DU { return 30 * dimen.PT })
	right := KnotCore{MaxW: dimen.Fil}
	got, err := owner.Freeze(context.Background(), k, []int{0, len(k.Kind) - 1}, Params{LineShape: shape, LeftSkip: KnotCore{}, RightSkip: right})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selections, k.SelectedDiscretionaries) || !slices.Equal(k.W, widths) || !reflect.DeepEqual(glyphs, k.shaping.glyphs) {
		t.Fatal("freeze changed input")
	}
	if got.origins[0].box != candidate.preBox || got.origins[1].box != candidate.postBox ||
		got.origins[0].fragment != logicalPreBreak || got.origins[1].fragment != logicalPostBreak ||
		got.origins[0].variant != 4 || got.origins[1].variant != 4 || got.lines[0].byteEnd != 3 || got.lines[1].byteStart != 3 {
		t.Fatal("lost selected replacement/provenance")
	}
	clear(k.SelectedDiscretionaries)
	unwireKhipu(k)
	if !reflect.DeepEqual(got.shaping.glyphs, glyphs) || got.shaping.boxes[candidate.preBox-1].replacement.text != "LONG-" || got.paragraph.Raw().String() != "éclair" || got.w[0] != 25*dimen.PT || got.w[1] != 20*dimen.PT {
		t.Fatal("frozen result lost retained storage")
	}
}

func TestFreezeSilentReplacements(t *testing.T) {
	k, pair := discretionaryStorageFixture(t)
	pair.pre.input.text, pair.pre.input.scalarCount = "", 0
	pair.pre.glyphs, pair.pre.widths = nil, candidateWidths{}
	pair.post.glyphs, pair.post.widths = nil, candidateWidths{}
	c, err := k.publishDiscretionary(0, 1, 0, pair)
	if err != nil {
		t.Fatal(err)
	}
	k.SelectDiscretionary(0, DiscretionarySelection{0, 1})
	got, err := k.owner.Freeze(context.Background(), k, []int{0, len(k.Kind) - 1}, Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 10 * dimen.PT }), LeftSkip: KnotCore{}, RightSkip: KnotCore{MaxW: dimen.Fil}})
	if err != nil {
		t.Fatal(err)
	}
	if got.origins[0].box != c.preBox || got.origins[1].box != c.postBox || got.w[0] != 0 || got.w[1] != 0 ||
		got.lines[0].byteEnd != 3 || got.lines[1].byteStart != 3 || got.lines[1].byteEnd != 4 ||
		got.shaping.boxes[c.preBox-1].replacement.text != "" || got.shaping.boxes[c.postBox-1].replacement.text != "z" {
		t.Fatal("lost silent replacement or source coverage", got)
	}
}

func TestFreezeEmptyAndTrimmed(t *testing.T) {
	for _, text := range []string{"", "   ", "  aa  "} {
		owner, k := logicalTestParagraph(t, text)
		got, err := owner.Freeze(context.Background(), k, []int{len(k.Kind) - 1}, Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 20 * dimen.PT }), LeftSkip: KnotCore{}, RightSkip: KnotCore{}})
		if err != nil {
			t.Fatal(err)
		}
		wantKnots, wantFill := 1, 20*dimen.PT
		if text == "  aa  " {
			wantKnots, wantFill = 2, 10*dimen.PT
		}
		if len(got.w) != wantKnots || got.w[len(got.w)-1] != wantFill || !got.origins[len(got.w)-1].paragraphFill ||
			got.lines[0].byteStart != 0 || got.lines[0].byteEnd != uint64(len(text)) {
			t.Fatal("trimmed/empty line lost its fill or coverage", text, got)
		}
	}
}
