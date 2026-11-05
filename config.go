package khipu

import "github.com/npillmayer/khipu/contracts"

// Config supplies the collaborators and parameters used by a Khipukamayuq.
// Its zero value selects the existing fixed-width shaper and default parameters,
// with lazy hyphenation disabled. Segmentation setup remains internal.
type Config struct {
	// Streaming opts into EncodeParagraphStreaming only. Nil leaves that entry
	// point unavailable; it does not change EncodeParagraph. The configuration
	// and Defaults.Features are copied; services and the factory are retained.
	Streaming *StreamingConfig
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

// StreamingConfig supplies the independent streaming encoding path. No font,
// resolver or shaper is inferred from the metric Config.Shaper. Defaults are
// independently applied to each original style span. Callers must keep service
// implementations stable and serialize owner operations.
type StreamingConfig struct {
	// NewShaper must return a fresh, exclusively owned stream for every call.
	// A completed or failed stream must never be returned again. The factory
	// must not start shaping; the encoder owns producer/consumer coordination.
	NewShaper     func() (contracts.StreamingShaper, error)
	Resolver      contracts.StyleResolver
	Defaults      contracts.ResolvedRunStyle
	BufferOptions contracts.ShapeBufferOptions
}
