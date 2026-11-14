package khipu

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

// ErrUnsupportedPolicy identifies recognized semantics that cannot be realized
// by the current pipeline. It is distinct from malformed arguments.
var ErrUnsupportedPolicy = errors.New("khipu: unsupported policy")

// ParagraphInput is normalized styled text with opaque inline occurrences.
// Every anchor selects one U+FFFC; unanchored markers remain ordinary text.
// Text, styles and external font services are borrowed and must remain stable
// for all prepared, bound and frozen results derived from this input.
type ParagraphInput struct {
	Paragraph *styled.Paragraph
	Atomics   []contracts.AtomicAnchor
}

// BoundaryAction permits an ordinary zero-penalty break or prohibits an optional
// one. Zero and unknown values are invalid; mandatory overrides are unsupported.
type BoundaryAction uint8

const (
	AllowBoundary BoundaryAction = iota + 1
	ProhibitBoundary
)

// AtomicBoundary overrides one interior boundary immediately before or after
// an attached atom. Offsets are paragraph-relative bytes, sorted and unique.
// Omission retains Unicode policy. Mandatory breaks cannot be downgraded.
type AtomicBoundary struct {
	ByteOffset uint64
	Action     BoundaryAction
}

// HyphenationMode fixes discovery policy for all bindings of a preparation.
// Enabled modes require a configured Hyphenator; NoHyphenation is the default.
type HyphenationMode uint8

const (
	NoHyphenation HyphenationMode = iota
	ExplicitHyphenation
	AutomaticHyphenation // Includes explicit opportunities.
)

// PrepareOptions supplies paragraph-wide policies, without target geometry.
type PrepareOptions struct {
	AtomicBoundaries []AtomicBoundary
	Hyphenation      HyphenationMode
}

// PreparedParagraph is immutable horizontal material with unbound atomic
// slots, not a working Khipu or a measurement. It belongs to its creating owner.
// Operations on the same owner must be serialized. Request slices are consumed
// during preparation; no caller-owned slice is retained. There is no Close.
type PreparedParagraph struct {
	*preparedMaterial
	owner       *Khipukamayuq
	atomicCount int
	hyphenation HyphenationMode
}

// PrepareParagraph validates mixed input and shapes text exactly once. Atomics
// participate in boundary/bidi analysis, but never enter a shaping request or
// require style/font resolution. No partial result is returned on failure.
// Cancellation follows EncodeParagraph's backend limits; cancelled contexts
// take precedence over other argument errors, as in Freeze.
func (kq *Khipukamayuq) PrepareParagraph(ctx context.Context, input ParagraphInput, options PrepareOptions) (*PreparedParagraph, error) {
	if ctx == nil {
		return nil, ErrIllegalArguments
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if kq == nil {
		return nil, ErrVoidKhipukamayuq
	}
	if input.Paragraph == nil || options.Hyphenation > AutomaticHyphenation {
		return nil, ErrIllegalArguments
	}
	if options.Hyphenation != NoHyphenation && nilRunService(kq.env.hyphenator) {
		return nil, fmt.Errorf("%w: hyphenation mode requires a provider", ErrIllegalArguments)
	}
	cfg := kq.env.streaming
	if cfg == nil || cfg.NewShaper == nil || nilRunService(cfg.Resolver) {
		return nil, ErrIllegalArguments
	}
	shaper, err := cfg.NewShaper()
	if err != nil {
		return nil, fmt.Errorf("khipu: creating streaming shaper: %w", err)
	}
	if nilRunService(shaper) {
		return nil, ErrIllegalArguments
	}
	candidates, err := streamMixedCandidateParagraph(ctx, input.Paragraph, cfg.Resolver, cfg.Defaults,
		*kq.env.params, kq.env.parFillSkip, shaper, cfg.BufferOptions, input.Atomics, options.AtomicBoundaries)
	if err != nil {
		return nil, err
	}
	storage, err := compileCandidateParagraph(candidates)
	if err != nil {
		return nil, err
	}
	p := &PreparedParagraph{owner: kq, preparedMaterial: storage,
		atomicCount: len(input.Atomics), hyphenation: options.Hyphenation}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p, nil
}

// AtomicAdvance supplies a fixed signed inline contribution, including any
// caller-resolved margins. Its finite magnitude must be less than dimen.Fil.
type AtomicAdvance struct {
	ID      contracts.AtomicID
	Advance dimen.DU
}

// BindParagraph supplies exactly one advance per occurrence, in any order.
// It neither shapes nor discovers discretionaries. Working planes and caches
// are independently owned; only immutable shaping records and source are shared.
// Later discovery/arena growth on one binding cannot affect another binding or
// the prepared paragraph. Inputs are consumed without retaining caller slices.
func (kq *Khipukamayuq) BindParagraph(ctx context.Context, prepared *PreparedParagraph, atomics []AtomicAdvance) (*Khipu, error) {
	if ctx == nil {
		return nil, ErrIllegalArguments
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if kq == nil {
		return nil, ErrVoidKhipukamayuq
	}
	if prepared == nil || prepared.owner != kq || len(atomics) != prepared.atomicCount {
		return nil, ErrIllegalArguments
	}
	advances := make(map[contracts.AtomicID]dimen.DU, len(atomics))
	for _, atom := range atomics {
		_, duplicate := advances[atom.ID]
		if atom.ID == 0 || duplicate || atom.Advance <= -dimen.Fil || atom.Advance >= dimen.Fil {
			return nil, fmt.Errorf("%w: invalid or duplicate atomic advance", ErrIllegalArguments)
		}
		advances[atom.ID] = atom.Advance
	}
	k := kq.allocKhipu(len(prepared.knots))
	k.paragraph = prepared.paragraph
	k.hyphenationPolicy, k.hyphenation = true, prepared.hyphenation
	for i, core := range prepared.knots {
		if id := prepared.shaping.sources[i].atomic; id != 0 {
			w, ok := advances[id]
			if !ok {
				return nil, fmt.Errorf("%w: missing atomic advance for %d", ErrIllegalArguments, id)
			}
			core.W, core.MinW, core.MaxW = w, w, w
		}
		k.appendKnot(core)
	}
	k.boxRefs = slices.Clone(prepared.boxRefs)
	k.shaping = prepared.shaping
	k.shaping.sources = slices.Clone(prepared.shaping.sources)
	// Published records are immutable. Cap shared slices to force later arena
	// appends to allocate, including the first discretionary on either binding.
	k.shaping.boxes = slices.Clip(k.shaping.boxes)
	k.shaping.glyphs = slices.Clip(k.shaping.glyphs)
	k.shaping.clusters = slices.Clip(k.shaping.clusters)
	k.shaping.contexts = slices.Clip(k.shaping.contexts)
	k.shaping.boundaries = slices.Clip(k.shaping.boundaries)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return k, nil
}
