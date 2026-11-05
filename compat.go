package khipu

import (
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

// These aliases preserve the original public names and type identity. New
// adapters should use contracts directly; default implementations live in
// helpers. The root package contains no duplicate contract or helper logic.

// FontLoader is the compatibility name for contracts.FontLoader.
type FontLoader = contracts.FontLoader

// FontFace is the compatibility name for contracts.FontFace.
type FontFace = contracts.FontFace

// FontMetrics is the compatibility name for contracts.FontMetrics.
type FontMetrics = contracts.FontMetrics

// FontScale is the compatibility name for contracts.FontScale.
type FontScale = contracts.FontScale

// RuneReader is the compatibility name for contracts.RuneReader.
type RuneReader = contracts.RuneReader

// Shaper is the compatibility name for contracts.Shaper.
type Shaper = contracts.Shaper

// StreamingShaper is the compatibility name for contracts.StreamingShaper.
type StreamingShaper = contracts.StreamingShaper

// ShapeParams is the compatibility name for contracts.ShapeParams.
type ShapeParams = contracts.ShapeParams

// ShapeFeature is the compatibility name for contracts.ShapeFeature.
type ShapeFeature = contracts.ShapeFeature

// ShapeBufferOptions is the compatibility name for contracts.ShapeBufferOptions.
type ShapeBufferOptions = contracts.ShapeBufferOptions

// ShapeFlushBoundary is the compatibility name for contracts.ShapeFlushBoundary.
type ShapeFlushBoundary = contracts.ShapeFlushBoundary

// ShapedGlyph is the compatibility name for contracts.ShapedGlyph.
type ShapedGlyph = contracts.ShapedGlyph

// StyleResolver is the compatibility name for contracts.StyleResolver.
type StyleResolver = contracts.StyleResolver

// ResolvedRunStyle is the compatibility name for contracts.ResolvedRunStyle.
type ResolvedRunStyle = contracts.ResolvedRunStyle

// SpacingMode is the compatibility name for contracts.SpacingMode.
type SpacingMode = contracts.SpacingMode

// Flush and spacing constants retain their original values and types.
const (
	ShapeFlushRun     = contracts.ShapeFlushRun
	ShapeFlushCluster = contracts.ShapeFlushCluster
	SpacingFixed      = contracts.SpacingFixed
	SpacingElastic    = contracts.SpacingElastic
)

// SyntheticFont is the compatibility name for helpers.SyntheticFont.
type SyntheticFont = helpers.SyntheticFont

// FixedWidthShaper is the compatibility name for helpers.FixedWidthShaper.
type FixedWidthShaper = helpers.FixedWidthShaper

// DefaultStyleResolver is the compatibility name for helpers.DefaultStyleResolver.
type DefaultStyleResolver = helpers.DefaultStyleResolver

// SyntheticStreamingShaper is the compatibility name for helpers.SyntheticStreamingShaper.
type SyntheticStreamingShaper = helpers.SyntheticStreamingShaper

// Streaming error values retain identity with their helpers counterparts.
var (
	ErrShaperBusy   = helpers.ErrShaperBusy
	ErrShaperClosed = helpers.ErrShaperClosed
)

// NewSyntheticFont forwards to helpers.NewSyntheticFont.
func NewSyntheticFont() *SyntheticFont { return helpers.NewSyntheticFont() }

// NewFixedWidthShaper forwards to helpers.NewFixedWidthShaper.
func NewFixedWidthShaper(cellWidth dimen.DU) (*FixedWidthShaper, error) {
	return helpers.NewFixedWidthShaper(cellWidth)
}

// NewDefaultStyleResolver forwards to helpers.NewDefaultStyleResolver.
func NewDefaultStyleResolver() *DefaultStyleResolver { return helpers.NewDefaultStyleResolver() }

// NewSyntheticStreamingShaper forwards to helpers.NewSyntheticStreamingShaper.
func NewSyntheticStreamingShaper(outputCapacity int) (*SyntheticStreamingShaper, error) {
	return helpers.NewSyntheticStreamingShaper(outputCapacity)
}
