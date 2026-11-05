package khipu

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

func TestStreamCandidateParagraph(t *testing.T) {
	for _, text := range []string{"alpha beta", "a\u2067אב\u2069z", "", "\u2066\u2069"} {
		for _, capacity := range []int{0, 64} {
			p := preparationParagraph(t, text)
			d := preparationDefaults()
			d.Spacing = contracts.SpacingElastic
			s, _ := helpers.NewSyntheticStreamingShaper(capacity)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			got, err := streamCandidateParagraph(ctx, p, helpers.DefaultStyleResolver{}, d, *newParameters(), KnotCore{MaxW: dimen.Fill}, s, contracts.ShapeBufferOptions{})
			cancel()
			if err != nil {
				t.Fatal(err)
			}
			if got.paragraph != p || len(got.knots) == 0 {
				t.Fatal("lost result metadata")
			}
			last := got.knots[len(got.knots)-1].core
			if last.Penalty != MaxMerit || last.Flags != 0 || last.Pos != uint64(len(text)) {
				t.Fatal(last)
			}
			glyphCount := 0
			for _, k := range got.knots {
				for _, c := range k.clusters {
					glyphCount += len(c.glyphs)
					if c.runIndex >= len(got.runs) || c.source.scalarEnd > uint64(len(got.boundaries)) {
						t.Fatal("dangling source association")
					}
				}
			}
			want := 0
			for _, r := range got.runs {
				want += int(r.source.scalarEnd - r.source.scalarStart)
			}
			if glyphCount != want {
				t.Fatalf("lost buffered glyphs: %d != %d", glyphCount, want)
			}
			if text == "alpha beta" && len(got.knots) != 4 {
				t.Fatal("unexpected knotting", got.knots)
			}
		}
	}
}

// A deliberately scripted backend for orchestration failure tests. All terminal
// state is configured before use; channels synchronize Shape completion.
type pipelineShaper struct {
	out                                       chan contracts.ShapedGlyph
	entered                                   chan struct{}
	exited                                    chan struct{}
	baseErr, shapeErr, finishErr, terminalErr error
	badAnchor                                 bool
	wait                                      bool
}

func (s *pipelineShaper) Glyphs() <-chan contracts.ShapedGlyph { return s.out }
func (s *pipelineShaper) Err() error                           { return s.terminalErr }
func (s *pipelineShaper) SetInitialCluster(uint32) error       { return s.baseErr }
func (s *pipelineShaper) Finish() error {
	if s.finishErr != nil {
		return s.finishErr
	}
	close(s.out)
	return nil
}
func (s *pipelineShaper) Shape(ctx context.Context, _ contracts.ShapeParams, _ contracts.RuneReader, _ contracts.ShapeBufferOptions) error {
	close(s.entered)
	defer close(s.exited)
	if s.badAnchor {
		select {
		case s.out <- contracts.ShapedGlyph{Cluster: 100}:
		case <-ctx.Done():
			close(s.out)
			return ctx.Err()
		}
	}
	if s.wait {
		<-ctx.Done()
		close(s.out)
		return ctx.Err()
	}
	if s.shapeErr != nil {
		close(s.out)
		return s.shapeErr
	}
	return nil
}
func newPipelineShaper() *pipelineShaper {
	return &pipelineShaper{out: make(chan contracts.ShapedGlyph), entered: make(chan struct{}), exited: make(chan struct{})}
}

func TestStreamCandidateFailures(t *testing.T) {
	failure := errors.New("pipeline test failure")
	for _, mode := range []string{"base", "shape", "finish", "terminal", "consumer"} {
		t.Run(mode, func(t *testing.T) {
			s := newPipelineShaper()
			want := failure
			switch mode {
			case "base":
				s.baseErr = failure
			case "shape":
				s.shapeErr = failure
			case "finish":
				s.finishErr = failure
			case "terminal":
				s.terminalErr = failure
			case "consumer":
				s.badAnchor = true
				s.wait = true
				want = contracts.ErrIllegalArguments
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := streamCandidateParagraph(ctx, preparationParagraph(t, "abc"), helpers.DefaultStyleResolver{}, preparationDefaults(), *newParameters(), KnotCore{MaxW: dimen.Fill}, s, contracts.ShapeBufferOptions{})
			if got != nil || !errors.Is(err, want) {
				t.Fatalf("%v %v", got, err)
			}
			if mode != "base" {
				select {
				case <-s.exited:
				default:
					t.Fatal("returned before producer exited")
				}
			}
		})
	}
}

func TestStreamCandidateCancellation(t *testing.T) {
	s := newPipelineShaper()
	s.wait = true
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Cancel only after Shape starts, with no sleeps or scheduler assumptions.
	cancelled := make(chan struct{})
	go func() {
		select {
		case <-s.entered:
			cancel()
		case <-ctx.Done():
		}
		close(cancelled)
	}()
	got, err := streamCandidateParagraph(ctx, preparationParagraph(t, "abc"), helpers.DefaultStyleResolver{}, preparationDefaults(), Params{}, KnotCore{}, s, contracts.ShapeBufferOptions{})
	<-cancelled
	if got != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("%v %v", got, err)
	}
	select {
	case <-s.exited:
	default:
		t.Fatal("producer not joined")
	}
}

func TestStreamCandidatePreparationAndFinalizationFailures(t *testing.T) {
	for _, mode := range []string{"prepare", "finalize", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			p := preparationParagraph(t, "a")
			d := preparationDefaults()
			if mode == "prepare" {
				d.Font = nil
			}
			if mode == "finalize" {
				d.EmSize = dimen.DU(math.MaxInt32)
			}
			// Make final source a space so its stretch exceeds finite DU range.
			if mode == "finalize" {
				p = preparationParagraph(t, " ")
				d.Spacing = contracts.SpacingElastic
			}
			s, _ := helpers.NewSyntheticStreamingShaper(1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			params := *newParameters()
			if mode == "finalize" {
				params.SpaceStretchPercent = 200
			}
			got, err := streamCandidateParagraph(ctx, p, helpers.DefaultStyleResolver{}, d, params, KnotCore{}, s, contracts.ShapeBufferOptions{})
			if got != nil || err == nil {
				t.Fatalf("published failed result: %v %v", got, err)
			}
		})
	}
}
