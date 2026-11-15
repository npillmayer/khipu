package firstfit

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

type kinx = int

const (
	noinx = -1
	// InfinityMerits is the forced-break penalty threshold (-10000).
	InfinityMerits = -10000
	// InfinityDemerits is the prohibited-break penalty threshold (10000).
	InfinityDemerits = 10000
)

// Parameters configures the SOA first-fit breaker.
// Geometry lives exclusively in khipu.Params, shared with freezing.
type Parameters struct {
	// MinHyphenGain is the inclusive minimum natural-width gain over the last
	// ordinary checkpoint. Zero permits zero improvement, not just positive gain.
	MinHyphenGain         dimen.DU
	DiscretionaryProvider DiscretionaryProvider
}

// NewDefaultParameters returns independent algorithm defaults: zero minimum
// hyphenation gain and no provider. Nil parameters to BreakParagraph use this
// profile. Paragraph geometry is resolved separately with DefaultLayout.
func NewDefaultParameters() *Parameters { return &Parameters{} }

// DefaultLayout resolves a ragged-right paragraph profile. The physical right
// skip absorbs positive slack at fil order, leaving finite interword glue at
// natural width. Higher-order paragraph fill still wins on the final line.
// This is not a no-compression policy: when natural width exceeds the target,
// the breaker may use the shrink capacity explicitly supplied by the knots.
// Apply overrides after this call and reuse the returned Params for freezing.
// No defaults are reapplied by BreakParagraph, even for explicitly zero skips.
func DefaultLayout(shape khipu.LineShape) khipu.Params {
	layout := khipu.DefaultParams()
	layout.LineShape = shape
	layout.LeftSkip = glue(0, 0, 0)
	layout.RightSkip = glue(0, 0, dimen.Fil)
	return layout
}

func glue(w, wmin int, wmax dimen.DU) khipu.KnotCore {
	return khipu.KnotCore{
		W:       dimen.DU(w) * dimen.BP,
		MinW:    dimen.DU(wmin) * dimen.BP,
		MaxW:    wmax,
		Penalty: 0,
		Kind:    khipu.KTGlue,
	}
}
