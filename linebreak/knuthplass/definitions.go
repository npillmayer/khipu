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

// DefaultParameters are the standard line-breaking parameters.
// They promote a tolerant configuration, suitable for almost always finding an
// acceptable set of linebreaks.
var DefaultParameters = &Parameters{
	Tolerance:           5000,
	PreTolerance:        100,
	LinePenalty:         10,
	FirstHyphenDemerits: 0,
	FinalHyphenDemerits: 50,
	EmergencyStretch:    dimen.DU(dimen.BP * 50),
	ParFillSkip:         glue(0, 0, dimen.Fill),
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
