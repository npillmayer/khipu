package khipu

import (
	"math"
	"testing"
)

// Penalty normalization is shared with streaming boundary preparation and must
// survive deletion of the old metric collector and its private tests.
func TestNormalizeEncodingPenalties(t *testing.T) {
	for _, tc := range []struct {
		input int64
		want  Penalty
	}{
		{0, 0}, {550, 550}, {-240, -240},
		{9999, 9999}, {-9999, -9999},
		{10000, MaxPenalty}, {-10000, MaxMerit},
		{18000, MaxPenalty}, {-18000, MaxMerit},
		{math.MaxInt64, MaxPenalty}, {math.MinInt64, MaxMerit},
	} {
		if got := normPenalty(tc.input); got != tc.want {
			t.Errorf("%d: got %d want %d", tc.input, got, tc.want)
		}
	}
}
