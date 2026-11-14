package linebreak

import (
	"errors"
	"testing"

	"github.com/npillmayer/khipu"
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

func TestWidthSummarySignedRecovery(t *testing.T) {
	wide := WSS{W: 1500000000, Min: 1500000000, Max: 1500000000}
	got := wide.Add(wide).Subtract(wide)
	if got != wide || got.Err() != nil {
		t.Fatal("positive prefix lost", got)
	}
	negative := WSS{}.Subtract(wide)
	got = negative.Add(negative).Add(wide)
	if got != negative || got.Err() != nil {
		t.Fatal("negative prefix lost", got)
	}
	got = wide
	for i := 0; i < 40; i++ {
		got = got.Add(got)
	}
	if !errors.Is(got.Err(), khipu.ErrIllegalArguments) {
		t.Fatal("overflow not reported", got)
	}
	if !errors.Is(got.Subtract(got).Err(), khipu.ErrIllegalArguments) {
		t.Fatal("overflow cleared by subtraction")
	}
}
