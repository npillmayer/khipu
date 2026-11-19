package khipu

// DiscretionaryCandidate stores one lazily discovered hyphenation alternative
// associated with a textbox knot. Variants are expected to be stable for a given
// textbox within one paragraph instance.
type DiscretionaryCandidate struct {
	// Variant is a nonzero identity unique within the source textbox's candidates.
	Variant uint16
	// PreBreak replaces the source textbox on the ending line; its penalty
	// controls eligibility. PostBreak starts the next line's material.
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
	// Source is the original textbox's zero-based knot index.
	Source int
	// Variant identifies a cached alternative at Source.
	Variant uint16
}

// AddDiscretionaryCandidate appends cand at index and clears any cached empty
// discovery there. It returns false for a nil receiver or invalid knot index.
// Success means appended: kind, variant uniqueness, source split and shaping
// provenance are not validated. Callers supplying metric-only alternatives must
// not assume they can be frozen. Normal clients use the owner's provider.
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

// DiscretionariesAt returns cached alternatives at index without discovery.
// Nil receivers, negative indices and absent map entries return nil. The slice
// shares cache storage; treat it as read-only while the paragraph is in use.
// An empty result does not indicate whether discovery has been attempted.
func (khipu *Khipu) DiscretionariesAt(index int) []DiscretionaryCandidate {
	if khipu == nil || index < 0 {
		return nil
	}
	return khipu.DiscretionaryCandidates[index]
}

// SelectDiscretionary stores choice for the zero-based line-ending breakpoint.
// It returns false for a nil receiver or invalid breakpoint. It does not validate
// choice.Source, choice.Variant, or candidate availability. Breaker-produced
// choices use the ending textbox as Source; successful breaker calls replace
// all prior choices. Freeze validates choices against the complete break list.
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

// SelectedDiscretionaryAt returns the stored choice and whether its map entry
// exists. Nil receivers and negative breakpoints return a zero value and false.
// The returned choice is a value copy; this query does not validate its candidate.
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
// passes/reruns. ExplicitHyphenation preparations additionally memoize complete
// immutable pairs and import them lazily into each binding; candidate/selection
// maps remain binding-local. Keep source and collaborator policy stable; changed
// policy requires a new owner/preparation or encoding.
// Manually supplied candidates take precedence over an earlier empty result.
// Streaming paragraphs support explicit U+00AD and automatic requests for one
// complete eligible source segment, never metric-shaped substitutes.
// Configure the owner as the linebreaker's provider to enable lazy discovery.
// This synchronous query has no context argument. Lazy replacement shaping uses
// context.Background(). The returned candidate slice shares the paragraph's cache storage.
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
