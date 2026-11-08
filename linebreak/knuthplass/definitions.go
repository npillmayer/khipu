package knuthplass

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

// Parameters is a collection of configuration parameters for line-breaking.
// Paragraph geometry lives in khipu.Params, not in this algorithm configuration.
// ParFillSkip remains an encoding convenience; the breaker reads the actual
// trailer from its input and does not substitute this value during breaking.
//
// The important distinction for the current implementation is:
//   - PreTolerance screens the rough first pass
//   - Tolerance screens the second and optional emergency passes
//   - breakpoint penalties come from KnotCore.Penalty or a discretionary candidate
//   - demerit extras such as FirstHyphenDemerits / FinalHyphenDemerits are local
//     linebreaker adjustments on top of the base breakpoint penalty
type Parameters struct {
	Tolerance           merits // admissible badness for passes 2 and 3
	PreTolerance        merits // admissible badness for pass 1
	LinePenalty         merits // base cost added to every accepted line
	FirstHyphenDemerits merits // demerits for hyphen in the first line
	FinalHyphenDemerits merits // demerits for hyphen in the last line
	// EmergencyStretch enables a third, final pass if the first two fail.
	// A positive value must be finite (< dimen.Fil); nonpositive values disable
	// the extra pass and leave pass 2 final. Virtual stretch changes badness,
	// demerits and lazy discovery, never knot widths or shrink capacity. Freezing
	// uses real capacities and may still reject underfull output.
	EmergencyStretch      dimen.DU
	ParFillSkip           khipu.KnotCore // upstream trailer dimensions; pass to khipu.Config.ParFillSkip when encoding
	DiscretionaryProvider DiscretionaryProvider
}

// NewDefaultParameters returns an independent standard algorithm profile:
// PreTolerance 100, Tolerance 200, LinePenalty 10, FirstHyphenDemerits 0,
// FinalHyphenDemerits 10000, and EmergencyStretch 0. This is the profile used
// when BreakParagraph receives nil parameters; final fallback occurs in pass 2.
// ParFillSkip is fill glue for upstream encoding; layout remains separate.
func NewDefaultParameters() *Parameters {
	return &Parameters{
		Tolerance:           200,
		PreTolerance:        100,
		LinePenalty:         10,
		FinalHyphenDemerits: 10000,
		ParFillSkip:         glue(0, 0, dimen.Fill),
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

const InfinityMerits = -10000
const InfinityDemerits = 10000
const AwfulDemerits = 1000000000

// clampPenalty caps a penalty value to the TeX-style sentinel range.
func clampPenalty(p merits) merits {
	if p > InfinityDemerits {
		return InfinityDemerits
	}
	if p < InfinityMerits {
		return InfinityMerits
	}
	return p
}

// WSS is shared with other linebreakers through the root linebreak package.
type WSS = linebreak.WSS
