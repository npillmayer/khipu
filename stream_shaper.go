package khipu

import (
	"context"
	"errors"
	"fmt"

	"github.com/npillmayer/khipu/contracts"
)

// finishStream releases an exclusively owned idle stream, after any producer
// has joined. Err is sampled before Finish so newly reported cleanup failures
// cannot be mistaken for an earlier terminal shaping error. When a primary
// failure already exists, that terminal error has already been handled by the
// coordinator (and may simply be its internal cancellation of the producer).
func finishStream(shaper contracts.StreamingShaper, primary error) error {
	terminal := shaper.Err()
	cleanup := shaper.Finish()
	if primary == nil {
		return cleanup
	}
	cleanup, _ = withoutStreamTerminal(cleanup, terminal)
	if cleanup != nil && !errors.Is(primary, cleanup) {
		return errors.Join(primary, fmt.Errorf("khipu: finishing stream: %w", cleanup))
	}
	return primary
}

// withoutStreamTerminal removes repeated terminal causes from a Finish error.
// Inspect joined causes separately: a joined terminal error must not hide an
// independent cleanup failure. Preserve untouched wrappers; when a wrapper
// contains both removed and surviving causes, return only the surviving tree.
func withoutStreamTerminal(err, terminal error) (error, bool) {
	if err == nil || terminal == nil {
		return err, false
	}
	if errors.Is(terminal, err) {
		return nil, true
	}
	switch wrapped := err.(type) {
	case interface{ Unwrap() []error }:
		var remaining []error
		changed := false
		for _, cause := range wrapped.Unwrap() {
			keep, removed := withoutStreamTerminal(cause, terminal)
			changed = changed || removed
			if keep != nil {
				remaining = append(remaining, keep)
			}
		}
		if changed {
			return errors.Join(remaining...), true
		}
	case interface{ Unwrap() error }:
		if keep, removed := withoutStreamTerminal(wrapped.Unwrap(), terminal); removed {
			return keep, true
		}
	}
	return err, false
}

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
