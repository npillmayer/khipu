package khipu

import (
	"context"
	"fmt"

	"github.com/npillmayer/khipu/contracts"
)

// consumeShaper owns the common streaming lifecycle, not glyph interpretation.
// produce runs in one goroutine and is responsible for input and Finish;
// consume runs on the caller's path. Preparation must precede this call and
// result finalization/publication must follow successful completion.
//
// Producer completion is distinct from channel closure: a pre-Shape failure
// need not close output, while successful Finish can leave buffered glyphs to
// drain. Every exit cancels and joins the producer. A consumer error is retained
// even when that cancellation subsequently causes a producer error. As in the
// shaper contract, cancellation cannot interrupt an arbitrary backend read.
func consumeShaper(ctx context.Context, shaper contracts.StreamingShaper,
	produce func(context.Context) error, consume func(contracts.ShapedGlyph) error) error {
	if ctx == nil || nilRunService(shaper) || produce == nil || consume == nil {
		return fmt.Errorf("%w: missing streaming collaborator", contracts.ErrIllegalArguments)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	output := shaper.Glyphs()
	if output == nil {
		return fmt.Errorf("%w: nil glyph channel", contracts.ErrIllegalArguments)
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- produce(ctx) }()
	joined := false
	defer func() {
		cancel()
		if !joined {
			<-done
		}
	}()
	completion := done
	for output != nil || completion != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-completion:
			joined, completion = true, nil
			if err != nil {
				return err
			}
		case glyph, ok := <-output:
			if !ok {
				output = nil
				continue
			}
			if err := consume(glyph); err != nil {
				return err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return shaper.Err()
}
