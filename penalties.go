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

// normPenalties preserves the current SOA penalty mapping during extraction.
// Mapping UAX sentinels to the linebreakers' sentinel range is separate work.
func normPenalties(p penalties) Penalty {
	if p.p1 == 0 {
		return 0
	}
	var p1 int8
	if p.p1 < 0 { // merit
		p1 = max(-100, int8(p.p1))
	} else {
		p1 = min(100, int8(p.p1))
	}
	var p2 int8
	if p.p1 < 0 { // merit
		p2 = max(-100, int8(p.p2))
	} else {
		p2 = min(100, int8(p.p2))
	}
	P := int16(p1) + int16(p2)
	if P < 0 { // merit
		return Penalty(max(-100, P))
	}
	return Penalty(min(100, P))
}
