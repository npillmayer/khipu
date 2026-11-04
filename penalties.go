package khipu

import "github.com/npillmayer/uax"

// Penalty is the cost of breaking after a SOA knot.
type Penalty int16

const MaxPenalty = 10000
const MaxMerit = -10000

// penalties carries primary and secondary segmenter penalties before encoding.
// Both the legacy and SOA encoders consume this pair.
type penalties struct {
	p1, p2 int
}

func penlty(p1, p2 int) penalties {
	return penalties{p1, p2}
}

// canWrapLine and breaksAtSpace are used by the legacy encoder.
func (p penalties) canWrapLine() bool {
	return p.p1 < uax.InfinitePenalty
}

func (p penalties) breaksAtSpace() bool {
	return p.p2 < uax.InfinitePenalty
}

// normPenalties preserves mandatory/prohibited primary boundaries and combines
// finite primary and secondary costs before narrowing to the stored type.
func normPenalties(p penalties) Penalty {
	if p.p1 >= uax.InfinitePenalty {
		return MaxPenalty
	}
	if p.p1 <= uax.InfiniteMerits {
		return MaxMerit
	}
	if p.p2 >= uax.InfinitePenalty {
		return MaxPenalty
	}
	if p.p2 <= uax.InfiniteMerits {
		return MaxMerit
	}
	return Penalty(max(MaxMerit, min(MaxPenalty, p.p1+p.p2)))
}
