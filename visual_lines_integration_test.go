package khipu_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

func TestVisualLinesFromBothBreakers(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		for _, tc := range []struct {
			text      string
			dir       bidi.Direction
			positions [][]uint64
		}{
			{"Go אב גד", bidi.LeftToRight, [][]uint64{{0, 2, 8, 7, 3, 12}}},
			{"Go אב גד", bidi.RightToLeft, [][]uint64{{12, 8, 7, 3, 2, 0}}},
			{"אב גד\nabc", bidi.RightToLeft, [][]uint64{{5, 4, 0}, {13, 10}}},
			{"", bidi.RightToLeft, [][]uint64{{0}}},
		} {
			t.Run(engine+tc.dir.String()+tc.text, func(t *testing.T) {
				input := styled.TextFromString(tc.text)
				para, err := styled.ParagraphFromText(&input, 0, input.Raw().Len(), tc.dir, nil)
				if err != nil {
					t.Fatal(err)
				}
				para.Offset = 100
				owner := khipu.NewKhipukamayuq(khipu.Config{})
				k, err := owner.EncodeParagraph(t.Context(), para)
				if err != nil {
					t.Fatal(err)
				}
				shape := linebreak.RectangularParShape(200 * dimen.PT)
				var breaks []int
				if engine == "firstfit" {
					params := *firstfit.DefaultParameters
					breaks, err = firstfit.BreakParagraph(k, shape, &params)
				} else {
					breaks, err = knuthplass.BreakParagraph(k, shape, knuthplass.NewKPDefaultParameters())
				}
				if err != nil {
					t.Fatal(err)
				}
				levels := para.BidiLevels().Clone()
				logical, err := khipu.MaterializeLogicalLinesForTest(owner, k, breaks)
				if err != nil {
					t.Fatal(err)
				}
				visual, err := khipu.MaterializeVisualLinesForTest(owner, k, breaks)
				if err != nil {
					t.Fatal(err)
				}
				if len(visual) != len(tc.positions) {
					t.Fatal(visual, breaks)
				}
				for i, line := range visual {
					var positions []uint64
					var width dimen.DU
					for _, knot := range line.Knots {
						positions = append(positions, knot.Pos-para.Offset)
						width += knot.W
					}
					if !slices.Equal(positions, tc.positions[i]) || width != line.NaturalWidth ||
						line.NaturalWidth != logical[i].NaturalWidth || line.ByteStart != logical[i].ByteStart || line.ByteEnd != logical[i].ByteEnd {
						t.Fatal("wrong visual sequence or geometry", i, positions, line)
					}
				}
				again, err := khipu.MaterializeVisualLinesForTest(owner, k, breaks)
				if err != nil || !reflect.DeepEqual(again, visual) || !reflect.DeepEqual(levels, para.BidiLevels()) {
					t.Fatal("changed input or unstable result", err)
				}
			})
		}
	}
}
