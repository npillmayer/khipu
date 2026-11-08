package khipu

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/npillmayer/khipu/contracts"
)

func TestConsumeShaperDrainsBeforeTerminalError(t *testing.T) {
	for _, terminal := range []error{nil, errors.New("terminal failure")} {
		s := newPipelineShaper()
		s.out = make(chan contracts.ShapedGlyph, 3)
		s.terminalErr = terminal
		var got []uint32
		err := consumeShaper(t.Context(), s, func(context.Context) error {
			for i := range uint32(3) {
				s.out <- contracts.ShapedGlyph{Cluster: i}
			}
			close(s.out)
			return nil
		}, func(g contracts.ShapedGlyph) error {
			got = append(got, g.Cluster)
			return nil
		})
		if !errors.Is(err, terminal) || !slices.Equal(got, []uint32{0, 1, 2}) {
			t.Fatalf("got anchors %v, error %v; want full output and %v", got, err, terminal)
		}
	}
}

func TestConsumeShaperProducerFailure(t *testing.T) {
	failure := errors.New("producer failure")
	for _, closeOutput := range []bool{false, true} {
		s := newPipelineShaper()
		err := consumeShaper(t.Context(), s, func(context.Context) error {
			if closeOutput {
				close(s.out)
			}
			return failure
		}, func(contracts.ShapedGlyph) error { return nil })
		if !errors.Is(err, failure) {
			t.Fatalf("closed=%v: got %v", closeOutput, err)
		}
	}
}

func TestConsumeShaperConsumerFailureCancelsAndJoins(t *testing.T) {
	s := newPipelineShaper()
	failure := errors.New("consumer failure")
	err := consumeShaper(t.Context(), s, func(ctx context.Context) error {
		defer close(s.exited)
		s.out <- contracts.ShapedGlyph{}
		<-ctx.Done()
		return ctx.Err()
	}, func(contracts.ShapedGlyph) error { return failure })
	if !errors.Is(err, failure) {
		t.Fatalf("consumer error lost: %v", err)
	}
	select {
	case <-s.exited:
	default:
		t.Fatal("returned before producer exited")
	}
}

func TestConsumeShaperCancellationJoinsProducer(t *testing.T) {
	s := newPipelineShaper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	err := consumeShaper(ctx, s, func(ctx context.Context) error {
		defer close(s.exited)
		cancel()
		<-ctx.Done()
		return ctx.Err()
	}, func(contracts.ShapedGlyph) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want cancellation", err)
	}
	select {
	case <-s.exited:
	default:
		t.Fatal("returned before producer exited")
	}
}

func TestConsumeShaperRejectsBeforeLaunch(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		s := newPipelineShaper()
		ctx, cancel := context.WithCancel(t.Context())
		want := contracts.ErrIllegalArguments
		if canceled {
			cancel()
			want = context.Canceled
		} else {
			s.out = nil
		}
		called := false
		err := consumeShaper(ctx, s, func(context.Context) error {
			called = true
			return nil
		}, func(contracts.ShapedGlyph) error { return nil })
		cancel()
		if called || !errors.Is(err, want) {
			t.Fatalf("producer called=%v, error %v; want %v", called, err, want)
		}
	}
}
