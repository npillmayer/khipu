package khipu

import (
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

// preparedRun describes one uniform shaping run before submission to a
// contracts.StreamingShaper. It is the internal, resolved form of the algorithm's P(R)
// together with the original source range to which those parameters apply.
//
// The paragraph owns the text. This record owns neither a text copy nor a
// reader cursor; the producer will obtain a reader over source when needed.
// Run preparation must resolve all parameters before publishing the record,
// and publish metadata needed by knotting before corresponding glyphs arrive.
// After publication, the record and its referenced data must remain immutable
// while producer, consumer, or later paragraph processing uses them.
//
// prepareParagraphRuns constructs these records independently. The current
// encoder does not yet prepare or consume them.
type preparedRun struct {
	source runSourceRange
	params runParameters
}

// runSourceRange identifies the same original text in two coordinate systems.
// Both ranges are half-open and relative to the paragraph: byteStart:byteEnd
// counts UTF-8 bytes, scalarStart:scalarEnd counts Unicode scalar values.
// Neither range describes normalized glyph output or counts shaping clusters.
// For example, "a\u00e9" spans three bytes and two scalars.
//
// Preparation must ensure ordered bounds, UTF-8 boundaries, and agreement of
// the two ranges with the paragraph source. Add the paragraph's Offset only
// when producing absolute source positions. The uint64 endpoints permit an
// exclusive scalar end beyond the last uint32 cluster ID; submission must
// check the contracts.StreamingShaper's representable range before narrowing indices.
// Per-scalar byte mapping, when needed, is separate from these run endpoints.
type runSourceRange struct {
	byteStart   uint64
	byteEnd     uint64
	scalarStart uint64
	scalarEnd   uint64
}

// runParameters holds resolved values, not styles requiring interpretation.
// Font selection, script/language resolution, and bidi analysis belong to the
// preparation procedure. All shaping properties are uniform over the run.
//
// Preparation must take an owned copy of shaping.Features. Its ranges remain
// run-relative scalar indices, as specified by contracts.ShapeFeature. contracts.FontFace and
// contracts.FontScale implementations obey their immutable-query contracts.
type runParameters struct {
	shaping contracts.ShapeParams
	// emSize records the positive selected size; scale must be obtained from
	// shaping.Font for this same size. Keeping the size allows later reshaping
	// without attempting to recover it from an opaque conversion object.
	emSize dimen.DU
	scale  contracts.FontScale
	// spacing governs interword spaces, not the adjustability of textboxes.
	spacing spacingMode
}

// spacingMode describes the run's resolved interword-spacing policy. It does
// not determine discardability or prescribe numerical stretch/shrink amounts.
// Although the zero value denotes fixed spacing, preparation must resolve the
// policy explicitly rather than treat zero as an unspecified paragraph default.
type spacingMode = contracts.SpacingMode

const (
	spacingFixed   = contracts.SpacingFixed
	spacingElastic = contracts.SpacingElastic
)
