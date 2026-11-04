package khipu

// Penalty is the cost of breaking after a SOA knot.
type Penalty int16

const MaxPenalty = 10000
const MaxMerit = -10000

// normPenalty bounds accumulated UAX#14 contributions before narrowing them to
// the stored type. Mandatory and prohibited boundaries use the same sentinels
// as the linebreakers.
func normPenalty(p int64) Penalty {
	return Penalty(max(int64(MaxMerit), min(int64(MaxPenalty), p)))
}
