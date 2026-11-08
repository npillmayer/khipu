package khipu_test

import (
	"context"
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
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
	khp, err := owner.EncodeParagraph(context.Background(), para)
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
	khp, err := owner.EncodeParagraph(context.Background(), para)
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
