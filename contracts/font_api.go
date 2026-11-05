package contracts

import (
	"io"

	"github.com/npillmayer/khipu/dimen"
)

// FontLoader is the draft font-loading boundary. An adapter owns parsing and
// retains any backing storage needed by the returned face. Font discovery,
// family matching, and fallback selection remain outside this small interface.
type FontLoader interface {
	LoadFont(src io.Reader) (FontFace, error)
}

// FontFace is an opaque, immutable face with query services. Its implementation
// retains the backend font; no OpenType representation crosses this boundary.
// A shaping adapter must reject faces from unsupported backends with an error.
// Loading a face does not select a size: one face may serve several sized runs.
type FontFace interface {
	Metrics() (FontMetrics, error)
	Scale(emSize dimen.DU) (FontScale, error)
}

// FontMetrics contains signed, unscaled font-unit measurements. Descent keeps
// the backend's signed coordinate convention (normally negative).
type FontMetrics struct {
	UnitsPerEm uint32
	Ascent     int32
	Descent    int32
	LineGap    int32
}

// FontScale converts font units to fractional dimen.DU units. It is bound to
// one face and positive em size and must be immutable and safe for concurrent
// reads. An OpenType adapter can obtain the factor with
// otquery.FontScale(font, float64(emSize)). Conversion preserves signs and does
// not change axis orientation. Keep fractions while accumulating advances;
// rounding and range checking before conversion to dimen.DU belong to the caller.
type FontScale interface {
	ToDU(fontUnits int32) float64
}
