package khipu_test

import (
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

func ExampleKhipukamayuq_EncodeParagraph() {
	text := styled.TextFromString("Hello world")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		panic(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	khp, err := owner.EncodeParagraph(para)
	if err != nil {
		panic(err)
	}
	shape := linebreak.RectangularParShape(100 * dimen.PT)
	greedy, err := firstfit.BreakParagraph(khp, firstfit.DefaultLayout(shape), nil)
	if err != nil {
		panic(err)
	}
	optimal, err := knuthplass.BreakParagraph(khp, knuthplass.DefaultLayout(shape), nil)
	if err != nil {
		panic(err)
	}
	last := khp.KnotByIndex(optimal[len(optimal)-1])
	fmt.Println("firstfit lines:", len(greedy))
	fmt.Println("Knuth-Plass lines:", len(optimal))
	fmt.Println("source end:", last.Pos, "trailer bytes:", last.Len)
	// Output:
	// firstfit lines: 1
	// Knuth-Plass lines: 1
	// source end: 11 trailer bytes: 0
}

// A small example provider; applications can supply a dictionary-backed one.
type exampleHyphenator struct{}

func ExampleKhipukamayuq_Freeze() {
	text := styled.TextFromString("Hello world")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		panic(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	input, err := owner.EncodeParagraph(para)
	if err != nil {
		panic(err)
	}
	// Resolve geometry once; freeze the chosen decisions before another breaker
	// run replaces the paragraph's selected discretionaries.
	layout := firstfit.DefaultLayout(linebreak.RectangularParShape(100 * dimen.PT))
	breaks, err := firstfit.BreakParagraph(input, layout, nil)
	if err != nil {
		panic(err)
	}
	frozen, err := owner.Freeze(input, breaks, layout)
	if err != nil {
		panic(err)
	}
	line, _ := frozen.Line(0)
	fmt.Println("lines:", frozen.LineCount())
	fmt.Println("target reached:", line.Width == line.TargetWidth)
	fmt.Println("source bytes:", line.Source.End-line.Source.Start)
	// Output:
	// lines: 1
	// target reached: true
	// source bytes: 11
}

func (exampleHyphenator) Discretionaries(word string, _ *khipu.Params) ([]khipu.HyphenationFragment, error) {
	if word != "breaking" {
		return nil, nil
	}
	return []khipu.HyphenationFragment{{Variant: 1, PreBreak: "break-", PostBreak: "ing", SourceSplit: khipu.SourceSplit{ByteOffset: 5, Valid: true}}}, nil
}

func ExampleConfig_hyphenator() {
	text := styled.TextFromString("xx breaking")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		panic(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{Hyphenator: exampleHyphenator{}})
	khp, err := owner.EncodeParagraph(para)
	if err != nil {
		panic(err)
	}
	// Wire the same owner into the breaker to permit lazy discovery.
	params := &firstfit.Parameters{DiscretionaryProvider: owner}
	breaks, err := firstfit.BreakParagraph(khp, firstfit.DefaultLayout(linebreak.RectangularParShape(45*dimen.PT)), params)
	if err != nil {
		panic(err)
	}
	chosen, ok := khp.SelectedDiscretionaryAt(breaks[0])
	fmt.Println("lines:", len(breaks))
	fmt.Println("selected:", ok, "variant:", chosen.Variant)
	// Output:
	// lines: 2
	// selected: true variant: 1
}

func ExampleKhipukamayuq_PrepareParagraph() {
	text := styled.TextFromString("A \uFFFC B")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		panic(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	prepared, err := owner.PrepareParagraph(khipu.ParagraphInput{
		Paragraph: para,
		Atomics:   []contracts.AtomicAnchor{{ID: 1, ByteOffset: 2}},
	}, khipu.PrepareOptions{})
	if err != nil {
		panic(err)
	}
	layout := firstfit.DefaultLayout(linebreak.RectangularParShape(100 * dimen.PT))
	for _, advance := range []dimen.DU{20 * dimen.PT, 30 * dimen.PT} {
		// Reuse shaped text, but give each binding independent working decisions.
		input, err := owner.BindParagraph(prepared, []khipu.AtomicAdvance{{ID: 1, Advance: advance}})
		if err != nil {
			panic(err)
		}
		breaks, err := firstfit.BreakParagraph(input, layout, nil)
		if err != nil {
			panic(err)
		}
		frozen, err := owner.Freeze(input, breaks, layout)
		if err != nil {
			panic(err)
		}
		for i := 0; i < frozen.KnotCount(); i++ {
			if id, ok := frozen.Atomic(i); ok {
				knot, _ := frozen.Knot(i)
				fmt.Println("atomic", id, "advance:", int64(knot.W/dimen.PT), "pt")
			}
		}
	}
	// Output:
	// atomic 1 advance: 20 pt
	// atomic 1 advance: 30 pt
}

func ExampleKhipukamayuq_MeasureIntrinsic() {
	text := styled.TextFromString("Hello world")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		panic(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	prepared, err := owner.PrepareParagraph(khipu.ParagraphInput{Paragraph: para}, khipu.PrepareOptions{})
	if err != nil {
		panic(err)
	}
	// No target geometry or binding is needed. This input has no atomic ranges.
	widths, err := owner.MeasureIntrinsic(prepared, khipu.MeasureRequest{})
	if err != nil {
		panic(err)
	}
	fmt.Println("min-content:", int64(widths.MinContent/dimen.PT), "pt")
	fmt.Println("max-content:", int64(widths.MaxContent/dimen.PT), "pt")
	// Output:
	// min-content: 25 pt
	// max-content: 55 pt
}
