package linebreak

import (
	"fmt"
	"math"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

// WSS (width, stretch, shrink) holds an elastic width triple used by
// linebreakers for segment bookkeeping and line-width calculation. W, Min and
// Max are natural, minimum and maximum widths, not stretch/shrink deltas.
// The zero value is an empty sum. Public fields are bounded to dimen.DU's int32
// range; private excess preserves signed sums for subsequent arithmetic until
// int64 overflow, reported by [WSS.Err]. Do not edit fields of computed sums.
// These numerical summaries do not implement freeze-time symbolic fill order.
type WSS struct {
	W   dimen.DU
	Min dimen.DU
	Max dimen.DU
	// Preserve the part outside DU's range so later signed contributions and
	// trimming can recover the exact sum. Public fields remain bounded views.
	excess   [3]int64
	overflow bool
}

// String returns the bounded width fields in big points for diagnostics.
func (wss WSS) String() string {
	return fmt.Sprintf("WSS[%.2f -%.2f +%.2f]",
		wss.W.Points(), wss.Min.Points(), wss.Max.Points())
}

// Spread returns the natural, minimum, and maximum widths of an elastic WSS.
func (wss WSS) Spread() (w dimen.DU, min dimen.DU, max dimen.DU) {
	return wss.W, wss.Min, wss.Max
}

// SetFromKnot returns a fresh summary of k, ignoring the receiver. It does not
// mutate the receiver and carries no previous excess or overflow state.
func (wss WSS) SetFromKnot(k khipu.KnotCore) WSS {
	return WSS{W: k.W, Min: k.MinW, Max: k.MaxW}
}

// Add returns the pairwise sum without changing either operand. The result
// retains excess beyond the public field range; check [WSS.Err] for overflow.
func (wss WSS) Add(other WSS) WSS {
	return wss.combine(other, false)
}

// Subtract returns the pairwise difference without changing either operand.
// Retained excess participates in subtraction; check [WSS.Err] for overflow.
func (wss WSS) Subtract(other WSS) WSS {
	return wss.combine(other, true)
}

// Err reports checked int64 accumulation overflow. Add/Subtract preserve this
// failure until the summary is reset. Do not mutate fields of computed sums;
// use SetFromKnot or a fresh literal to start a new summary.
func (wss WSS) Err() error {
	if wss.overflow {
		return fmt.Errorf("%w: line width accumulation overflow", khipu.ErrIllegalArguments)
	}
	return nil
}

func (wss WSS) combine(other WSS, subtract bool) WSS {
	result := WSS{overflow: wss.overflow || other.overflow}
	a := [3]dimen.DU{wss.W, wss.Min, wss.Max}
	b := [3]dimen.DU{other.W, other.Min, other.Max}
	var fields [3]dimen.DU
	for i := range a {
		x, y := int64(a[i])+wss.excess[i], int64(b[i])+other.excess[i]
		if subtract {
			if (y > 0 && x < math.MinInt64+y) || (y < 0 && x > math.MaxInt64+y) {
				result.overflow = true
				continue
			}
			x -= y
		} else {
			if (y > 0 && x > math.MaxInt64-y) || (y < 0 && x < math.MinInt64-y) {
				result.overflow = true
				continue
			}
			x += y
		}
		fields[i] = boundedWidth(x)
		result.excess[i] = x - int64(fields[i])
	}
	result.W, result.Min, result.Max = fields[0], fields[1], fields[2]
	return result
}

// These are width summaries, not freeze-time fill-order allocations. Saturation
// keeps a paragraph fill plus finite text/skips from wrapping to negative width.
// The excess is retained privately, not lost at each intermediate addition.
func boundedWidth(w int64) dimen.DU {
	return dimen.DU(max(int64(math.MinInt32), min(int64(math.MaxInt32), w)))
}
