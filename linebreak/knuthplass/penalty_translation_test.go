package knuthplass

import (
	"testing"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

// Real encoding must not introduce rewards for ordinary space breaks. Check
// both the encoder/breaker contract and the winning path, rather than pinning a
// line count that may legitimately change with future geometry refinements.
func TestEncodedOrdinaryBreaksHaveNoReward(t *testing.T) {
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	khp, err := owner.EncodeParagraph(t.Context(), loremParagraph(t, 250))
	if err != nil {
		t.Fatal(err)
	}
	allowed := 0
	for i, p := range khp.Penalty[:len(khp.Penalty)-1] {
		if p < InfinityDemerits {
			allowed++
			if p != 0 {
				t.Fatalf("ordinary break %d has penalty %d, want zero", i, p)
			}
		}
	}
	if allowed == 0 {
		t.Fatal("fixture has no ordinary opportunities")
	}
	for _, width := range []dimen.DU{150 * dimen.PT, 300 * dimen.PT} {
		result, err := selectParagraphBreaks(khp, DefaultLayout(linebreak.RectangularParShape(width)), nil)
		if err != nil || !result.complete {
			t.Fatalf("width %v: incomplete result: %v", width, err)
		}
		if result.quality.totalCost < 0 {
			t.Fatalf("width %v: ordinary path earned a reward: %d", width, result.quality.totalCost)
		}
	}
}

func TestIntentionalNegativePenaltyStillRewardsBreak(t *testing.T) {
	params := NewDefaultParameters()
	if got := calcDemerits(0, -29, params); got != -741 {
		t.Fatalf("explicit ranking penalty was neutralized: got %d, want -741", got)
	}
}
