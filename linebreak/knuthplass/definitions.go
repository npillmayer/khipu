package knuthplass

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

// Merits is a signed, dimensionless score with underlying type int64. It is
// used for badness thresholds, line penalties and additive demerits; it is not
// a physical dimension or a breakpoint penalty (see khipu.Penalty).
// Convert typed configuration values explicitly, for example Merits(tolerance).
// This alias preserves the identity of the original parameter field type.
//
// Computed badness lies in [0, 10000]. Tolerances in that range are normally
// useful; a negative tolerance rejects even zero badness, and a tolerance above
// 10000 adds no further quality allowance. Hard shrink limits still apply.
// Cost parameters are not clamped or overflow-checked. Callers must keep the
// squared line costs and accumulated paragraph totals within int64's range.
type Merits = merits

type merits int64 // keep the original defined type for source/type compatibility

// Parameters is a collection of configuration parameters for line-breaking.
// Paragraph geometry lives in BreakRequest, not this algorithm configuration.
// Paragraph fill belongs to Config.ParFillSkip and the immutable input trailer.
//
// The important distinction for the current implementation is:
//   - PreTolerance screens the rough first pass
//   - Tolerance screens the second and optional emergency passes
//   - breakpoint penalties come from KnotCore.Penalty or a discretionary candidate
//   - demerit extras such as FirstHyphenDemerits / FinalHyphenDemerits are local
//     linebreaker adjustments on top of the base breakpoint penalty
type Parameters struct {
	Tolerance           Merits // admissible badness for passes 2 and 3
	PreTolerance        Merits // admissible badness for pass 1
	LinePenalty         Merits // added to badness before squaring the line cost
	FirstHyphenDemerits Merits // additive cost for a first-line discretionary break
	FinalHyphenDemerits Merits // additive cost when the last line starts after a discretionary break
	// EmergencyStretch enables a third, final pass if the first two fail.
	// A positive value must be finite (< dimen.Fil); nonpositive values disable
	// the extra pass and leave pass 2 final. Virtual stretch changes badness,
	// demerits and lazy discovery, never knot widths or shrink capacity. Freezing
	// uses real capacities and may report an underfull result with fit diagnostics.
	EmergencyStretch      dimen.DU
	discretionaryProvider discretionaryProvider
}

// NewDefaultParameters returns an independent standard algorithm profile:
// PreTolerance 100, Tolerance 200, LinePenalty 10, FirstHyphenDemerits 0,
// FinalHyphenDemerits 10000, and EmergencyStretch 0. This is the profile used
// when BreakParagraph receives nil parameters; final fallback occurs in pass 2.
// Paragraph fill is configured separately at preparation.
func NewDefaultParameters() *Parameters {
	return &Parameters{
		Tolerance:           200,
		PreTolerance:        100,
		LinePenalty:         10,
		FinalHyphenDemerits: 10000,
	}
}

// NewTolerantParameters returns an independent permissive algorithm profile.
// Relative to NewDefaultParameters it sets Tolerance to 5000,
// FinalHyphenDemerits to 50, and EmergencyStretch to 50 BP, enabling pass 3.
// Neither profile installs a discretionary provider. Mutating returned fields
// does not change future constructor results or the nil-parameter defaults.
func NewTolerantParameters() *Parameters {
	params := NewDefaultParameters()
	params.Tolerance = 5000
	params.FinalHyphenDemerits = 50
	params.EmergencyStretch = 50 * dimen.BP
	return params
}

func glue(w, wmin int, wmax dimen.DU) khipu.KnotCore {
	return khipu.KnotCore{
		W:       dimen.DU(w) * dimen.BP,
		MinW:    dimen.DU(wmin) * dimen.BP,
		MaxW:    wmax,
		Len:     0,
		Penalty: 0,
		Kind:    khipu.KnotType(khipu.KTGlue),
	}
}

// InfinityMerits is the forced-break penalty threshold (-10000).
// Use it in knot or discretionary penalties, not as a tolerance.
const InfinityMerits = -10000

// InfinityDemerits is the prohibited-break penalty threshold (10000).
// It also equals the maximum computed line badness.
const InfinityDemerits = 10000

// AwfulDemerits is the base cost of a final-pass fallback transition.
// It is not a cap on accumulated path costs.
const AwfulDemerits = 1000000000

// DefaultLayout preserves the finite-stretch right skip of the K&P profile.
// Resolve it before breaking and copy its shape/skips into BreakRequest.Geometry.
// Nil algorithm parameters never substitute geometry defaults.
func DefaultLayout(shape khipu.LineShape) khipu.Params {
	layout := khipu.DefaultParams()
	layout.LineShape = shape
	layout.LeftSkip = glue(0, 0, 0)
	layout.RightSkip = glue(0, 0, 50*dimen.BP)
	return layout
}
