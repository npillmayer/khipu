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
	// Private references used only within one freezing snapshot. Zero
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
