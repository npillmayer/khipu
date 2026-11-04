package linebreak

import (
	"testing"

	"github.com/npillmayer/khipu/dimen"
)

func TestWidthSummaryWithParagraphFill(t *testing.T) {
	text := WSS{W: 40 * dimen.PT, Min: 40 * dimen.PT, Max: 40 * dimen.PT}
	fill := WSS{Max: dimen.Fill}
	got := text.Add(fill).Add(WSS{Max: 50 * dimen.PT})
	if got.W != text.W || got.Min != text.Min || got.Max != dimen.Infinity {
		t.Fatalf("fill overflowed width summary: %+v", got)
	}
	got = got.Subtract(WSS{W: 5 * dimen.PT, Min: 5 * dimen.PT, Max: 5 * dimen.PT})
	if got.Max < 1000*dimen.PT {
		t.Fatalf("trimming lost final-line stretch: %+v", got)
	}
}
