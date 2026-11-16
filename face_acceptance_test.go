package khipu

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

// A second representation with deliberately noncomparable storage. The test
// backend uses synthetic units for both representations without metric queries.
type acceptanceFace struct {
	helpers.SyntheticFont
	data []byte
}
type acceptanceStream struct {
	contracts.StreamingShaper
	check                           func(contracts.FontFace) error
	checks, shapes, bases, finishes int
	cleanup                         error
	nilOutput                       bool
}

func acceptanceTestStream() *acceptanceStream {
	s, _ := helpers.NewSyntheticStreamingShaper(0)
	return &acceptanceStream{StreamingShaper: s}
}
func (s *acceptanceStream) CheckFace(f contracts.FontFace) error {
	s.checks++
	if s.check != nil {
		return s.check(f)
	}
	if _, ok := f.(acceptanceFace); ok {
		return nil
	}
	return s.StreamingShaper.CheckFace(f)
}
func (s *acceptanceStream) Shape(ctx context.Context, p contracts.ShapeParams, r contracts.RuneReader, o contracts.ShapeBufferOptions) error {
	s.shapes++
	if _, ok := p.Font.(acceptanceFace); ok {
		p.Font = helpers.NewSyntheticFont()
	}
	return s.StreamingShaper.Shape(ctx, p, r, o)
}
func (s *acceptanceStream) SetInitialCluster(n uint32) error {
	s.bases++
	return s.StreamingShaper.SetInitialCluster(n)
}
func (s *acceptanceStream) Glyphs() <-chan contracts.ShapedGlyph {
	if s.nilOutput {
		return nil
	}
	return s.StreamingShaper.Glyphs()
}
func (s *acceptanceStream) Finish() error {
	s.finishes++
	return errors.Join(s.StreamingShaper.Finish(), s.cleanup)
}
func assertAbandoned(t *testing.T, s *acceptanceStream) {
	t.Helper()
	if s.finishes != 1 || s.shapes != 0 || s.bases != 0 {
		t.Fatalf("preflight lifecycle: %+v", s)
	}
	select {
	case _, ok := <-s.StreamingShaper.Glyphs():
		if ok {
			t.Fatal("preflight emitted glyph")
		}
	default:
		t.Fatal("abandoned stream still open")
	}
}

func TestFacePreflightChecksAllResolvedRunsBeforeProduction(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(map[bool]string{false: "multiple representations", true: "later rejection"}[reject], func(t *testing.T) {
			p := preparationParagraph(t, "ab")
			if _, err := p.Style(intersectionStyle("second"), 1, 2); err != nil {
				t.Fatal(err)
			}
			resolver := preparationResolver(func(style styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
				if style != nil {
					d.Font = acceptanceFace{data: []byte{1}}
				}
				return d, nil
			})
			s := acceptanceTestStream()
			if reject {
				s.check = s.StreamingShaper.CheckFace
			}
			got, err := streamCandidateParagraph(t.Context(), p, resolver, preparationDefaults(), *newParameters(), KnotCore{MaxW: dimen.Fill}, s, contracts.ShapeBufferOptions{})
			if s.checks != 2 {
				t.Fatal("did not preflight actual runs", s.checks)
			}
			if reject {
				if got != nil || !errors.Is(err, contracts.ErrUnsupportedFontFace) {
					t.Fatal(got, err)
				}
				assertAbandoned(t, s)
			} else if err != nil || got == nil || s.shapes != 2 {
				t.Fatal(got, err, s.shapes)
			}
		})
	}
}

func TestFacePreflightCleanupAndCancellation(t *testing.T) {
	failure := errors.New("acceptance failed")
	cleanup := errors.New("cleanup failed")
	for _, mode := range []string{"reject", "cancel check", "cancel entry", "preparation", "nil output"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			s := acceptanceTestStream()
			resolver := contracts.StyleResolver(helpers.DefaultStyleResolver{})
			want := failure
			switch mode {
			case "reject":
				s.check = func(contracts.FontFace) error { return failure }
				s.cleanup = cleanup
			case "cancel check":
				s.check = func(contracts.FontFace) error { cancel(); return nil }
				want = context.Canceled
			case "cancel entry":
				cancel()
				want = context.Canceled
			case "preparation":
				resolver = preparationResolver(func(_ styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
					return d, failure
				})
			case "nil output":
				s.nilOutput = true
				want = contracts.ErrIllegalArguments
			}
			got, err := streamCandidateParagraph(ctx, preparationParagraph(t, "a"), resolver, preparationDefaults(), *newParameters(), KnotCore{MaxW: dimen.Fill}, s, contracts.ShapeBufferOptions{})
			if got != nil || !errors.Is(err, want) {
				t.Fatal(got, err)
			}
			if mode == "reject" && !errors.Is(err, cleanup) {
				t.Fatal("cleanup hid cause", err)
			}
			assertAbandoned(t, s)
		})
	}
}

