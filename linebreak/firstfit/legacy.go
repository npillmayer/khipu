package firstfit

// LICContent is the legacy internal classification for visible content.
//
// Deprecated: use khipu.KnotCore.IsContent to inspect a knot.
const LICContent = licContent

// LICTrimDiscardable is the legacy internal classification for edge trimming.
//
// Deprecated: use khipu.KnotCore.IsDiscardable to inspect a knot.
const LICTrimDiscardable = licTrimDiscardable

// LICRetainedNeutral is the legacy internal classification for retained material
// that is neither discardable nor visible content, such as paragraph fill.
//
// Deprecated: inspect khipu.KnotCore.IsDiscardable and IsContent instead.
const LICRetainedNeutral = licRetainedNeutral
