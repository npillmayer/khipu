package khipu_test

import (
	"reflect"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

func TestLogicalLinesFromBothBreakers(t *testing.T) {
	for _, engine := range []string{"firstfit", "knuthplass"} {
		for _, tc := range []struct {
			text          string
			widths        []dimen.DU
			discretionary bool
		}{
			{"", []dimen.DU{0}, false},
			{"  aa  bb  ", []dimen.DU{10 * dimen.PT, 10 * dimen.PT}, false},
			{"aa\nbb", []dimen.DU{10 * dimen.PT, 10 * dimen.PT}, false},
			{"a abcd", []dimen.DU{25 * dimen.PT, 10 * dimen.PT}, true},
			{"a ab\u00adcd", []dimen.DU{25 * dimen.PT, 10 * dimen.PT}, true},
		} {
			t.Run(engine+tc.text, func(t *testing.T) {
				policy := &streamingWordPolicy{}
				owner := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: policy})
				text := styled.TextFromString(tc.text)
				para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
				if err != nil {
					t.Fatal(err)
				}
				k, err := owner.EncodeParagraph(t.Context(), para)
				if err != nil {
					t.Fatal(err)
				}
				width := 15 * dimen.PT
				if tc.discretionary {
					width = 25 * dimen.PT
				}
				shape := linebreak.RectangularParShape(width)
				var breaks []int
				if engine == "firstfit" {
					params := *firstfit.DefaultParameters
					params.DiscretionaryProvider = owner
					breaks, err = firstfit.BreakParagraph(k, shape, &params)
				} else {
					params := knuthplass.NewKPDefaultParameters()
					params.DiscretionaryProvider = owner
					if tc.discretionary {
						params.RightSkip = khipu.KnotCore{MaxW: dimen.PT}
						params.EmergencyStretch = 0
					}
					breaks, err = knuthplass.BreakParagraph(k, shape, params)
				}
				if err != nil {
					t.Fatal(err)
				}
				calls := len(policy.requests)
				lines, err := khipu.MaterializeLogicalLinesForTest(owner, k, breaks)
				if err != nil {
					t.Fatal(breaks, err)
				}
				visual, err := khipu.MaterializeVisualLinesForTest(owner, k, breaks)
				if err != nil || !reflect.DeepEqual(lines, visual) {
					t.Fatal("LTR visual integration changed content", visual, err)
				}
				wantWidths := tc.widths
				if engine == "knuthplass" && tc.text == "  aa  bb  " {
					// Current K&P may choose the initial space opportunity as an
					// empty first line. Materialization preserves that decision.
					wantWidths = append([]dimen.DU{0}, wantWidths...)
				}
				if len(lines) != len(wantWidths) || len(policy.requests) != calls {
					t.Fatal(lines, policy.requests)
				}
				var start uint64
				for i, line := range lines {
					if line.ByteStart != start || line.NaturalWidth != wantWidths[i] {
						t.Fatalf("line %d: %+v", i, line)
					}
					start = line.ByteEnd
				}
				if start != uint64(len(tc.text)) {
					t.Fatal("lost source coverage")
				}
				if tc.discretionary {
					choice, ok := k.SelectedDiscretionaryAt(breaks[0])
					if !ok {
						t.Fatal("fixture did not select a discretionary")
					}
					candidate := k.DiscretionariesAt(choice.Source)[0]
					if !reflect.DeepEqual(lines[0].Knots, []khipu.KnotCore{k.KnotByIndex(0), k.KnotByIndex(1), candidate.PreBreak}) ||
						!reflect.DeepEqual(lines[1].Knots, []khipu.KnotCore{candidate.PostBreak, k.KnotByIndex(len(k.Kind) - 1)}) {
						t.Fatal("incorrect selected-fragment materialization", lines)
					}
				}
			})
		}
	}
}
