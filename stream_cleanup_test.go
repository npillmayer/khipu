package khipu

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

// cleanupStream retains cancellation like the real adapters. Its malformed
// output forces consumer-side failure before the producer waits for cancellation.
// Finish can also report an independent resource-release failure.
type cleanupStream struct {
	mu                       sync.Mutex
	out                      chan contracts.ShapedGlyph
	exited                   chan struct{}
	terminal, release        error
	finishes                 int
	closed, finishBeforeExit bool
}

func newCleanupStream(release error) *cleanupStream {
	return &cleanupStream{out: make(chan contracts.ShapedGlyph), exited: make(chan struct{}), release: release}
}
func (*cleanupStream) CheckFace(face contracts.FontFace) error {
	return new(helpers.SyntheticStreamingShaper).CheckFace(face)
}
func (*cleanupStream) SetInitialCluster(uint32) error         { return nil }
func (s *cleanupStream) Glyphs() <-chan contracts.ShapedGlyph { return s.out }
func (s *cleanupStream) Err() error                           { s.mu.Lock(); defer s.mu.Unlock(); return s.terminal }
func (s *cleanupStream) Shape(ctx context.Context, _ contracts.ShapeParams, _ contracts.RuneReader, _ contracts.ShapeBufferOptions) error {
	defer close(s.exited)
	select {
	case s.out <- contracts.ShapedGlyph{Cluster: 100, XAdvance: 500}:
	case <-ctx.Done():
	}
	<-ctx.Done()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.terminal = fmt.Errorf("backend stopped: %w", ctx.Err())
	s.closed = true
	close(s.out)
	return s.terminal
}
func (s *cleanupStream) Finish() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finishes++
	select {
	case <-s.exited:
	default:
		s.finishBeforeExit = true
	}
	if !s.closed {
		close(s.out)
		s.closed = true
	}
	// A wrapper around joined causes must not hide the independent release error
	// or expose the repeated terminal error through errors.Is.
	return fmt.Errorf("finish: %w", errors.Join(s.terminal, s.release))
}

func TestStreamCleanupPreservesConsumerFailure(t *testing.T) {
	release := errors.New("resource release failed")
	for _, path := range []string{"base", "lazy pre", "lazy post"} {
		for _, extra := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cleanup=%t", path, extra), func(t *testing.T) {
				var cleanup error
				if extra {
					cleanup = fmt.Errorf("native handle: %w", release)
				}
				s := newCleanupStream(cleanup)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				var err error
				if path == "base" {
					result, e := streamCandidateParagraph(ctx, preparationParagraph(t, "abc"), helpers.DefaultStyleResolver{}, preparationDefaults(), *newParameters(), KnotCore{MaxW: dimen.Fill}, s, contracts.ShapeBufferOptions{})
					err = e
					if result != nil {
						t.Fatal("published partial base result")
					}
				} else {
					in, params := fragmentFixture(t)
					calls := 0
					result, e := shapeDiscretionaryInput(ctx, in, params, params, func() (contracts.StreamingShaper, error) {
						calls++
						if path == "lazy pre" || calls == 2 {
							return s, nil
						}
						return helpers.NewSyntheticStreamingShaper(0)
					}, contracts.ShapeBufferOptions{})
					err = e
					if result != nil {
						t.Fatal("published partial replacement pair")
					}
					wantCalls := 1
					if path == "lazy post" {
						wantCalls = 2
					}
					if calls != wantCalls {
						t.Fatal("wrong replacement stream", calls)
					}
				}
				if !errors.Is(err, contracts.ErrIllegalArguments) {
					t.Fatal("lost consumer error", err)
				}
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("internal cancellation escaped", err)
				}
				if errors.Is(err, release) != extra {
					t.Fatal("lost independent cleanup failure", err)
				}
				if ctx.Err() != nil {
					t.Fatal("cancelled caller context")
				}
				if !errors.Is(s.Err(), context.Canceled) {
					t.Fatal("fixture did not retain cancellation")
				}
				if s.finishes != 1 || s.finishBeforeExit {
					t.Fatal("cleanup did not follow producer join", s.finishes, s.finishBeforeExit)
				}
				if _, ok := <-s.Glyphs(); ok {
					t.Fatal("stream not closed")
				}
			})
		}
	}
}

// An idle stream for exercising error combinations without manufacturing glyphs.
// Err changes during Finish to prove that cleanup is compared with the terminal
// error captured before resource release, not with an updated Err value.
type cleanupErrors struct {
	contracts.StreamingShaper
	terminal, final, finish error
	calls                   int
}

func (s *cleanupErrors) Err() error    { return s.terminal }
func (s *cleanupErrors) Finish() error { s.calls++; s.terminal = s.final; return s.finish }

func TestFinishStreamPreservesIndependentCauses(t *testing.T) {
	primary := errors.New("consumer failure")
	release := errors.New("release failure")
	backend := errors.New("backend failure")
	for _, tc := range []struct {
		name                                                 string
		primary, terminal, finish, final                     error
		wantPrimary, wantRelease, wantCancelled, wantBackend bool
	}{
		{name: "successful cleanup"},
		{name: "terminal without primary", terminal: backend, finish: fmt.Errorf("finish: %w", backend), wantBackend: true},
		{name: "release without primary", finish: release, wantRelease: true},
		{name: "caller cancellation", primary: context.Canceled, terminal: context.Canceled, finish: context.Canceled, wantCancelled: true},
		{name: "independent cancellation", primary: primary, finish: context.Canceled, wantPrimary: true, wantCancelled: true},
		{name: "repeated backend failure", primary: primary, terminal: backend, finish: fmt.Errorf("finish: %w", backend), wantPrimary: true},
		{name: "joined release", primary: primary, terminal: context.Canceled, finish: errors.Join(context.Canceled, fmt.Errorf("close handle: %w", release)), wantPrimary: true, wantRelease: true},
		{name: "finish updates terminal", primary: primary, terminal: context.Canceled, finish: errors.Join(context.Canceled, release), final: errors.Join(context.Canceled, release), wantPrimary: true, wantRelease: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &cleanupErrors{terminal: tc.terminal, finish: tc.finish, final: tc.final}
			got := finishStream(s, tc.primary)
			if errors.Is(got, primary) != tc.wantPrimary || errors.Is(got, release) != tc.wantRelease || errors.Is(got, context.Canceled) != tc.wantCancelled || errors.Is(got, backend) != tc.wantBackend {
				t.Fatal("incorrect causes", got)
			}
			if tc.primary == nil && tc.finish == nil && got != nil {
				t.Fatal(got)
			}
			if s.calls != 1 {
				t.Fatal("cleanup skipped or repeated", s.calls)
			}
		})
	}
}
