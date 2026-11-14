package khipu

// DiscretionaryCandidate stores one lazily discovered hyphenation alternative
// associated with a textbox knot. Variants are expected to be stable for a given
// textbox within one paragraph instance.
type DiscretionaryCandidate struct {
	Variant   uint16
	PreBreak  KnotCore
	PostBreak KnotCore
	// SourceSplit, when Valid, is relative to the original paragraph in UTF-8
	// bytes (unlike HyphenationFragment.SourceSplit, which is word-relative).
	// It is never derived from PreBreak.Len or replacement text length. Manually
	// supplied candidates may leave it unknown; encoding supplies it.
	SourceSplit SourceSplit
	// Owner-local optional references into the khipu's shaping arena. Zero
	// denotes a metric-only candidate. Breakers copy candidates unchanged and
	// continue to read KnotCore metrics; glyph access is not part of their API.
	preBox, postBox shapedBoxID
}

// DiscretionarySelection records which discretionary candidate was selected
// for a selected break decision.
type DiscretionarySelection struct {
	Source  int
	Variant uint16
}

func defaultFlagsForKind(kind KnotType) KnotFlags {
	switch kind {
	case KTGlue, KTKern:
		return KFDiscardable
	default:
		return 0
	}
}

func (khipu *Khipu) flagsAt(index int) KnotFlags {
	if khipu == nil || index < 0 || index >= len(khipu.Flags) {
		return 0
	}
	return khipu.Flags[index]
}

func (k KnotCore) IsDiscardable() bool {
	return k.Flags&KFDiscardable != 0
}

func (khipu *Khipu) SetKnotFlags(index int, flags KnotFlags) bool {
	if khipu == nil || index < 0 || index >= len(khipu.Kind) {
		return false
	}
	if len(khipu.Flags) < len(khipu.Kind) {
		missing := len(khipu.Kind) - len(khipu.Flags)
		khipu.Flags = append(khipu.Flags, make([]KnotFlags, missing)...)
	}
	khipu.Flags[index] = flags
	return true
}

func (khipu *Khipu) AddDiscretionaryCandidate(index int, cand DiscretionaryCandidate) bool {
	if khipu == nil || index < 0 || index >= len(khipu.Kind) {
		return false
	}
	if khipu.DiscretionaryCandidates == nil {
		khipu.DiscretionaryCandidates = make(map[int][]DiscretionaryCandidate)
	}
	khipu.DiscretionaryCandidates[index] = append(khipu.DiscretionaryCandidates[index], cand)
	delete(khipu.discretionaryEmpty, index)
	return true
}

func (khipu *Khipu) DiscretionariesAt(index int) []DiscretionaryCandidate {
	if khipu == nil || index < 0 {
		return nil
	}
	return khipu.DiscretionaryCandidates[index]
}

func (khipu *Khipu) SelectDiscretionary(breakpoint int, choice DiscretionarySelection) bool {
	if khipu == nil || breakpoint < 0 || breakpoint >= len(khipu.Kind) {
		return false
	}
	if khipu.SelectedDiscretionaries == nil {
		khipu.SelectedDiscretionaries = make(map[int]DiscretionarySelection)
	}
	khipu.SelectedDiscretionaries[breakpoint] = choice
	return true
}

func (khipu *Khipu) SelectedDiscretionaryAt(breakpoint int) (DiscretionarySelection, bool) {
	if khipu == nil || breakpoint < 0 {
		return DiscretionarySelection{}, false
	}
	choice, ok := khipu.SelectedDiscretionaries[breakpoint]
	return choice, ok
}

// DiscretionaryCandidates returns the currently known discretionary candidates
// for one textbox knot. If no candidates are cached yet, Khipukamayuq will
// consult its Hyphenator, shape the returned fragments, and cache the resulting
// discretionary candidates on the Khipu.
// For owned, encoded paragraphs, successful empty results are also remembered,
// including alternatives filtered out during preparation. Errors remain
// retryable. Both caches live with the paragraph and are reused across breaker
// passes/reruns, not shared across paragraphs or owners. Keep the source and
// collaborator policy stable; changed policy requires a new owner/encoding.
// Manually supplied candidates take precedence over an earlier empty result.
// Streaming paragraphs support explicit U+00AD and automatic requests for one
// complete eligible source segment, never metric-shaped substitutes.
// Configure the owner as the linebreaker's provider to enable lazy discovery.
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
	if khp.Kind[at] == KTAtomic || (khp.hyphenationPolicy && khp.hyphenation == NoHyphenation) {
		return nil, nil
	}
	if cands := khp.DiscretionariesAt(at); len(cands) > 0 {
		return cands, nil
	}
	if _, empty := khp.discretionaryEmpty[at]; empty {
		return nil, nil
	}
	if kq.env.hyphenator == nil {
		return nil, nil
	}
	if khp.Kind[at] != KTTextBox {
		return nil, nil
	}
	cands, err := kq.streamingDiscretionaryCandidates(khp, at)
	// Ownerless/manual Khipus may be queried through different providers. Do
	// not let one provider's refusal suppress a future provider's discovery.
	if err == nil && len(cands) == 0 && khp.owner == kq && khp.paragraph != nil {
		if khp.discretionaryEmpty == nil {
			khp.discretionaryEmpty = make(map[int]struct{})
		}
		khp.discretionaryEmpty[at] = struct{}{}
	}
	return cands, err
}

// discretionaryPenalty resolves the effective penalty of a discretionary
// fragment. A fragment may carry a script-specific penalty supplied by the
// Hyphenator; otherwise we fall back to Khipukamayuq's current parameter set.
func discretionaryPenalty(frag HyphenationFragment, params *Params) Penalty {
	if frag.Penalty != 0 {
		return frag.Penalty
	}
	if params == nil {
		return 0
	}
	return Penalty(params.Hyphenpenalty)
}
