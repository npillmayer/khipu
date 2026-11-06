package khipu_test

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/helpers"
)

type encodingResolver func(styled.Style, contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error)

func (f encodingResolver) ResolveStyle(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
	return f(s, d)
}

// encodingStreamingConfig explicitly interprets the fixture's bold style as
// metrically identical. Tests of changed font size override this resolver.
func encodingStreamingConfig() *khipu.StreamingConfig {
	return &khipu.StreamingConfig{
		NewShaper: func() (contracts.StreamingShaper, error) { return helpers.NewSyntheticStreamingShaper(0) },
		Resolver: encodingResolver(func(s styled.Style, d contracts.ResolvedRunStyle) (contracts.ResolvedRunStyle, error) {
			if s != nil && s != encodingStyle("bold") {
				return d, fmt.Errorf("unexpected fixture style %v", s)
			}
			return d, nil
		}),
		Defaults: contracts.ResolvedRunStyle{Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT, Spacing: contracts.SpacingElastic},
	}
}

// encodingScriptStream publishes scripted scalar anchors/advances after reading
// a run. It models unusual clusters or a failure after partial output, while
// retaining channel closure and cancellation semantics of a real backend.
// Producer calls are serial; Err may be queried by the consumer concurrently.
type encodingScriptStream struct {
	records []contracts.ShapedGlyph
	failure error
	output  chan contracts.ShapedGlyph
	base    uint32
	input   string
	params  contracts.ShapeParams
	mu      sync.Mutex
	err     error
	closed  bool
}

func newEncodingScriptStream(records []contracts.ShapedGlyph, failure error) *encodingScriptStream {
	return &encodingScriptStream{records: records, failure: failure, output: make(chan contracts.ShapedGlyph)}
}

func (s *encodingScriptStream) Shape(ctx context.Context, params contracts.ShapeParams, input contracts.RuneReader, _ contracts.ShapeBufferOptions) error {
	s.params = params
	var runes []rune
	for {
		r, _, err := input.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return s.finish(err)
		}
		runes = append(runes, r)
	}
	s.input += string(runes)
	for _, g := range s.records {
		g.Cluster += s.base
		select {
		case s.output <- g:
		case <-ctx.Done():
			return s.finish(ctx.Err())
		}
	}
	s.base += uint32(len(runes))
	if s.failure != nil {
		return s.finish(s.failure)
	}
	return nil
}

func (s *encodingScriptStream) SetInitialCluster(base uint32) error  { s.base = base; return nil }
func (s *encodingScriptStream) Glyphs() <-chan contracts.ShapedGlyph { return s.output }
func (s *encodingScriptStream) Finish() error                        { return s.finish(nil) }
func (s *encodingScriptStream) Err() error                           { s.mu.Lock(); defer s.mu.Unlock(); return s.err }
func (s *encodingScriptStream) finish(err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.err = err
		s.closed = true
		close(s.output)
	}
	return s.err
}
