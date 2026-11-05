package khipu

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/helpers"
)

// collectProduced models the calling execution path, not a second consumer
// goroutine. Completion and channel closure may arrive in either order.
func collectProduced(ctx context.Context, p *styled.Paragraph, runs []preparedRun, s contracts.StreamingShaper) ([]contracts.ShapedGlyph, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- producePreparedRuns(ctx, p, runs, s, contracts.ShapeBufferOptions{}) }()
	output := s.Glyphs()
	var glyphs []contracts.ShapedGlyph
	for output != nil || done != nil {
		select {
		case g, ok := <-output:
			if !ok {
				output = nil
			} else {
				glyphs = append(glyphs, g)
			}
		case err := <-done:
			done = nil
			if err != nil {
				return nil, err
			}
		case <-ctx.Done():
			cancel()
			if done != nil {
				<-done
			}
			return nil, ctx.Err()
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return glyphs, nil
}

func TestProducePreparedRuns(t *testing.T) {
	for _, tc := range []struct {
		text string
		ids  []uint32
	}{
		{"aéz", []uint32{0, 1, 2}},
		{"a\u2067אב\u2069z", []uint32{0, 2, 3, 5}},
		{"", nil}, {"\u2066\u2069", nil},
	} {
		for _, capacity := range []int{0, 16} {
			p := preparationParagraph(t, tc.text)
			runs, err := prepareParagraphRuns(p, helpers.DefaultStyleResolver{}, preparationDefaults())
			if err != nil {
				t.Fatal(err)
			}
			s, _ := helpers.NewSyntheticStreamingShaper(capacity)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			glyphs, err := collectProduced(ctx, p, runs, s)
			cancel()
			if err != nil {
				t.Fatal(err)
			}
			var ids []uint32
			for _, g := range glyphs {
				ids = append(ids, g.Cluster)
				if g.XAdvance != 500 {
					t.Fatal(g)
				}
			}
			if !reflect.DeepEqual(ids, tc.ids) {
				t.Fatalf("%q: %v want %v", tc.text, ids, tc.ids)
			}
		}
	}
}

// These wrappers inject lifecycle failures without pretending that they are
// conforming backend output. SetInitialCluster failure leaves Glyphs open.
type producerFailure struct {
	*helpers.SyntheticStreamingShaper
	baseErr, finishErr error
}

func (s producerFailure) SetInitialCluster(n uint32) error {
	if s.baseErr != nil {
		return s.baseErr
	}
	return s.SyntheticStreamingShaper.SetInitialCluster(n)
}
func (s producerFailure) Finish() error {
	if s.finishErr != nil {
		return s.finishErr
	}
	return s.SyntheticStreamingShaper.Finish()
}

func TestProducerFailures(t *testing.T) {
	p := preparationParagraph(t, "a")
	runs, err := prepareParagraphRuns(p, helpers.DefaultStyleResolver{}, preparationDefaults())
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("injected failure")
	for _, finish := range []bool{false, true} {
		s, _ := helpers.NewSyntheticStreamingShaper(0)
		wrapper := producerFailure{SyntheticStreamingShaper: s, baseErr: failure}
		if finish {
			wrapper.baseErr = nil
			wrapper.finishErr = failure
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		got, err := collectProduced(ctx, p, runs, wrapper)
		cancel()
		if got != nil || !errors.Is(err, failure) {
			t.Fatalf("%v %v", got, err)
		}
	}
	// Synthetic shaping rejects features after accepting Shape, closing output.
	runs[0].params.shaping.Features = []contracts.ShapeFeature{{Enabled: true}}
	s, _ := helpers.NewSyntheticStreamingShaper(0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if got, err := collectProduced(ctx, p, runs, s); got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatalf("%v %v", got, err)
	}
}

func TestProducerClusterLimits(t *testing.T) {
	p := preparationParagraph(t, "a")
	runs, err := prepareParagraphRuns(p, helpers.DefaultStyleResolver{}, preparationDefaults())
	if err != nil {
		t.Fatal(err)
	}
	// Artificial metadata exercises narrowing without allocating billions of scalars.
	runs[0].source.scalarStart = math.MaxUint32
	runs[0].source.scalarEnd = uint64(math.MaxUint32) + 1
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, _ := helpers.NewSyntheticStreamingShaper(0)
	got, err := collectProduced(ctx, p, runs, s)
	if err != nil || len(got) != 1 || got[0].Cluster != math.MaxUint32 {
		t.Fatalf("%v %v", got, err)
	}
	runs[0].source.scalarEnd++
	s, _ = helpers.NewSyntheticStreamingShaper(0)
	if got, err := collectProduced(ctx, p, runs, s); got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
		t.Fatalf("%v %v", got, err)
	}
}

func TestProducerCancellationAfterConsumerStops(t *testing.T) {
	p := preparationParagraph(t, "abcdef")
	runs, err := prepareParagraphRuns(p, helpers.DefaultStyleResolver{}, preparationDefaults())
	if err != nil {
		t.Fatal(err)
	}
	s, _ := helpers.NewSyntheticStreamingShaper(0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- producePreparedRuns(ctx, p, runs, s, contracts.ShapeBufferOptions{}) }()
	// Receipt synchronizes with an active Shape. Stop draining, then cancel;
	// no sleep or assumption about scheduling the next send is needed.
	select {
	case <-s.Glyphs():
	case <-time.After(5 * time.Second):
		t.Fatal("no first glyph")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("producer did not terminate")
	}
}
