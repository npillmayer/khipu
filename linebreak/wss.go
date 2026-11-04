package linebreak

import (
	"fmt"
	"math"

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
		W:   boundedWidth(int64(wss.W) + int64(other.W)),
		Min: boundedWidth(int64(wss.Min) + int64(other.Min)),
		Max: boundedWidth(int64(wss.Max) + int64(other.Max)),
	}
}

// Subtract returns the pairwise difference of two elastic widths.
func (wss WSS) Subtract(other WSS) WSS {
	return WSS{
		W:   boundedWidth(int64(wss.W) - int64(other.W)),
		Min: boundedWidth(int64(wss.Min) - int64(other.Min)),
		Max: boundedWidth(int64(wss.Max) - int64(other.Max)),
	}
}

// These are width summaries, not freeze-time fill-order allocations. Saturation
// keeps a paragraph fill plus finite text/skips from wrapping to negative width.
func boundedWidth(w int64) dimen.DU {
	return dimen.DU(max(int64(math.MinInt32), min(int64(math.MaxInt32), w)))
}
