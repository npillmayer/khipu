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
	// Automatic preparation requires Valid; explicit-marker requests ignore it.
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
// Automatic streaming requests receive a local copy with Language, Script and
// BidiDir taken from the textbox's retained shaping context. Returned fragments
// must have valid interior SourceSplit positions; replacement lengths are never
// used to infer them. Nonzero Variant IDs must be unique; zero IDs are assigned
// unused positive IDs. Empty replacement halves are skipped after validation.
//
// The explicit-marker preparation stage also uses this interface: a request
// containing exactly U+00AD asks for break material, not word segmentation.
// Returned PreBreak is appended to the left text and PostBreak prepended to the
// right text before shaping both complete fragments. Empty material is valid;
// returning no alternatives declines the request. No ASCII hyphen is inferred.
// For this request SourceSplit and Variant are not used: the owner supplies the
// original marker positions and assigns unique variants across those positions.
// Implementations supporting only automatic discovery may decline this request.
// Streaming lazy lookup uses this convention; paragraph encoding does not query
// the policy.
type Hyphenator interface {
	Discretionaries(word string, params *Params) ([]HyphenationFragment, error)
}
