package khipu

import "github.com/npillmayer/uax/bidi"

// typEnv holds the internal encoding environment. Public SOA callers configure
// its collaborators through Config; legacy encoders still use its pipeline and
// resolved bidi levels directly.
type typEnv struct {
	shaper Shaper
	// hyphenator is consulted lazily for textbox-specific discretionaries.
	// The linebreaker reaches it through Khipukamayuq.DiscretionaryCandidates.
	hyphenator  Hyphenator
	params      *Params
	parFillSkip KnotCore
	pipeline    *TypesettingPipeline // legacy encoding only
	// levels is used only by the legacy text encoder. SOA encoding obtains
	// bidi/style runs from the styled paragraph instead.
	levels *bidi.ResolvedLevels
}
