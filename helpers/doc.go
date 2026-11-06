// Package helpers provides deterministic implementations of khipu's service
// contracts for tests, examples, and simple client setups. It depends on
// contracts, not the root paragraph engine, and requires no installed fonts.
//
// SyntheticFont, DefaultStyleResolver, and SyntheticStreamingShaper form the
// default paragraph-encoding backend and can also be used in standalone tests.
// They do not provide real typography.
//
// Faces, scales, and the default resolver are immutable and can be shared.
// Streaming shapers have
// their own producer/consumer lifecycle; do not copy them after first use.
// Future fixtures that depend on whole paragraphs belong in a separate testing
// package, so helpers never needs to import the root engine.
package helpers
