package knuthplass

import "github.com/npillmayer/khipu"

// DiscretionaryProvider supplies hyphenation alternatives for one textbox knot.
// Ownership of candidate discovery and caching stays with the provider.
type DiscretionaryProvider interface {
	DiscretionaryCandidates(khp *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error)
}

func (kp *linebreaker) discretionaryCandidates(khp *khipu.Khipu, at kinx) ([]khipu.DiscretionaryCandidate, error) {
	if khp == nil || at < 0 || at >= len(khp.Kind) {
		return nil, khipu.ErrIllegalArguments
	}
	if kp != nil && kp.params != nil && kp.params.DiscretionaryProvider != nil {
		return kp.params.DiscretionaryProvider.DiscretionaryCandidates(khp, int(at))
	}
	return khp.DiscretionariesAt(int(at)), nil
}
