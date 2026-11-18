package firstfit

import (
	"context"

	"github.com/npillmayer/khipu"
)

// ContinueParagraph locks a certified prefix and searches only the remaining
// logical knots using ordinary first-fit trimming, forced breaks and final fill.
// The binding must belong to the checkpoint's NoHyphenation preparation. No base
// shaping, font acceptance or discretionary provider is invoked. A KP prefix is
// retained exactly; its optimization state is neither required nor resumed.
//
// Supply a stable one-based width schedule and the original skips, and reuse that
// geometry for freezing: accepted lines retain their original widths and only
// suffix lines get new widths. Full binding/freezing is allowed. This operation
// owns no frame height or CSS state and does not enforce the caller's width history.
//
// The result is a detached complete breakpoint path, suitable for freezing and
// for certifying a later checkpoint. Empty and fully accepted prefixes are valid.
// Cancellation is checked between knots, after LineLength calls and before
// publishing selections. A synchronous LineLength call itself cannot be interrupted.
// Errors return nil, preserve their cause and leave prior selections/checkpoints
// reusable. Serialize operations sharing the binding or its owner.
func ContinueParagraph(ctx context.Context, khp *khipu.Khipu, layout khipu.Params, checkpoint *khipu.BreakCheckpoint) ([]int, error) {
	prefix, err := checkpoint.PrefixFor(ctx, khp)
	if err != nil {
		return nil, err
	}
	return breakParagraph(khp, layout, nil, &breakSeed{ctx: ctx, prefix: prefix})
}

type breakSeed struct {
	ctx    context.Context
	prefix []int // already detached and certified by khipu
}

func (s *breakSeed) err() error {
	if s == nil {
		return nil
	}
	return s.ctx.Err()
}
