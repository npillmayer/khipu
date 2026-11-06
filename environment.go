package khipu

// typEnv holds the internal encoding environment. Callers configure its
// collaborators through Config; paragraphs supply their resolved bidi/style runs.
type typEnv struct {
	streaming *StreamingConfig
	// hyphenator is consulted lazily for textbox-specific discretionaries.
	// The linebreaker reaches it through Khipukamayuq.DiscretionaryCandidates.
	hyphenator  Hyphenator
	params      *Params
	parFillSkip KnotCore
}
