package helpers

import (
	"fmt"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

const syntheticUnitsPerEm = 1000

// SyntheticFont implements contracts.FontFace for tests and simple client setups.
// Its zero value is ready to use. Metrics are fixed at 1000 units per em,
// ascent 800, descent -200, and line gap zero.
//
// It has no glyphs, outlines, character map, or backend font data. A matching
// synthetic shaper must define its own advances; an advance of 500 font units
// represents half an em. Real shaping adapters must reject this face unless
// they explicitly support it. The current khipu.Config shaper does not use a face.
// The face and all scales it returns are safe for concurrent reads.
type SyntheticFont struct{}

// NewSyntheticFont returns a ready-to-use synthetic face without loading a font.
func NewSyntheticFont() *SyntheticFont {
	return &SyntheticFont{}
}

// Metrics returns a value snapshot of the fixed, unscaled font metrics.
func (SyntheticFont) Metrics() (contracts.FontMetrics, error) {
	return contracts.FontMetrics{
		UnitsPerEm: syntheticUnitsPerEm,
		Ascent:     800,
		Descent:    -200,
		LineGap:    0,
	}, nil
}

// Scale returns an immutable conversion for a positive em size. Nonpositive
// sizes return an error wrapping contracts.ErrIllegalArguments and no scale.
func (SyntheticFont) Scale(emSize dimen.DU) (contracts.FontScale, error) {
	if emSize <= 0 {
		return nil, fmt.Errorf("%w: synthetic font em size must be positive", contracts.ErrIllegalArguments)
	}
	return syntheticFontScale{emSize: emSize}, nil
}

type syntheticFontScale struct {
	emSize dimen.DU
}

// ToDU preserves signs and fractional DU values. Conversion to float64 occurs
// before multiplication to avoid integer overflow. The caller owns rounding
// and range checking before narrowing the result to dimen.DU.
func (s syntheticFontScale) ToDU(fontUnits int32) float64 {
	return float64(fontUnits) * float64(s.emSize) / syntheticUnitsPerEm
}

var (
	_ contracts.FontFace  = SyntheticFont{}
	_ contracts.FontScale = syntheticFontScale{}
)