func TestAtomicOnlySkipsFacePreflight(t *testing.T) {
	s := acceptanceTestStream()
	s.check = func(contracts.FontFace) error { t.Fatal("marker acceptance queried"); return nil }
	got, err := streamMixedCandidateParagraph(t.Context(), preparationParagraph(t, "\uFFFC"), helpers.DefaultStyleResolver{}, preparationDefaults(), *newParameters(), KnotCore{MaxW: dimen.Fill}, s, contracts.ShapeBufferOptions{}, []contracts.AtomicAnchor{{ID: 1, ByteOffset: 0}}, nil)
	if got == nil || err != nil || s.shapes != 0 || s.checks != 0 {
		t.Fatal(got, err)
	}
}

func TestLazyFacePreflightRetriesWithoutPublishingOrReshaping(t *testing.T) {
	for _, measureFirst := range []bool{true, false} {
		for _, side := range []int{1, 2} {
			t.Run(map[bool]string{true: "measure", false: "bind"}[measureFirst]+string(rune('0'+side)), func(t *testing.T) {
				o, _, baseStreams := explicitOwner([]HyphenationFragment{{PreBreak: "-"}})
				p := intrinsicPrepare(t, o, "ab\u00adcd", PrepareOptions{Hyphenation: ExplicitHyphenation})
				base := *baseStreams
				before := slices.Clone(p.shaping.glyphs)
				prior, err := o.BindParagraph(t.Context(), p, nil)
				if err != nil {
					t.Fatal(err)
				}
				priorGlyphs := slices.Clone(prior.shaping.glyphs)
				var streams []*acceptanceStream
				o.env.streaming.NewShaper = func() (contracts.StreamingShaper, error) {
					s := acceptanceTestStream()
					if len(streams)+1 == side {
						s.check = func(contracts.FontFace) error { return contracts.ErrUnsupportedFontFace }
					}
					streams = append(streams, s)
					return s, nil
				}
				if measureFirst {
					widths, err := o.MeasureIntrinsic(t.Context(), p, MeasureRequest{})
					if widths != (IntrinsicWidths{}) || !errors.Is(err, contracts.ErrUnsupportedFontFace) {
						t.Fatal(widths, err)
					}
				} else {
					c, err := o.DiscretionaryCandidates(prior, 0)
					if len(c) != 0 || !errors.Is(err, contracts.ErrUnsupportedFontFace) {
						t.Fatal(c, err)
					}
				}
				if len(p.explicit) != 0 || len(prior.DiscretionaryCandidates) != 0 || !reflect.DeepEqual(priorGlyphs, prior.shaping.glyphs) {
					t.Fatal("published failed pair")
				}
				assertAbandoned(t, streams[side-1])
				// The next actual factory instance accepts the face; failure is retryable.
				widths, err := o.MeasureIntrinsic(t.Context(), p, MeasureRequest{})
				if err != nil || widths != (IntrinsicWidths{15 * dimen.BP, 20 * dimen.BP}) {
					t.Fatal(widths, err)
				}
				count := len(streams)
				for range 2 {
					if _, err = o.MeasureIntrinsic(t.Context(), p, MeasureRequest{}); err != nil {
						t.Fatal(err)
					}
					if c, e := o.DiscretionaryCandidates(prior, 0); e != nil || len(c) != 1 {
						t.Fatal(c, e)
					}
				}
				if len(streams) != count || *baseStreams != base || !reflect.DeepEqual(before, p.shaping.glyphs) {
					t.Fatal("cache miss or base reshaped")
				}
			})
		}
	}
}

func TestReplacementFactoryNilAndCancelledAcquisition(t *testing.T) {
	in, params := fragmentFixture(t)
	for _, typed := range []bool{false, true} {
		factory := func() (contracts.StreamingShaper, error) {
			if typed {
				return (*acceptanceStream)(nil), nil
			}
			return nil, nil
		}
		got, err := shapeDiscretionaryInput(t.Context(), in, params, params, factory, contracts.ShapeBufferOptions{})
		if got != nil || !errors.Is(err, contracts.ErrIllegalArguments) {
			t.Fatal(got, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s := acceptanceTestStream()
	got, err := shapeDiscretionaryInput(ctx, in, params, params, func() (contracts.StreamingShaper, error) { cancel(); return s, nil }, contracts.ShapeBufferOptions{})
	if got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
	assertAbandoned(t, s)
}
