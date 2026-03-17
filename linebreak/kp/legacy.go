package knuthplass

import (
	"fmt"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

// Merits is the legacy demerits type used by the original kp implementation.
type Merits int32

type CharPos int64

// Parameters is the original Knuth-Plass parameter set kept local to the
// legacy kp implementation. Newer breakers keep their own parameter models.
type Parameters struct {
	Tolerance            Merits     // acceptable demerits
	PreTolerance         Merits     // acceptable demerits for first (rough) pass
	LinePenalty          Merits     // penalty for an additional line
	HyphenPenalty        Merits     // penalty for hyphenating words
	ExHyphenPenalty      Merits     // penalty for explicit words
	DoubleHyphenDemerits Merits     // demerits for consecutive hyphens
	FinalHyphenDemerits  Merits     // demerits for hyphen in the last line
	EmergencyStretch     dimen.DU   // stretching acceptable when desperate
	LeftSkip             khipu.Glue // glue at left edge of paragraphs
	RightSkip            khipu.Glue // glue at right edge of paragraphs
	ParFillSkip          khipu.Glue // glue at the end of a paragraph
}

// DefaultParameters preserve the historic kp defaults for callers that still
// use the legacy package directly.
var DefaultParameters = &Parameters{
	Tolerance:            5000,
	PreTolerance:         100,
	LinePenalty:          10,
	HyphenPenalty:        50,
	ExHyphenPenalty:      50,
	DoubleHyphenDemerits: 0,
	FinalHyphenDemerits:  50,
	EmergencyStretch:     dimen.DU(dimen.BP * 50),
	LeftSkip:             khipu.NewGlue(0, 0, 0),
	RightSkip:            khipu.NewGlue(0, 0, 0),
	ParFillSkip:          khipu.NewGlue(0, 0, 0),
}

// WSS holds an elastic width triple for the legacy kp implementation.
type WSS struct {
	W   dimen.DU
	Min dimen.DU
	Max dimen.DU
}

func (wss WSS) Spread() (w dimen.DU, min dimen.DU, max dimen.DU) {
	return wss.W, wss.Min, wss.Max
}

func (wss WSS) SetFromKnot(knot khipu.Knot) WSS {
	if knot == nil {
		return wss
	}
	wss.W = knot.W()
	wss.Min = knot.MinW()
	wss.Max = knot.MaxW()
	return wss
}

func (wss WSS) Add(other WSS) WSS {
	return WSS{
		W:   wss.W + other.W,
		Min: wss.Min + other.Min,
		Max: wss.Max + other.Max,
	}
}

func (wss WSS) Subtract(other WSS) WSS {
	return WSS{
		W:   wss.W - other.W,
		Min: wss.Min - other.Min,
		Max: wss.Max - other.Max,
	}
}

func (wss WSS) Copy() WSS {
	return WSS{W: wss.W, Min: wss.Min, Max: wss.Max}
}

func (wss WSS) String() string {
	return fmt.Sprintf("{%.2f < %.2f < %.2f}", wss.Min.Points(), wss.W.Points(), wss.Max.Points())
}

const (
	InfinityDemerits Merits = 10000
	InfinityMerits   Merits = -10000
)

func CapDemerits(d Merits) Merits {
	if d > InfinityDemerits {
		d = InfinityDemerits
	} else if d < InfinityMerits-1000 {
		d = InfinityMerits - 1000
	}
	return d
}

// Cursor is the legacy iterator abstraction used by the original kp package.
type Cursor interface {
	Next() bool
	Knot() khipu.Knot
	Peek() (khipu.Knot, bool)
	Mark() khipu.Mark
	Khipu() *khipu.KhipuAOS
}
