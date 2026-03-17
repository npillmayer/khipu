package knuthplass

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

// Parameters is a collection of configuration parameters for line-breaking.
//
// The important distinction for the current implementation is:
//   - PreTolerance screens the rough first pass
//   - Tolerance screens the second pass
//   - penalties such as HyphenPenalty belong to one breakpoint candidate
//   - demerit extras such as FirstHyphenDemerits / FinalHyphenDemerits are local
//     linebreaker adjustments on top of the base breakpoint penalty
type Parameters struct {
	Tolerance             merits         // admissible badness for pass 2
	PreTolerance          merits         // admissible badness for pass 1
	LinePenalty           merits         // base cost added to every accepted line
	HyphenPenalty         merits         // penalty for hyphenating words
	FirstHyphenDemerits   merits         // demerits for hyphen in the first line
	DoubleHyphenDemerits  merits         // demerits for consecutive hyphens; currently reserved, not yet applied
	FinalHyphenDemerits   merits         // demerits for hyphen in the last line
	EmergencyStretch      dimen.DU       // stretching acceptable when desperate
	LeftSkip              khipu.KnotCore // glue at left edge of paragraphs
	RightSkip             khipu.KnotCore // glue at right edge of paragraphs
	ParFillSkip           khipu.KnotCore // glue at the end of a paragraph
	DiscretionaryProvider DiscretionaryProvider
}

// DefaultParameters are the standard line-breaking parameters.
// The promote a tolerant configuration, suitable for almost always finding an
// acceptable set of linebreaks.
var DefaultParameters = &Parameters{
	Tolerance:            5000,
	PreTolerance:         100,
	LinePenalty:          10,
	HyphenPenalty:        50,
	FirstHyphenDemerits:  0,
	DoubleHyphenDemerits: 0,
	FinalHyphenDemerits:  50,
	EmergencyStretch:     dimen.DU(dimen.BP * 50),
	LeftSkip:             glue(0, 0, 0),
	RightSkip:            glue(0, 0, 50*dimen.BP),
	ParFillSkip:          glue(0, 0, dimen.Fill),
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
