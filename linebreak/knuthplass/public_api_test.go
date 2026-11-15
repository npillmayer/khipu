package knuthplass_test

import (
	"fmt"
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
)

func ExampleParameters() {
	// These typed values can come from flags or a decoded application config.
	var tolerance, pretolerance, linePenalty, firstHyphen, finalHyphen int64 = 500, 100, 10, 25, 50
	params := knuthplass.NewDefaultParameters()
	params.Tolerance = knuthplass.Merits(tolerance)
	params.PreTolerance = knuthplass.Merits(pretolerance)
	params.LinePenalty = knuthplass.Merits(linePenalty)
	params.FirstHyphenDemerits = knuthplass.Merits(firstHyphen)
	params.FinalHyphenDemerits = knuthplass.Merits(finalHyphen)
	fmt.Println(params.Tolerance, params.PreTolerance, params.LinePenalty,
		params.FirstHyphenDemerits, params.FinalHyphenDemerits)
	// Output: 500 100 10 25 50
}

// Keep the old helper signatures and typed/untyped constants usable by clients.
// These are compatibility checks, not recommendations for new client code.
func TestLegacyExportsRemainUsable(t *testing.T) {
	var plain func(int) knuthplass.BreakRef = knuthplass.PlainBreak
	var index func(knuthplass.BreakRef) int = knuthplass.BreakRef.Kinx
	br := plain(7)
	if index(br) != 7 || br.IsDiscretionary() || br.String() != "k-7" {
		t.Fatalf("legacy plain break changed: %+v", br)
	}
	variant := knuthplass.BreakRef{At: 7, Variant: 2}
	if !variant.IsDiscretionary() || variant.String() != "k-7[v:2]" {
		t.Fatalf("legacy discretionary break changed: %+v", variant)
	}
	if knuthplass.NoBreakRef != (knuthplass.BreakRef{At: -1}) {
		t.Fatal("legacy sentinel initial value changed")
	}
	if got := [6]uint8{
		uint8(firstfit.LICContent), uint8(firstfit.LICTrimDiscardable), uint8(firstfit.LICRetainedNeutral),
		uint8(knuthplass.LICContent), uint8(knuthplass.LICTrimDiscardable), uint8(knuthplass.LICRetainedNeutral),
	}; got != [6]uint8{0, 1, 2, 0, 1, 2} {
		t.Fatalf("legacy classification values changed: %v", got)
	}
	var capacity int64 = khipu.DefaultInitialKhipuCapa
	if capacity != 20 {
		t.Fatalf("legacy capacity changed: %d", capacity)
	}
}
