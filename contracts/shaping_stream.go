package contracts

import (
	"context"

	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

// StreamingShaper is the draft replacement for Shaper. It is deliberately
// separate: existing encoding and discretionary paths still use Shaper.
// Package helpers supplies a deterministic streaming implementation for tests;
// applications supply adapters for real font backends.
//
// A producer calls Shape sequentially for uniform runs while a consumer drains
// Glyphs. Shape is synchronous and may block on output backpressure. Successful
// calls leave the channel open. Finish closes it after the final run. An
// accepted Shape failure or cancellation closes it with a terminal Err; already
// emitted glyphs remain available. Commit paragraph output only after success.
//
// Concurrent Shape calls are unsupported. Finish and SetInitialCluster during
// Shape must report an error without disrupting the active call. Glyphs and Err
// may be queried concurrently. A consumer stopping early cancels the producer
// context; cancellation need not interrupt an already executing source read.
// A finished instance cannot be reused for later discretionary shaping.
type StreamingShaper interface {
	Shape(context.Context, ShapeParams, RuneReader, ShapeBufferOptions) error
	SetInitialCluster(uint32) error
	Glyphs() <-chan ShapedGlyph
	Finish() error
	Err() error
}

// ShapeParams describes one uniform run. Font, script, language, and direction
// stay fixed during a call. Spacing, font size/scale, and
// source mappings remain in Khipukamayuq's run metadata. Publish metadata needed
// by the consumer before the corresponding glyphs can arrive.
type ShapeParams struct {
	Font      FontFace
	Direction bidi.Direction
	Script    language.Script
	Language  language.Tag
	Features  []ShapeFeature
}

// ShapeFeature selects an OpenType-style feature without importing backend
// types. Tag contains four bytes. Start/End are a half-open range of original
// run-relative Unicode scalar indices, not bytes or glyph indices. Start <= 0
// means the beginning; End <= 0 means the end. Value is a one-based alternate,
// zero requests the default, and negative values request random selection.
// In ResolvedRunStyle, these coordinates instead refer to the original style
// span; run preparation clips and rebases them before constructing ShapeParams.
type ShapeFeature struct {
	Tag        [4]byte
	Enabled    bool
	Value      int
	Start, End int
}

// ShapeBufferOptions controls backend input buffering, independently of the
// adapter's output channel capacity. Counts are input Unicode scalars; zeros
// select backend defaults. Cluster flushing never authorizes an unsafe cut.
type ShapeBufferOptions struct {
	Flush         ShapeFlushBoundary
	HighWatermark int
	LowWatermark  int
	MaxBuffer     int
}

// ShapeFlushBoundary requests run or cluster emission. Explicit flush signaling
// is omitted because the proposed backend does not implement it yet.
type ShapeFlushBoundary uint8

const (
	ShapeFlushRun ShapeFlushBoundary = iota
	ShapeFlushCluster
)

// ShapedGlyph is a value record in logical shaping order, including RTL runs.
// Order inside a cluster may reflect normalization and mark reordering. GlyphID
// is meaningful only with its run's face. Advances include nominal advance and
// shaping adjustments; offsets move the glyph without advancing the pen. All
// positions are signed font units, to be converted using that run's FontScale.
//
// Cluster anchors the first original Unicode scalar of its shaping group, not
// the last rune carrying a boundary penalty. IDs continue across successful
// calls by input scalar count, including scalars producing no output.
// Within logical-order output anchors are nondecreasing; glyphs sharing an
// anchor are consecutive, including for RTL runs.
// SetInitialCluster overrides the next base. Khipukamayuq retains source spans
// and penalties separately; cluster boundaries alone do not permit line breaks.
// The channel contains no run or cluster completion markers. Backend feature
// masks and raw safety bits are intentionally not exposed as layout policy.
type ShapedGlyph struct {
	GlyphID  uint32
	Cluster  uint32
	XAdvance int32
	YAdvance int32
	XOffset  int32
	YOffset  int32
}
