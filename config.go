package khipu

import "github.com/npillmayer/khipu/contracts"

// Config supplies the collaborators and parameters used by a Khipukamayuq.
// Its zero value selects the existing fixed-width shaper and default parameters,
// with lazy hyphenation disabled. Segmentation setup remains internal.
type Config struct {
	// Shaper supplies cluster metrics for paragraph and discretionary encoding.
	// Nil selects the built-in fixed-width shaper.
	// The supplied instance is retained, not copied. Khipukamayuq changes its
	// source and style, so callers must not use it concurrently with encoding
	// or discretionary lookup, including through another Khipukamayuq.
	Shaper contracts.Shaper

	// Hyphenator supplies lazy discretionary alternatives. Nil disables discovery;
	// candidates already cached on a Khipu remain available.
	// The supplied instance is retained, not copied.
	Hyphenator Hyphenator

	// Params is copied at construction. Later changes to the caller's value do
	// not affect the Khipukamayuq. Nil selects the existing default parameters.
	// A non-nil value is used in full: zero fields are not filled with defaults.
	Params *Params

	// ParFillSkip supplies the final knot's W, MinW, and MaxW. Nil selects
	// zero natural/minimum width and dimen.Fill maximum width. A non-nil value
	// is copied, including zero widths. EncodeParagraph sets all other fields
	// and rejects widths outside MinW <= W <= MaxW.
	ParFillSkip *KnotCore
}
