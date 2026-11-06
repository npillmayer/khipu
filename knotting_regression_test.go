package khipu

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

// boundaryTestShaper deliberately omits scalars or combines all input under
// one anchor. These are valid coordinate patterns, not a typography model.
type boundaryTestShaper struct {
	out    chan contracts.ShapedGlyph
	base   uint32
	mode   string
	err    error
	closed bool
}

func (s *boundaryTestShaper) Glyphs() <-chan contracts.ShapedGlyph { return s.out }
func (s *boundaryTestShaper) Err() error                           { return s.err }
func (s *boundaryTestShaper) SetInitialCluster(n uint32) error     { s.base = n; return nil }
func (s *boundaryTestShaper) Finish() error {
	if !s.closed {
		close(s.out)
		s.closed = true
	}
	return s.err
}
func (s *boundaryTestShaper) Shape(ctx context.Context, _ contracts.ShapeParams, r contracts.RuneReader, _ contracts.ShapeBufferOptions) error {
	for i := uint32(0); ; i++ {
		c, _, err := r.ReadRune()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			s.err = err
			s.Finish()
			return err
		}
		if s.mode == "silent" || (s.mode == "first" && i > 0) || (s.mode == "omit-newline" && c == '\n') {
			continue
		}
		g := contracts.ShapedGlyph{Cluster: s.base + i, GlyphID: uint32(c) + 1, XAdvance: 500}
		select {
		case s.out <- g:
		case <-ctx.Done():
			s.err = ctx.Err()
			s.Finish()
			return s.err
		}
	}
}

func TestPublicKnottingMandatoryBreakPolicy(t *testing.T) {
	for _, tc := range []struct {
		text, mode string
		reject     bool
	}{
		{"a\nb", "first", true},          // mandatory boundary inside inferred cluster
		{"a\nb", "omit-newline", false},  // boundary at the next anchor survives
		{"\na", "omit-newline", true},    // leading glyphless mandatory break
		{"a\n\nb", "omit-newline", true}, // repeated mandatory breaks
		{"\n\n", "silent", true},
		{"abc", "silent", false},
		{"a b", "first", false}, // optional interior break may be suppressed
		{"a\n", "first", false}, // final boundary belongs to the trailer
	} {
		t.Run(tc.mode+tc.text, func(t *testing.T) {
			owner := NewKhipukamayuq(Config{Streaming: &StreamingConfig{
				Resolver: helpers.DefaultStyleResolver{}, Defaults: preparationDefaults(),
				NewShaper: func() (contracts.StreamingShaper, error) {
					return &boundaryTestShaper{out: make(chan contracts.ShapedGlyph), mode: tc.mode}, nil
				},
			}})
			got, err := owner.EncodeParagraph(t.Context(), preparationParagraph(t, tc.text))
			if tc.reject {
				if got != nil || !errors.Is(err, ErrUnrepresentableMandatoryBreak) {
					t.Fatalf("%v %v", got, err)
				}
			} else if err != nil || got == nil {
				t.Fatalf("%v %v", got, err)
			}
		})
	}
}

func TestTrailingGapDoesNotEraseMandatoryEndpoint(t *testing.T) {
	a, g := candidateFixture(t, "ab")
	// Split metadata into two runs, the second of which emits no glyphs.
	first := a.runs[0]
	second := first
	first.source = runSourceRange{0, 1, 0, 1}
	second.source = runSourceRange{1, 2, 1, 2}
	a.runs = []preparedRun{first, second}
	a.boundaries[0].penalty = MaxMerit
	g, err := newGlyphClusterAssembler(a.runs, a.boundaries)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = g.push(contracts.ShapedGlyph{Cluster: 0})
	c, _ := g.finish()
	if err := a.push(c); err != nil {
		t.Fatal(err)
	}
	if got, err := a.finish(); got != nil || !errors.Is(err, ErrUnrepresentableMandatoryBreak) {
		t.Fatalf("lost trailing break: %v %v", got, err)
	}
}

func TestPublicStyleBoundaryDoesNotCreateBreak(t *testing.T) {
	p := preparationParagraph(t, "ab")
	if _, err := p.Style(intersectionStyle("large"), 1, 2); err != nil {
		t.Fatal(err)
	}
	resolver := preparationResolver(func(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
		if s != nil {
			d.EmSize = 20 * dimen.PT
		}
		return d, nil
	})
	owner := NewKhipukamayuq(Config{Streaming: &StreamingConfig{
		Resolver: resolver, Defaults: preparationDefaults(),
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(1) },
	}})
	k, err := owner.EncodeParagraph(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if len(k.Kind) != 3 || k.Penalty[0] != MaxPenalty || k.W[0] != 5*dimen.PT || k.W[1] != 10*dimen.PT || len(k.shaping.contexts) != 2 {
		t.Fatal("style boundary lost parameters or authorized a break", k)
	}
}
