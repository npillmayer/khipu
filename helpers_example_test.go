package khipu_test

import (
	"context"
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/uax/bidi"
)

func ExampleConfig_syntheticFontSize() {
	text := styled.TextFromString("aéz")
	// Synthetic cells advance half an em: a 14 pt em yields 7 pt cells.
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		panic(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: &khipu.StreamingConfig{
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(0) },
		Resolver:  helpers.DefaultStyleResolver{},
		Defaults:  contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 14 * dimen.PT},
	}})
	paragraph, err := owner.EncodeParagraph(context.Background(), para)
	if err != nil {
		panic(err)
	}
	fmt.Println("text width in pt:", float64(paragraph.KnotByIndex(0).W)/float64(dimen.PT))
	// Output:
	// text width in pt: 21
}
