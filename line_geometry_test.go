package khipu

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/uax/bidi"
)

type geometryShapeFunc func(int32) dimen.DU

func (f geometryShapeFunc) LineLength(n int32) dimen.DU { return f(n) }

func geometryFixture(t *testing.T) *visualParagraph {
	t.Helper()
	_, k := logicalTestParagraph(t, "aa bb")
	p, err := l1Materialized(t, k, 3).visualLines(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLineGeometrySnapshot(t *testing.T) {
	p := geometryFixture(t)
	before := slices.Clone(p.lines[0].knots)
	left := KnotCore{W: 2, MinW: 1, MaxW: dimen.Fil, Flags: KFDiscardable}
	right := KnotCore{W: 3, MinW: 0, MaxW: dimen.Filll, Penalty: 42}
	targets := []dimen.DU{15 * dimen.PT, 0} // insufficient shrink is NOT a stage-1 error
	var calls []int32
	shape := geometryShapeFunc(func(n int32) dimen.DU { calls = append(calls, n); return targets[n-1] })
	got, err := p.prepareGeometry(context.Background(), Params{LineShape: shape, LeftSkip: left, RightSkip: right})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, []int32{1, 2}) {
		t.Fatal("targets must be sampled once, one-based", calls)
	}
	for i, line := range got.lines {
		want := p.lines[i].naturalWidth + 5
		if line.leftSkip != left || line.rightSkip != right || line.naturalWidth != want ||
			line.targetWidth != targets[i] || line.delta != targets[i]-want || !reflect.DeepEqual(line.content, p.lines[i]) {
			t.Fatal("incorrect geometry", line)
		}
	}
	left.W, right.W, targets[0] = 999, 999, 999
	if got.lines[0].leftSkip.W != 2 || got.lines[0].rightSkip.W != 3 || got.lines[0].targetWidth != 15*dimen.PT {
		t.Fatal("borrowed parameters")
	}
	got.lines[0].content.knots[0].core.W++
	if !reflect.DeepEqual(before, p.lines[0].knots) {
		t.Fatal("geometry aliases visual content")
	}
}

func TestLineGeometryPhysicalEdgesAndFill(t *testing.T) {
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		for _, text := range []string{"", "   ", "abc", "אבג"} {
			t.Run(dir.String()+text, func(t *testing.T) {
				input := styled.TextFromString(text)
				para, err := styled.ParagraphFromText(&input, 0, input.Raw().Len(), dir, nil)
				if err != nil {
					t.Fatal(err)
				}
				fill := KnotCore{W: 7, MinW: 7, MaxW: dimen.Fill}
				owner := NewKhipukamayuq(Config{ParFillSkip: &fill})
				k, err := owner.EncodeParagraph(t.Context(), para)
				if err != nil {
					t.Fatal(err)
				}
				left := KnotCore{W: 2, MinW: 2, MaxW: 2}
				right := KnotCore{W: 3, MinW: 3, MaxW: 3}
				got, err := owner.materializeLineGeometry(context.Background(), k, []int{len(k.Kind) - 1}, Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 100 * dimen.PT }), LeftSkip: left, RightSkip: right})
				if err != nil {
					t.Fatal(err)
				}
				line := got.lines[0]
				fillCount := 0
				width := left.W + right.W
				for _, item := range line.content.knots {
					width += item.core.W
					if item.paragraphFill {
						fillCount++
					}
				}
				at := len(line.content.knots) - 1
				if dir == bidi.RightToLeft {
					at = 0
				}
				if fillCount != 1 || !line.content.knots[at].paragraphFill || line.leftSkip != left || line.rightSkip != right || line.naturalWidth != width {
					t.Fatal("swapped edges or double-counted fill", line)
				}
				if (text == "" || text == "   ") && line.naturalWidth != 12 {
					t.Fatal("empty-line accounting", line)
				}
			})
		}
	}
}

func TestLineGeometryNegativeFiniteSkip(t *testing.T) {
	p := geometryFixture(t)
	left := KnotCore{W: -5, MinW: -7, MaxW: -1}
	got, err := p.prepareGeometry(context.Background(), Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 1 }), LeftSkip: left, RightSkip: KnotCore{}})
	if err != nil || got.lines[0].naturalWidth != p.lines[0].naturalWidth-5 {
		t.Fatal(got, err)
	}
}

func TestLineGeometryFailures(t *testing.T) {
	for _, mode := range []string{"nil paragraph", "no source", "no lines", "nil shape", "typed nil shape", "negative target", "fill target", "reversed skip", "small max", "infinite natural", "infinite min", "infinity max", "total overflow", "negative total", "stale width", "bad content", "late target"} {
		t.Run(mode, func(t *testing.T) {
			p := geometryFixture(t)
			var shape LineShape = geometryShapeFunc(func(int32) dimen.DU { return 10 * dimen.PT })
			left, right := KnotCore{}, KnotCore{}
			switch mode {
			case "nil paragraph":
				p = nil
			case "no source":
				p.paragraph = nil
			case "no lines":
				p.lines = nil
			case "nil shape":
				shape = nil
			case "typed nil shape":
				shape = geometryShapeFunc(nil)
			case "negative target":
				shape = geometryShapeFunc(func(int32) dimen.DU { return -1 })
			case "fill target":
				shape = geometryShapeFunc(func(int32) dimen.DU { return dimen.Fil })
			case "reversed skip":
				left.MinW = 1
			case "small max":
				left.MaxW = -1
			case "infinite natural":
				left.W, left.MaxW = dimen.Fil, dimen.Fill
			case "infinite min":
				left.MinW = -dimen.Fil
			case "infinity max":
				left.MaxW = dimen.Infinity
			case "total overflow":
				left.W, left.MaxW = dimen.Fil-1, dimen.Fill
				right = left
			case "negative total":
				left.W, left.MinW = -20*dimen.PT, -20*dimen.PT
			case "stale width":
				p.lines[0].naturalWidth++
			case "bad content":
				p.lines[1].knots[0].core.MinW = dimen.Fil
			case "late target":
				shape = geometryShapeFunc(func(n int32) dimen.DU {
					if n == 2 {
						return -1
					}
					return 10 * dimen.PT
				})
			}
			var before []logicalKnot
			if p != nil && len(p.lines) > 0 {
				before = slices.Clone(p.lines[0].knots)
			}
			got, err := p.prepareGeometry(context.Background(), Params{LineShape: shape, LeftSkip: left, RightSkip: right})
			if got != nil || !errors.Is(err, ErrIllegalArguments) {
				t.Fatal("accepted invalid geometry", got, err)
			}
			if mode == "late target" && !strings.Contains(err.Error(), "line 2") {
				t.Fatal(err)
			}
			if before != nil && !reflect.DeepEqual(before, p.lines[0].knots) {
				t.Fatal("failure changed input")
			}
			if mode == "late target" {
				if _, err := p.prepareGeometry(context.Background(), Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 0 }), LeftSkip: left, RightSkip: right}); err != nil {
					t.Fatal("retry failed", err)
				}
			}
		})
	}
}
