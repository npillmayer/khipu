package khipu

import (
	"context"
	"errors"
	"fmt"

	"github.com/npillmayer/cords/styled"
)

// ErrStreamingDiscretionariesUnsupported reports missing streaming configuration
// when an explicit alternative requires reshaping. Never fall back to metric
// shaping. Automatic streaming discovery is disabled and returns no candidates.
var ErrStreamingDiscretionariesUnsupported = errors.New("khipu: streaming discretionary shaping is unavailable")

// ErrUnrepresentableMandatoryBreak means source analysis requires a break that
// cannot be retained without splitting a shaped cluster or representing a
// glyphless blank line. Streaming encoding rejects such paragraphs atomically;
// it never downgrades a mandatory break to an optional or prohibited one.
var ErrUnrepresentableMandatoryBreak = errors.New("khipu: mandatory break cannot be represented by shaped knots")

// EncodeParagraphStreaming shapes and publishes a logical-order SOA paragraph
// with retained textbox glyphs, cluster provenance and shaping contexts. It
// requires Config.Streaming and uses a fresh factory-created stream per call.
// Glue retains metrics/source metadata, not glyphs. Space elasticity comes from
// Params and resolved spacing; the metric path's KnotStyler is not invoked.
//
// Source text/styles must remain stable during this call and later reshaping.
// The owner is not safe for concurrent operations. Errors return no paragraph;
// any launched producer is cancelled and joined before returning. Synchronous
// preparation and factory creation are not interrupted mid-call by cancellation.
// A backend blocked in a source read retains its documented cancellation limits.
// Freezing, visual ordering and lazy discretionary reshaping are not performed.
// Mandatory breaks inside clusters or unsupported glyphless ranges return
// ErrUnrepresentableMandatoryBreak. Optional opportunities inside clusters are
// suppressed. The final source boundary is represented by the paragraph trailer.
// U+00AD is filtered before shaping, with returned anchors restored to original
// source coordinates. Its location is retained, but ordinary breaks there are
// prohibited. Configure this owner as the breaker's DiscretionaryProvider to
// prepare and select explicit replacement fragments lazily during linebreaking.
func (kq *Khipukamayuq) EncodeParagraphStreaming(ctx context.Context, para *styled.Paragraph) (*Khipu, error) {
	if kq == nil {
		return nil, ErrVoidKhipukamayuq
	}
	if ctx == nil || para == nil {
		return nil, ErrIllegalArguments
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfg := kq.env.streaming
	if cfg == nil || cfg.NewShaper == nil || nilRunService(cfg.Resolver) {
		return nil, fmt.Errorf("%w: streaming configuration requires a factory and resolver", ErrIllegalArguments)
	}
	shaper, err := cfg.NewShaper()
	if err != nil {
		return nil, fmt.Errorf("khipu: creating streaming shaper: %w", err)
	}
	if nilRunService(shaper) {
		return nil, fmt.Errorf("%w: factory returned nil streaming shaper", ErrIllegalArguments)
	}
	candidates, err := streamCandidateParagraph(ctx, para, cfg.Resolver, cfg.Defaults, *kq.env.params, kq.env.parFillSkip, shaper, cfg.BufferOptions)
	if err != nil {
		return nil, err
	}
	k, err := kq.publishCandidateParagraph(candidates)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		unwireKhipu(k)
		return nil, err
	}
	return k, nil
}
