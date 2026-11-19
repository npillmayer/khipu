package khipu

import (
	"context"
	"errors"
	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
)

// ErrUnrepresentableMandatoryBreak means source analysis requires a break that
// cannot be retained without splitting a shaped cluster or representing a
// glyphless blank line. Streaming encoding rejects such paragraphs atomically;
// it never downgrades a mandatory break to an optional or prohibited one.
var ErrUnrepresentableMandatoryBreak = errors.New("khipu: mandatory break cannot be represented by shaped knots")

// EncodeParagraph constructs text-only material and its discovery facility.
// Configured hyphenation enables automatic (including explicit) opportunities.
func (q *Khipukamayuq) EncodeParagraph(p *styled.Paragraph) (*Khipu, *Discovery, error) {
	mode := NoHyphenation
	if q != nil && !nilRunService(q.env.hyphenator) {
		mode = AutomaticHyphenation
	}
	return q.PrepareParagraph(ParagraphInput{Paragraph: p}, PrepareOptions{Hyphenation: mode})
}

// PrepareParagraph shapes and compiles immutable paragraph material once.
// Source/styles/services remain borrowed and stable. Errors publish no handles.
func (q *Khipukamayuq) PrepareParagraph(input ParagraphInput, options PrepareOptions) (*Khipu, *Discovery, error) {
	if q == nil {
		return nil, nil, ErrVoidKhipukamayuq
	}
	if input.Paragraph == nil || options.Hyphenation > AutomaticHyphenation ||
		(options.Hyphenation != NoHyphenation && nilRunService(q.env.hyphenator)) {
		return nil, nil, ErrIllegalArguments
	}
	cfg := q.env.streaming
	if cfg == nil || cfg.NewShaper == nil || nilRunService(cfg.Resolver) {
		return nil, nil, ErrIllegalArguments
	}
	stream, err := cfg.NewShaper()
	if err != nil {
		return nil, nil, err
	}
	if nilRunService(stream) {
		return nil, nil, ErrIllegalArguments
	}
	candidates, err := streamMixedCandidateParagraph(context.Background(), input.Paragraph, cfg.Resolver, cfg.Defaults, *q.env.params, q.env.parFillSkip, stream, cfg.BufferOptions, input.Atomics, options.AtomicBoundaries)
	if err != nil {
		return nil, nil, err
	}
	compiled, err := compileCandidateParagraph(candidates)
	if err != nil {
		return nil, nil, err
	}
	k := &Khipu{id: IncarnationID{new(incarnationToken)}, material: compiled, hyphenation: options.Hyphenation, atomicSlots: make(map[contracts.AtomicID]int)}
	for _, s := range compiled.shaping.sources {
		if s.atomic != 0 {
			k.atomicSlots[s.atomic] = len(k.atomicIDs)
			k.atomicIDs = append(k.atomicIDs, s.atomic)
		}
	}
	// A copied recipe has no backlink to the constructor or geometry provider.
	params := *q.env.params
	params.LineShape = nil
	params.LeftSkip = KnotCore{}
	params.RightSkip = KnotCore{}
	recipe := NewKhipukamayuq(Config{Params: &params, Streaming: cfg, Hyphenator: q.env.hyphenator, ParFillSkip: &q.env.parFillSkip})
	d := &Discovery{material: k, recipe: recipe, batches: make(map[int]*AlternativeSet)}
	return k, d, nil
}
