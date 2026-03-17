package linebreak

import (
	"fmt"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

// WSS (width, stretch, shrink) holds an elastic width triple used by
// linebreakers for segment bookkeeping and line-width calculation.
type WSS struct {
	W   dimen.DU
	Min dimen.DU
	Max dimen.DU
}

func (wss WSS) String() string {
	return fmt.Sprintf("WSS[%.2f -%.2f +%.2f]",
		wss.W.Points(), wss.Min.Points(), wss.Max.Points())
}

// Spread returns the natural, minimum, and maximum widths of an elastic WSS.
func (wss WSS) Spread() (w dimen.DU, min dimen.DU, max dimen.DU) {
	return wss.W, wss.Min, wss.Max
}

// SetFromKnot copies elastic widths from one knot core into a WSS helper.
func (wss WSS) SetFromKnot(k khipu.KnotCore) WSS {
	wss.W = k.W
	wss.Min = k.MinW
	wss.Max = k.MaxW
	return wss
}

// Add returns the pairwise sum of two elastic widths.
func (wss WSS) Add(other WSS) WSS {
	return WSS{
		W:   wss.W + other.W,
		Min: wss.Min + other.Min,
		Max: wss.Max + other.Max,
	}
}

// Subtract returns the pairwise difference of two elastic widths.
func (wss WSS) Subtract(other WSS) WSS {
	return WSS{
		W:   wss.W - other.W,
		Min: wss.Min - other.Min,
		Max: wss.Max - other.Max,
	}
}
