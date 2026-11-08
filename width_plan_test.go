package khipu

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/uax/bidi"
)

func TestWidthPlanGeometry(t *testing.T) {
	p := geometryFixture(t)
	left, right := allocationItem(dimen.PT, 0, 10*dimen.PT), allocationItem(2*dimen.PT, 0, 10*dimen.PT)
	g, err := p.prepareGeometry(geometryShapeFunc(func(int32) dimen.DU { return 20 * dimen.PT }), left, right)
	if err != nil {
		t.Fatal(err)
	}
	glyphs := slices.Clone(g.arena.glyphs)
	for i, line := range g.lines {
		before := slices.Clone(line.content.knots)
		plan, err := line.planWidths()
		if err != nil {
			t.Fatal(err)
		}
		sum := int64(plan.leftSkip) + int64(plan.rightSkip)
		for j, w := range plan.content {
			sum += int64(w)
			k := line.content.knots[j]
			if k.core.Kind == KTTextBox && w != k.core.W {
				t.Fatal("changed fixed text")
			}
			if k.paragraphFill && w != 7*dimen.PT {
				t.Fatal("did not allocate residual to fill", w)
			}
		}
		if sum != int64(line.targetWidth) {
			t.Fatal("did not count side skips exactly once", sum)
		}
		if i == 0 && (plan.leftSkip <= left.W || plan.rightSkip <= right.W) {
			t.Fatal("finite skips did not share stretch", plan)
		}
		if i == 1 && (plan.leftSkip != left.W || plan.rightSkip != right.W) {
			t.Fatal("fill did not override finite skips")
		}
		again, err := line.planWidths()
		if err != nil || !reflect.DeepEqual(plan, again) {
			t.Fatal("unstable proposal", err)
		}
		plan.content[0]++
		if !reflect.DeepEqual(before, line.content.knots) || !reflect.DeepEqual(glyphs, g.arena.glyphs) || reflect.DeepEqual(plan, again) {
			t.Fatal("proposal aliases geometry or glyphs")
		}
	}
}

func TestWidthPlanRTLAndFillOrders(t *testing.T) {
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		input := styled.TextFromString("אב גד")
		para, err := styled.ParagraphFromText(&input, 0, input.Raw().Len(), dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		owner := NewKhipukamayuq(Config{})
		k, err := owner.EncodeParagraph(t.Context(), para)
		if err != nil {
			t.Fatal(err)
		}
		for _, order := range []dimen.DU{dimen.Fil, dimen.Fill, dimen.Filll} {
			g, err := owner.materializeLineGeometry(k, []int{len(k.Kind) - 1}, geometryShapeFunc(func(int32) dimen.DU { return 25*dimen.PT + 5 }), allocationItem(0, 0, order), KnotCore{})
			if err != nil {
				t.Fatal(err)
			}
			line := g.lines[0]
			plan, err := line.planWidths()
			if err != nil {
				t.Fatal(err)
			}
			at := len(plan.content) - 1
			if dir == bidi.RightToLeft {
				at = 0
			}
			if !line.content.knots[at].paragraphFill {
				t.Fatal("fixture fill position")
			}
			wantLeft, wantFill := dimen.DU(0), dimen.DU(5)
			if order == dimen.Fill {
				wantLeft, wantFill = 3, 2
			}
			if order == dimen.Filll {
				wantLeft, wantFill = 5, 0
			}
			if plan.leftSkip != wantLeft || plan.rightSkip != 0 || plan.content[at] != wantFill {
				t.Fatal(dir, order, plan)
			}
			frozen, err := g.freeze()
			if err != nil {
				t.Fatal(err)
			}
			checkFrozenGeometry(t, frozen, g)
			if !slices.Equal(frozen.w, plan.content) || frozen.lines[0].leftSkip != wantLeft || frozen.w[at] != wantFill {
				t.Fatal("frozen fill allocation differs from plan", dir, order)
			}
			for j, w := range plan.content {
				if j != at && w != line.content.knots[j].core.W {
					t.Fatal("fill stretched ordinary content")
				}
			}
		}
	}
}

func TestWidthPlanElasticTextboxIsOnlyProposal(t *testing.T) {
	p := geometryFixture(t)
	// Model a future measured textbox capacity, without altering its glyphs.
	p.lines[0].knots[0].core.MaxW += 10
	g, err := p.prepareGeometry(geometryShapeFunc(func(int32) dimen.DU { return 10*dimen.PT + 5 }), KnotCore{}, KnotCore{})
	if err != nil {
		t.Fatal(err)
	}
	before := g.lines[0].content.knots[0]
	glyphs := slices.Clone(g.arena.glyphs)
	plan, err := g.lines[0].planWidths()
	if err != nil || plan.content[0] != before.core.W+5 {
		t.Fatal(plan, err)
	}
	if g.lines[0].content.knots[0] != before || !reflect.DeepEqual(glyphs, g.arena.glyphs) {
		t.Fatal("realized a numeric proposal")
	}
}

func TestWidthPlanRejectsStaleGeometry(t *testing.T) {
	for _, mode := range []string{"content", "natural", "delta", "target", "capacity"} {
		t.Run(mode, func(t *testing.T) {
			g, err := geometryFixture(t).prepareGeometry(geometryShapeFunc(func(int32) dimen.DU { return 10 * dimen.PT }), KnotCore{}, KnotCore{})
			if err != nil {
				t.Fatal(err)
			}
			line := g.lines[0]
			want := ErrIllegalArguments
			switch mode {
			case "content":
				line.content.naturalWidth++
			case "natural":
				line.naturalWidth++
			case "delta":
				line.delta++
			case "target":
				line.targetWidth = dimen.Fill
				line.delta = line.targetWidth - line.naturalWidth
			case "capacity":
				line.targetWidth++
				line.delta++
				want = errInsufficientCapacity
			}
			before := slices.Clone(line.content.knots)
			if got, err := line.planWidths(); got != nil || !errors.Is(err, want) {
				t.Fatal(got, err)
			}
			if !reflect.DeepEqual(before, line.content.knots) {
				t.Fatal("error changed geometry")
			}
		})
	}
}
