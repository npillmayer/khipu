package khipu

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

func fragmentFixture(t *testing.T) (discretionaryInput, runParameters) {
	t.Helper()
	in, err := prepareDiscretionaryInput("aéz", runSourceRange{100, 104, 20, 23}, HyphenationFragment{
		PreBreak: "aé-", PostBreak: "z", SourceSplit: SourceSplit{3, true},
	})
	if err != nil {
		t.Fatal(err)
	}
	d := preparationDefaults()
	scale, err := d.Font.Scale(d.EmSize)
	if err != nil {
		t.Fatal(err)
	}
	return in, runParameters{shaping: contracts.ShapeParams{Font: d.Font}, emSize: d.EmSize, scale: scale}
}

func TestShapeDiscretionaryInput(t *testing.T) {
	for _, capacity := range []int{0, 64} {
		in, params := fragmentFixture(t)
		calls := 0
		factory := func() (contracts.StreamingShaper, error) {
			calls++
			return helpers.NewSyntheticStreamingShaper(capacity)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		got, err := shapeDiscretionaryInput(ctx, in, params, params, factory, contracts.ShapeBufferOptions{})
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if calls != 2 || got.split != in.split || got.source != in.source || got.pre.input != in.pre || got.post.input != in.post {
			t.Fatalf("lost provenance/fresh streams: %+v, calls %d", got, calls)
		}
		if len(got.pre.glyphs) != 3 || len(got.post.glyphs) != 1 || got.post.glyphs[0].Cluster != 0 ||
			got.pre.widths != (candidateWidths{15 * dimen.PT, 15 * dimen.PT, 15 * dimen.PT}) || got.post.widths.W != 5*dimen.PT {
			t.Fatalf("incorrect local glyphs/metrics: %+v", got)
		}
	}
}

func TestShapeDiscretionaryFailuresAreAtomic(t *testing.T) {
	failure := errors.New("fragment backend failed")
	for _, mode := range []string{"factory", "nil", "base", "shape", "finish", "terminal", "anchor"} {
		t.Run(mode, func(t *testing.T) {
			in, params := fragmentFixture(t)
			calls := 0
			s := newPipelineShaper()
			want := failure
			switch mode {
			case "nil":
				want = contracts.ErrIllegalArguments
			case "base":
				s.baseErr = failure
			case "shape":
				s.shapeErr = failure
			case "finish":
				s.finishErr = failure
			case "terminal":
				s.terminalErr = failure
			case "anchor":
				s.badAnchor, s.wait = true, true
				want = contracts.ErrIllegalArguments
			}
			factory := func() (contracts.StreamingShaper, error) {
				calls++
				if calls == 1 {
					return helpers.NewSyntheticStreamingShaper(0)
				}
				if mode == "factory" {
					return nil, failure
				}
				if mode == "nil" {
					return nil, nil
				}
				return s, nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := shapeDiscretionaryInput(ctx, in, params, params, factory, contracts.ShapeBufferOptions{})
			if got != nil || !errors.Is(err, want) {
				t.Fatalf("partial result or wrong error: %+v %v", got, err)
			}
			if mode == "shape" || mode == "finish" || mode == "terminal" || mode == "anchor" {
				select {
				case <-s.exited:
				default:
					t.Fatal("producer not joined")
				}
			}
		})
	}
}

func TestShapeDiscretionaryRejectsFeaturesBeforeFactory(t *testing.T) {
	in, params := fragmentFixture(t)
	post := params
	post.shaping.Features = []contracts.ShapeFeature{{End: 3}}
	calls := 0
	factory := func() (contracts.StreamingShaper, error) { calls++; return nil, nil }
	got, err := shapeDiscretionaryInput(context.Background(), in, params, post, factory, contracts.ShapeBufferOptions{})
	if got != nil || !errors.Is(err, contracts.ErrIllegalArguments) || calls != 0 {
		t.Fatalf("%v %v calls %d", got, err, calls)
	}
}

func TestShapeDiscretionaryCancellation(t *testing.T) {
	in, params := fragmentFixture(t)
	s := newPipelineShaper()
	s.wait = true
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		select {
		case <-s.entered:
			cancel()
		case <-ctx.Done():
		}
		close(done)
	}()
	got, err := shapeDiscretionaryInput(ctx, in, params, params, func() (contracts.StreamingShaper, error) { return s, nil }, contracts.ShapeBufferOptions{})
	<-done
	if got != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("%v %v", got, err)
	}
	select {
	case <-s.exited:
	default:
		t.Fatal("producer not joined")
	}
}

func TestShapeDiscretionaryEmptyAndOmittedGlyphs(t *testing.T) {
	in, params := fragmentFixture(t)
	in.pre = replacementInput{provenance: in.pre.provenance}
	// The scripted backend omits all glyphs even for nonempty post-break text.
	got, err := shapeDiscretionaryInput(context.Background(), in, params, params, func() (contracts.StreamingShaper, error) { return newPipelineShaper(), nil }, contracts.ShapeBufferOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.pre.glyphs) != 0 || len(got.post.glyphs) != 0 || got.post.widths.W != 0 || got.post.input != in.post {
		t.Fatalf("%+v", got)
	}
}
