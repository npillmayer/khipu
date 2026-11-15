package khipu

import "github.com/npillmayer/uax"

// Penalty is the cost of breaking after a SOA knot.
// Values >= MaxPenalty prohibit a break; values <= MaxMerit force one.
// Values between the thresholds permit a break, with zero neutral, positive
// costs discouraging and negative costs encouraging it under a scoring breaker.
// A penalty is an input to scoring, not an accumulated demerit score.
type Penalty int16

// MaxPenalty is the inclusive threshold for a prohibited break.
const MaxPenalty = 10000

// MaxMerit is the inclusive threshold for a forced break.
const MaxMerit = -10000

// penaltyFromUAX translates a finalized Unicode boundary classification into a
// layout penalty. Finite UAX rule weights establish allowed opportunities, not
// typographic preferences: squaring negative weights in Knuth-Plass would reward
// extra lines. Ordinary allowed breaks therefore have zero cost. Translation
// must follow accumulation, since later rules can revise earlier boundaries.
// This policy applies only to source boundaries, never to client-assigned knot
// penalties or shaped discretionary penalties, which may express preferences.
func penaltyFromUAX(p int64) Penalty {
	switch {
	case p <= -int64(uax.InfinitePenalty):
		return MaxMerit
	case p >= int64(uax.InfinitePenalty):
		return MaxPenalty
	default:
		return 0
	}
}
