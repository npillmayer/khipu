package khipu

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/uax/bidi"
)

func TestVisualLinesSnapshot(t *testing.T) {
	for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.RightToLeft} {
		for _, text := range []string{"", "   ", "Go אב גד", "a\u2067א\u2066xy\u2069ב\u2069z"} {
			t.Run(dir.String()+text, func(t *testing.T) {
				input := styled.TextFromString(text)
				para, err := styled.ParagraphFromText(&input, 0, input.Raw().Len(), dir, nil)
				if err != nil {
					t.Fatal(err)
				}
				para.Offset = 100
				owner := NewKhipukamayuq(Config{}) // default Params direction is LTR
				k, err := owner.EncodeParagraph(t.Context(), para)
				if err != nil {
					t.Fatal(err)
				}
				p := l1Materialized(t, k)
				before := slices.Clone(p.lines[0].knots)
				levels := para.BidiLevels().Clone()
				glyphs := slices.Clone(p.arena.glyphs)
				got, err := p.visualLines(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				v := got.lines[0]
				if len(v.knots) != len(before) || v.byteStart != p.lines[0].byteStart || v.byteEnd != p.lines[0].byteEnd ||
					v.breakpoint != p.lines[0].breakpoint || v.naturalWidth != p.lines[0].naturalWidth {
					t.Fatal("lost line metadata", v)
				}
				seen := make(map[logicalKnot]int)
				for _, knot := range before {
					seen[knot]++
				}
				for _, knot := range v.knots {
					seen[knot]--
				}
				for _, count := range seen {
					if count != 0 {
						t.Fatal("changed or duplicated knot")
					}
				}
				fillIndex := len(v.knots) - 1
				if dir == bidi.RightToLeft {
					fillIndex = 0
				}
				if !v.knots[fillIndex].paragraphFill {
					t.Fatal("ignored authoritative paragraph direction")
				}
				if dir == bidi.LeftToRight && strings.Contains(text, "\u2066") {
					var starts []uint64
					for _, knot := range v.knots {
						starts = append(starts, knot.source.byteStart)
					}
					if !slices.Equal(starts, []uint64{0, 14, 9, 4, 19, 20}) {
						t.Fatal("incorrect nested-isolate visual order", starts)
					}
				}
				again, err := p.visualLines(context.Background())
				if err != nil || !reflect.DeepEqual(got, again) || !reflect.DeepEqual(levels, para.BidiLevels()) ||
					!reflect.DeepEqual(before, p.lines[0].knots) || !reflect.DeepEqual(glyphs, p.arena.glyphs) {
					t.Fatal("mutated input or unstable output", err)
				}
				got.lines[0].knots[0].core.W++
				if !reflect.DeepEqual(before, p.lines[0].knots) || reflect.DeepEqual(got.lines, again.lines) {
					t.Fatal("visual and logical slices alias")
				}
				unwireKhipu(k)
				if !reflect.DeepEqual(glyphs, again.arena.glyphs) || again.paragraph != para {
					t.Fatal("lost retained arena/source")
				}
			})
		}
	}
}

func TestVisualLinesFailureIsAtomic(t *testing.T) {
	if got, err := (*logicalParagraph)(nil).visualLines(context.Background()); got != nil || !errors.Is(err, ErrIllegalArguments) {
		t.Fatal(got, err)
	}
	for _, mode := range []string{"mapping", "placement"} {
		t.Run(mode, func(t *testing.T) {
			_, k := logicalTestParagraph(t, "aa bb")
			p := l1Materialized(t, k, 3)
			levels := p.paragraph.BidiLevels().Clone()
			original := slices.Clone(p.lines[1].knots)
			if mode == "mapping" {
				p.lines[1].knots[0].source.byteStart = 0
			} else {
				p.lines[1].knots[len(original)-1].paragraphFill = false
			}
			bad := slices.Clone(p.lines[1].knots)
			got, err := p.visualLines(context.Background())
			if got != nil || !errors.Is(err, ErrIllegalArguments) || !strings.Contains(err.Error(), "line 2") {
				t.Fatal("published partial result or lost context", got, err)
			}
			if !reflect.DeepEqual(bad, p.lines[1].knots) || !reflect.DeepEqual(levels, p.paragraph.BidiLevels()) {
				t.Fatal("failure consumed input")
			}
			p.lines[1].knots = original
			if _, err := p.visualLines(context.Background()); err != nil {
				t.Fatal("cannot retry", err)
			}
		})
	}
}
