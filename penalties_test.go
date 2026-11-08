package khipu

import (
	"math"
	"testing"

	"github.com/npillmayer/uax"
)

func TestPenaltyFromUAX(t *testing.T) {
	limit := int64(uax.InfinitePenalty)
	for _, tc := range []struct {
		input int64
		want  Penalty
	}{
		{math.MinInt64, MaxMerit}, {-limit - 1, MaxMerit}, {-limit, MaxMerit},
		{-limit + 1, 0}, {-417, 0}, {-240, 0}, {-29, 0}, {0, 0}, {1, 0}, {550, 0}, {limit - 1, 0},
		{limit, MaxPenalty}, {limit + 1, MaxPenalty}, {math.MaxInt64, MaxPenalty},
	} {
		if got := penaltyFromUAX(tc.input); got != tc.want {
			t.Errorf("penaltyFromUAX(%d) = %d, want %d", tc.input, got, tc.want)
		}
	}
}
