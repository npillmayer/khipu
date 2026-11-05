// Package helpers provides deterministic implementations of khipu's service
// contracts for tests, examples, and simple client setups. It depends on
// contracts, not the root paragraph engine, and requires no installed fonts.
//
// FixedWidthShaper provides configurable DU metrics for khipu.Config.Shaper.
// SyntheticFont, DefaultStyleResolver, and SyntheticStreamingShaper can be
// combined for standalone streaming tests. They do not provide real typography
// or integrate the draft streaming path into paragraph encoding.
//
// Faces, scales, and the default resolver are immutable and can be shared.
// Metric shapers are mutable and require exclusive use. Streaming shapers have
// their own producer/consumer lifecycle; do not copy them after first use.
// Future fixtures that depend on whole paragraphs belong in a separate testing
// package, so helpers never needs to import the root engine.
package helpers
