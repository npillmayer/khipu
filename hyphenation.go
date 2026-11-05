package khipu

// HyphenationFragment is one hyphenation alternative for a textbox.
// The Khipukamayuq will shape these fragments and turn them into
// discretionary candidates stored on a Khipu.
type HyphenationFragment struct {
	Variant   uint16
	PreBreak  string
	PostBreak string
	Penalty   Penalty
	// SourceSplit identifies the original input boundary independently of the
	// replacement strings. Coordinates are UTF-8 bytes relative to the word
	// passed to Hyphenator.Discretionaries, not to PreBreak or PostBreak.
	// Legacy metric callers may omit it; streaming preparation requires Valid.
	SourceSplit SourceSplit
}

// SourceSplit is an explicitly optional original-text boundary. Valid=false
// means unknown; ByteOffset=0 with Valid=true is a known boundary at input start.
// Range and UTF-8 validation require the associated source. This is provenance,
// not an assertion that the boundary is linguistically or typographically safe.
type SourceSplit struct {
	ByteOffset uint64
	Valid      bool
}

// Hyphenator discovers hyphenation alternatives for one textbox string.
// It returns fragment strings; shaping and final discretionary construction
// remain the responsibility of Khipukamayuq.
// The params argument belongs to Khipukamayuq and must not be modified.
// Implementations needing to retain parameters should copy the value.
type Hyphenator interface {
	Discretionaries(word string, params *Params) ([]HyphenationFragment, error)
}
