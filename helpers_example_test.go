package khipu_test

import (
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/uax/bidi"
)

func ExampleConfig_fixedWidthShaper() {
	shaper, err := helpers.NewFixedWidthShaper(7 * dimen.PT)
	if err != nil {
		panic(err)
	}
	text := styled.TextFromString("aéz")
	// The existing encoder accepts this public helper through Config.Shaper.
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		panic(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{Shaper: shaper})
	paragraph, err := owner.EncodeParagraph(para, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println("text width in pt:", float64(paragraph.KnotByIndex(0).W)/float64(dimen.PT))
	// Output:
	// text width in pt: 21
}
