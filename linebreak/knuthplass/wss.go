package knuthplass

import (
	"fmt"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

// Parameters is a collection of configuration parameters for line-breaking.
type Parameters struct {
	Tolerance            merits         // acceptable demerits
	PreTolerance         merits         // acceptabale demerits for first (rough) pass
	LinePenalty          merits         // penalty for an additional line
	HyphenPenalty        merits         // penalty for hyphenating words
	ExHyphenPenalty      merits         // penalty for explicit words
	DoubleHyphenDemerits merits         // demerits for consecutive hyphens
	FinalHyphenDemerits  merits         // demerits for hyphen in the last line
	EmergencyStretch     dimen.DU       // stretching acceptable when desperate
	LeftSkip             khipu.KnotCore // glue at left edge of paragraphs
	RightSkip            khipu.KnotCore // glue at right edge of paragraphs
	ParFillSkip          khipu.KnotCore // glue at the end of a paragraph
}

// DefaultParameters are the standard line-breaking parameters.
// The promote a tolerant configuration, suitable for almost always finding an
// acceptable set of linebreaks.
var DefaultParameters = &Parameters{
	Tolerance:            5000,
	PreTolerance:         100,
	LinePenalty:          10,
	HyphenPenalty:        50,
	ExHyphenPenalty:      50,
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

// capDemerits caps a demerit value at infinity.
func capDemerits(d merits) merits {
	if d > InfinityDemerits {
		d = InfinityDemerits
	} else if d < InfinityMerits-1000 {
		d = InfinityMerits - 1000
	}
	return d
}

// ----------------------------------------------------------------------

// WSS (width stretch & shrink) is a type to hold an elastic width (of text).
type WSS struct {
	W   dimen.DU
	Min dimen.DU
	Max dimen.DU
}

func (wss WSS) String() string {
	return fmt.Sprintf("WSS[%.2f -%.2f +%.2f]",
		wss.W.Points(), wss.Min.Points(), wss.Max.Points())
}

// Spread returns the width's of an elastic WSS.
func (wss WSS) Spread() (w dimen.DU, min dimen.DU, max dimen.DU) {
	return wss.W, wss.Min, wss.Max
}

// SetFromKnot sets the width's of an elastic WSS from a knot.
func (wss WSS) SetFromKnot(k khipu.KnotCore) WSS {
	wss.W = k.W
	wss.Min = k.MinW
	wss.Max = k.MaxW
	return wss
}

// Add adds dimensions from a given WSS to wss, returning a new WSS.
func (wss WSS) Add(other WSS) WSS {
	return WSS{
		W:   wss.W + other.W,
		Min: wss.Min + other.Min,
		Max: wss.Max + other.Max,
	}
}

// Subtract subtracts dimensions from a given WSS from wss, returning a new WSS.
func (wss WSS) Subtract(other WSS) WSS {
	return WSS{
		W:   wss.W - other.W,
		Min: wss.Min - other.Min,
		Max: wss.Max - other.Max,
	}
}
