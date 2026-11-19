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
	material, discovery, err := owner.EncodeParagraph(para)
	if err != nil {
		panic(err)
	}
	khp, err := khipu.BindParagraph(material, nil)
	if err != nil {
		panic(err)
	}
	shape := linebreak.RectangularParShape(100 * dimen.PT)
	greedy, err := firstfit.BreakParagraph(exampleRequest(khp, discovery, firstfit.DefaultLayout(shape)), nil)
	if err != nil {
		panic(err)
	}
	optimal, err := knuthplass.BreakParagraph(exampleRequest(khp, discovery, knuthplass.DefaultLayout(shape)), nil)
	if err != nil {
		panic(err)
	}
	last, _ := khp.Knot(optimal.Lines()[len(optimal.Lines())-1].End.Knot)
	fmt.Println("firstfit lines:", len(greedy.Lines()))
	fmt.Println("Knuth-Plass lines:", len(optimal.Lines()))
	fmt.Println("source end:", last.Pos, "trailer bytes:", last.Len)
	// Output:
	// firstfit lines: 1
	// Knuth-Plass lines: 1
	// source end: 11 trailer bytes: 0
}

// A small example provider; applications can supply a dictionary-backed one.
type exampleHyphenator struct{}

func ExampleFreeze() {
	text := styled.TextFromString("Hello world")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		panic(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	material, discovery, err := owner.EncodeParagraph(para)
	if err != nil {
		panic(err)
	}
	// Each plan captures its geometry and survives independent later trials.
	input, err := khipu.BindParagraph(material, nil)
	if err != nil {
		panic(err)
	}
	layout := firstfit.DefaultLayout(linebreak.RectangularParShape(100 * dimen.PT))
	breaks, err := firstfit.BreakParagraph(exampleRequest(input, discovery, layout), nil)
	if err != nil {
		panic(err)
	}
	frozen, err := khipu.Freeze(breaks)
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
	material, discovery, err := owner.EncodeParagraph(para)
	if err != nil {
		panic(err)
	}
	// Discovery retains the fixed policy; each trial owns its decisions.
	khp, err := khipu.BindParagraph(material, nil)
	if err != nil {
		panic(err)
	}
	params := &firstfit.Parameters{}
	breaks, err := firstfit.BreakParagraph(exampleRequest(khp, discovery, firstfit.DefaultLayout(linebreak.RectangularParShape(45*dimen.PT))), params)
	if err != nil {
		panic(err)
	}
	chosen := breaks.Lines()[0].End
	ok := chosen.Kind == khipu.DiscretionaryBoundary
	fmt.Println("lines:", len(breaks.Lines()))
	fmt.Println("selected:", ok, "variant:", chosen.Alternative.Variant)
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
	prepared, discovery, err := owner.PrepareParagraph(khipu.ParagraphInput{
		Paragraph: para,
		Atomics:   []contracts.AtomicAnchor{{ID: 1, ByteOffset: 2}},
	}, khipu.PrepareOptions{})
	if err != nil {
		panic(err)
	}
	layout := firstfit.DefaultLayout(linebreak.RectangularParShape(100 * dimen.PT))
	for _, advance := range []dimen.DU{20 * dimen.PT, 30 * dimen.PT} {
		// Reuse shaped text, but give each binding independent working decisions.
		input, err := khipu.BindParagraph(prepared, []khipu.AtomicAdvance{{ID: 1, Advance: advance}})
		if err != nil {
			panic(err)
		}
		breaks, err := firstfit.BreakParagraph(exampleRequest(input, discovery, layout), nil)
		if err != nil {
			panic(err)
		}
		frozen, err := khipu.Freeze(breaks)
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

func ExampleMeasureIntrinsic() {
	text := styled.TextFromString("Hello world")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		panic(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	prepared, discovery, err := owner.PrepareParagraph(khipu.ParagraphInput{Paragraph: para}, khipu.PrepareOptions{})
	if err != nil {
		panic(err)
	}
	// No target geometry or binding is needed. This input has no atomic ranges.
	widths, err := khipu.MeasureIntrinsic(prepared, discovery, khipu.MeasureRequest{})
	if err != nil {
		panic(err)
	}
	fmt.Println("min-content:", int64(widths.MinContent/dimen.PT), "pt")
	fmt.Println("max-content:", int64(widths.MaxContent/dimen.PT), "pt")
	// Output:
	// min-content: 25 pt
	// max-content: 55 pt
}

func exampleRequest(b *khipu.AtomicBinding, d *khipu.Discovery, l khipu.Params) khipu.BreakRequest {
	return khipu.BreakRequest{Binding: b, Discovery: d, Geometry: khipu.GeometryRequest{Shape: l.LineShape, LeftSkip: l.LeftSkip, RightSkip: l.RightSkip}}
}
