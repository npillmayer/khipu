package contracts

import (
	"context"

	"golang.org/x/text/language"
	"golang.org/x/text/unicode/bidi"
)

// StreamingShaper supplies glyphs and positioning for paragraph encoding and
// discretionary reshaping, in logical source order.
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
	FontAcceptor
	// Shape consumes a uniform run synchronously while the caller drains Glyphs.
	// Success leaves the stream open for another run; accepted failure is terminal.
	Shape(context.Context, ShapeParams, RuneReader, ShapeBufferOptions) error
	// SetInitialCluster overrides the next run's scalar base while idle.
	// Calling it during Shape must fail without disrupting the active operation.
	SetInitialCluster(uint32) error
	// Glyphs supplies logical-order records. Drain concurrently with Shape;
	// channel closure alone does not establish successful producer completion.
	Glyphs() <-chan ShapedGlyph
	// Finish closes output after the final run. It must not disrupt active Shape.
	// It is idempotent and releases stream-owned resources even when Shape was
	// never called. Finishing an unused idle stream emits no glyphs and needs no
	// drainer. Callers finish abandoned streams after any active producer joins.
	Finish() error
	// Err reports terminal stream failure. Check it after draining output and
	// observing producer completion; it may be queried concurrently with Shape.
	Err() error
}

// ShapeParams describes one uniform run. Font, script, language, and direction
// stay fixed during a call. Font carries its immutable size/scale binding; output remains in font units.
// Spacing and source mappings remain in Khipukamayuq's run metadata. Publish metadata needed
// by the consumer before the corresponding glyphs can arrive.
type ShapeParams struct {
	Font      FontInstance
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

// ShapeFlushBoundary requests a backend buffering/emission policy. Neither mode
// adds run/cluster completion markers to Glyphs or permits unsafe shaping cuts.
// Emission timing and input buffering depend on the backend; these values do not
// promise a fixed latency or that a whole run is withheld until Shape returns.
// Backends should report unsupported options rather than silently treating them
// as supported. The synthetic helper accepts both modes and emits scalar cells.
type ShapeFlushBoundary uint8

const (
	// ShapeFlushRun requests run-oriented emission and is the zero-value policy.
	ShapeFlushRun ShapeFlushBoundary = iota
	// ShapeFlushCluster requests emission at safe completed-cluster boundaries.
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
