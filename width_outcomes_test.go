package khipu

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/uax/bidi"
)

func TestWidthAllocationOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		items   []KnotCore
		target  dimen.DU
		widths  []dimen.DU
		fit     widthFit
		ratio   float64
		badness int
		fill    dimen.DU
	}{
		{"exact", []KnotCore{allocationItem(10, 10, 10)}, 10, []dimen.DU{10}, widthFits, 0, 0, 0},
		{"empty slack", nil, 5, nil, widthUnderfull, math.Inf(1), 10000, 0},
		{"no stretch", []KnotCore{allocationItem(10, 5, 10)}, 15, []dimen.DU{10}, widthUnderfull, math.Inf(1), 10000, 0},
		{"stretch limit", []KnotCore{allocationItem(10, 5, 15)}, 15, []dimen.DU{15}, widthFits, 1, 100, 0},
		{"excess stretch", []KnotCore{allocationItem(10, 5, 15)}, 20, []dimen.DU{20}, widthUnderfull, 2, 800, 0},
		{"stretch rounding", []KnotCore{allocationItem(10, 10, 11), allocationItem(10, 10, 12)}, 27, []dimen.DU{12, 15}, widthUnderfull, 7.0 / 3, 1270, 0},
		{"saturated badness", []KnotCore{allocationItem(0, 0, 1)}, 100, []dimen.DU{100}, widthUnderfull, 100, 10000, 0},
		{"shrink limit", []KnotCore{allocationItem(10, 5, 20)}, 5, []dimen.DU{5}, widthFits, -1, 100, 0},
		{"exhausted shrink", []KnotCore{allocationItem(10, 5, 20), allocationItem(10, 8, 20)}, 12, []dimen.DU{5, 8}, widthOverfull, -8.0 / 7, 10000, 0},
		{"no shrink", []KnotCore{allocationItem(10, 10, dimen.Fill)}, 9, []dimen.DU{10}, widthOverfull, math.Inf(-1), 10000, 0},
		{"fill beats finite", []KnotCore{allocationItem(10, 10, 11), allocationItem(0, 0, dimen.Fill)}, 100, []dimen.DU{10, 90}, widthFits, 0, 0, dimen.Fill},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := slices.Clone(tc.items)
			w, a, err := allocateWidths(tc.items, tc.target)
			if err != nil || !slices.Equal(w, tc.widths) || a.fit != tc.fit || a.ratio != tc.ratio || a.badness != tc.badness || a.fill != tc.fill {
				t.Fatalf("widths=%v outcome=%+v err=%v", w, a, err)
			}
			var sum dimen.DU
			for i, v := range w {
				sum += v
				if v < tc.items[i].MinW {
					t.Fatal("shrank below minimum")
				}
			}
			if a.actual != sum || a.target != tc.target || a.residual != tc.target-sum {
				t.Fatal("inconsistent accounting", a)
			}
			if !reflect.DeepEqual(before, tc.items) {
				t.Fatal("mutated input")
			}
			w2, a2, err := allocateWidths(tc.items, tc.target)
			if err != nil || !slices.Equal(w, w2) || a != a2 {
				t.Fatal("nondeterministic outcome")
			}
		})
	}
}

func TestFrozenOutcomesAcrossDirections(t *testing.T) {
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		for _, target := range []dimen.DU{20 * dimen.PT, 30 * dimen.PT} {
			text := styled.TextFromString("אב גד")
			para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			owner := NewKhipukamayuq(Config{ParFillSkip: &KnotCore{}})
			k, err := owner.EncodeParagraph(t.Context(), para)
			if err != nil {
				t.Fatal(err)
			}
			before := slices.Clone(k.W)
			layout := Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return target })}
			g, err := owner.materializeLineGeometry(k, []int{len(k.Kind) - 1}, layout)
			if err != nil {
				t.Fatal(err)
			}
			f, err := g.freeze()
			if err != nil {
				t.Fatal(err)
			}
			line := f.lines[0]
			sum := line.leftSkip + line.rightSkip
			for i, w := range f.w {
				sum += w
				core := g.lines[0].content.knots[i].core
				if w < core.MinW || (core.Kind != KTGlue && w != core.W) {
					t.Fatal("invalid realized width")
				}
			}
			if sum != line.width || line.adjustment.residual != target-sum || !slices.Equal(before, k.W) {
				t.Fatal("inconsistent or mutated output")
			}
			if target == 20*dimen.PT {
				if line.adjustment.fit != widthOverfull || line.adjustment.residual >= 0 {
					t.Fatal(line.adjustment)
				}
			} else if line.adjustment.fit != widthUnderfull || line.adjustment.residual != 0 || math.Abs(line.adjustment.ratio-2) > 0.0001 || line.adjustment.badness != 800 {
				t.Fatal(line.adjustment)
			}
		}
	}
}

func TestFreezePublishesImperfectLines(t *testing.T) {
	for _, target := range []dimen.DU{9 * dimen.PT, 11 * dimen.PT} {
		p := geometryFixture(t)
		// Remove terminal stretch so both lines have fixed 10pt content.
		p.lines[1].knots[len(p.lines[1].knots)-1].core.MaxW = 0
		g, err := p.prepareGeometry(Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return target })})
		if err != nil {
			t.Fatal(err)
		}
		f, err := g.freeze()
		if err != nil || len(f.lines) != 2 {
			t.Fatal(f, err)
		}
		for _, line := range f.lines {
			fit := widthUnderfull
			if target < 10*dimen.PT {
				fit = widthOverfull
			}
			if line.width != 10*dimen.PT || line.adjustment.actual != line.width || line.adjustment.target != target || line.adjustment.residual != target-line.width || line.adjustment.fit != fit {
				t.Fatal("lost allocation outcome", line)
			}
		}
	}
}
