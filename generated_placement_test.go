package khipu

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/uax/bidi"
)

func TestGeneratedPlacementParagraphEdge(t *testing.T) {
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		for _, text := range []string{"", "   ", "abc", "אבג", "abc אבג", "אבג abc", "a\u2067אב\u2069"} {
			t.Run(dir.String()+"/"+text, func(t *testing.T) {
				input := styled.TextFromString(text)
				para, err := styled.ParagraphFromText(&input, 0, input.Raw().Len(), dir, nil)
				if err != nil {
					t.Fatal(err)
				}
				para.Offset = 100
				// Finite, nonzero fill proves identity is not guessed from width
				// sentinels. Owner defaults are LTR even for the RTL paragraphs.
				fill := KnotCore{W: 2 * dimen.PT, MinW: dimen.PT, MaxW: 3 * dimen.PT}
				owner := NewKhipukamayuq(Config{ParFillSkip: &fill})
				k, err := owner.EncodeParagraph(t.Context(), para)
				if err != nil {
					t.Fatal(err)
				}
				p := l1Materialized(t, k)
				orders, err := p.lineOrderings(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				line := p.lines[0]
				knots, glyphs := slices.Clone(line.knots), slices.Clone(p.arena.glyphs)
				partial, err := line.mapOrdering(orders[0])
				if err != nil {
					t.Fatal(err)
				}
				before := &knotOrdering{slices.Clone(partial.visual), slices.Clone(partial.unresolved)}
				got, err := line.placeGenerated(partial, dir)
				if err != nil {
					t.Fatal(err)
				}
				trailer := len(line.knots) - 1
				want := append(slices.Clone(partial.visual), trailer)
				if dir == bidi.RightToLeft {
					want = append([]int{trailer}, partial.visual...)
				}
				if !slices.Equal(got, want) || len(got) != len(line.knots) || !line.knots[trailer].paragraphFill {
					t.Fatal("wrong fill placement", got, want)
				}
				if line.knots[trailer].core.W != fill.W || line.knots[trailer].core.MinW != fill.MinW || line.knots[trailer].core.MaxW != fill.MaxW {
					t.Fatal("changed fill geometry")
				}
				again, err := line.placeGenerated(partial, dir)
				if err != nil || !slices.Equal(got, again) {
					t.Fatal("not repeatable", again, err)
				}
				got[0] = -1 // returned indices must not alias the partial mapping
				if !reflect.DeepEqual(partial, before) || !reflect.DeepEqual(knots, line.knots) || !reflect.DeepEqual(glyphs, p.arena.glyphs) {
					t.Fatal("placement changed input")
				}
			})
		}
	}
}

func TestGeneratedPlacementWrappedLines(t *testing.T) {
	_, k := logicalTestParagraph(t, "  aa bb ")
	p := l1Materialized(t, k, 2, 5)
	orders, err := p.lineOrderings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range p.lines {
		partial, err := line.mapOrdering(orders[i])
		if err != nil {
			t.Fatal(err)
		}
		got, err := line.placeGenerated(partial, bidi.LeftToRight)
		if err != nil || len(got) != len(line.knots) {
			t.Fatal(got, err)
		}
		if i < len(p.lines)-1 && (!slices.Equal(got, partial.visual) || len(partial.unresolved) != 0) {
			t.Fatal("invented fill on intermediate line", got)
		}
	}
}

func TestGeneratedPlacementAfterDiscretionary(t *testing.T) {
	owner, k, _ := logicalTestDiscretionary(t)
	p, err := owner.materializeLogicalLines(context.Background(), k, []int{0, len(k.Kind) - 1})
	if err != nil {
		t.Fatal(err)
	}
	orders, err := p.lineOrderings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range p.lines {
		partial, err := line.mapOrdering(orders[i])
		if err != nil {
			t.Fatal(err)
		}
		got, err := line.placeGenerated(partial, bidi.LeftToRight)
		want := []int{0}
		if i == 1 {
			want = []int{0, 1}
		}
		if err != nil || !slices.Equal(got, want) {
			t.Fatal(got, err)
		}
	}
}

func TestGeneratedPlacementRejectsAmbiguity(t *testing.T) {
	for _, mode := range []string{"nil", "direction", "negative", "outside", "duplicate", "missing", "unmarked", "kern", "discardable", "not forced", "source", "len", "fragment", "box", "origin", "premapped fill", "unresolved text"} {
		t.Run(mode, func(t *testing.T) {
			_, k := logicalTestParagraph(t, "abc")
			p := l1Materialized(t, k)
			line := p.lines[0]
			partial := &knotOrdering{visual: []int{0}, unresolved: []int{1}}
			dir := bidi.LeftToRight
			fill := &line.knots[1]
			switch mode {
			case "nil":
				partial = nil
			case "direction":
				dir = bidi.Direction(99)
			case "negative":
				partial.visual[0] = -1
			case "outside":
				partial.visual[0] = 9
			case "duplicate":
				partial.visual = []int{0, 0}
			case "missing":
				partial.unresolved = nil
			case "unmarked":
				fill.paragraphFill = false
			case "kern":
				fill.core.Kind = KTKern
			case "discardable":
				fill.core.Flags |= KFDiscardable
			case "not forced":
				fill.core.Penalty = 0
			case "source":
				fill.source.byteStart--
			case "len":
				fill.core.Len = 1
			case "fragment":
				fill.fragment = logicalPostBreak
			case "box":
				fill.box = 1
			case "origin":
				fill.origin = 999
			case "premapped fill":
				partial.visual, partial.unresolved = []int{0, 1}, nil
			case "unresolved text":
				partial.visual, partial.unresolved = nil, []int{0, 1}
			}
			knots := slices.Clone(line.knots)
			if got, err := line.placeGenerated(partial, dir); got != nil || !errors.Is(err, ErrIllegalArguments) {
				t.Fatal("accepted invalid attachment", got, err)
			}
			if !reflect.DeepEqual(knots, line.knots) {
				t.Fatal("failure changed input")
			}
		})
	}
}
