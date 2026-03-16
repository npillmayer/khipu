package khipu

// DiscretionaryCandidates returns the currently known discretionary candidates
// for one textbox knot. At this stage it serves only cached candidates already
// stored in the Khipu. Future versions may populate that cache lazily.
func (kq *Khipukamayuq) DiscretionaryCandidates(khp *Khipu, at int) ([]DiscretionaryCandidate, error) {
	if kq == nil {
		return nil, ErrVoidKhipukamayuq
	}
	if khp == nil || at < 0 || at >= len(khp.Kind) {
		return nil, ErrIllegalArguments
	}
	if khp.owner != nil && khp.owner != kq {
		return nil, ErrIllegalArguments
	}
	return khp.DiscretionariesAt(at), nil
}
