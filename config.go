package khipu

import (
	"slices"

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
	// complete alternatives are memoized separately in Discovery.
	// The supplied instance is retained, not copied.
	// Keep its policy stable while encoded paragraphs are in use: both positive
	// and successful empty discovery results are cached for each paragraph.
	// To change policy, create a new owner and encode again.
	Hyphenator Hyphenator

	// Params is copied at construction. Later changes to the caller's value do
	// not affect the Khipukamayuq. Nil selects the existing default parameters.
	// A non-nil value is used in full: zero fields are not filled with defaults.
	// LineShape is a shared service, not deep-copied. Encoding does not use
	// paragraph geometry; supply geometry in BreakRequest when breaking.
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
	// On error, the factory owns cleanup and must return no stream. Successful
	// streams undergo CheckFace preflight and are finished even if never shaped.
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

// Khipukamayuq is a configured preparation executor, not material identity.
// Construct it with NewKhipukamayuq. PrepareParagraph and EncodeParagraph return
// immutable material and its independent Discovery; the executor need not survive
// those calls. Serialize construction calls sharing mutable collaborators. Keep
// borrowed source, styles and font services stable for all derived results.
type Khipukamayuq struct {
	env typEnv
}

// NewKhipukamayuq creates an owner for SOA paragraphs using config.
// It copies parameters and retains collaborator instances as described by Config.
// A Khipukamayuq must not be used concurrently.
func NewKhipukamayuq(config Config) *Khipukamayuq {
	env := typEnv{
		hyphenator:  config.Hyphenator,
		parFillSkip: KnotCore{MaxW: dimen.Fill},
	}
	if config.Streaming != nil {
		streaming := *config.Streaming
		streaming.Defaults.Features = slices.Clone(streaming.Defaults.Features)
		env.streaming = &streaming
	} else {
		env.streaming = defaultStreamingConfig()
	}
	if config.ParFillSkip != nil {
		env.parFillSkip = *config.ParFillSkip
	}
	if config.Params == nil {
		env.params = newParameters()
	} else {
		params := *config.Params
		env.params = &params
	}
	return &Khipukamayuq{
		env: env,
	}
}
