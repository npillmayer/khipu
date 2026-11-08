package khipu

import (
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

// Config supplies the collaborators and parameters used by a Khipukamayuq.
// Its zero value enables synthetic streaming encoding and default parameters,
// with lazy hyphenation disabled.
type Config struct {
	// Streaming configures EncodeParagraph.
	// Nil selects a fresh synthetic stream per encoding/reshaping call, a default
	// resolver for unstyled text, a synthetic font at 10 pt, and elastic spaces.
	// This is a deterministic test/simple-layout backend, not real font shaping.
	// A non-nil configuration is used in full: missing services and invalid
	// defaults cause encoding errors, not silent fallback to synthetic shaping.
	// The configuration and Defaults.Features are copied; services and the factory
	// are retained.
	Streaming *StreamingConfig

	// Hyphenator supplies lazy discretionary alternatives. Nil disables discovery;
	// candidates already cached on a Khipu remain available.
	// The supplied instance is retained, not copied.
	Hyphenator Hyphenator

	// Params is copied at construction. Later changes to the caller's value do
	// not affect the Khipukamayuq. Nil selects the existing default parameters.
	// A non-nil value is used in full: zero fields are not filled with defaults.
	// LineShape is a shared service, not deep-copied. Encoding does not use
	// paragraph geometry; pass the resolved Params to breaking/freezing explicitly.
	Params *Params

	// ParFillSkip supplies the final knot's W, MinW, and MaxW. Nil selects
	// zero natural/minimum width and dimen.Fill maximum width. A non-nil value
	// is copied, including zero widths. EncodeParagraph sets all other fields
	// and rejects widths outside MinW <= W <= MaxW.
	ParFillSkip *KnotCore
}

// StreamingConfig supplies the streaming encoding path. Defaults are
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

// defaultStreamingConfig provides a complete synthetic backend, not defaults to
// merge into an application backend. Keep font/run defaults independent of
// legacy Params.Language/Script; applications resolve those through style data.
func defaultStreamingConfig() *StreamingConfig {
	return &StreamingConfig{
		NewShaper: func() (contracts.StreamingShaper, error) {
			return helpers.NewSyntheticStreamingShaper(0)
		},
		Resolver: helpers.DefaultStyleResolver{},
		Defaults: contracts.ResolvedRunStyle{
			Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT,
			Spacing: contracts.SpacingElastic,
		},
	}
}
