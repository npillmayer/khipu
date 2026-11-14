package khipu

import (
	"context"
	"fmt"
	"io"
)

// Entries own immutable replacement coordinates/glyphs, never binding arena IDs.
// A present entry with no replacements is a successful empty result.
type explicitBatch struct {
	positions    int
	replacements []explicitReplacement
}

type explicitReplacement struct {
	variant uint16
	penalty Penalty
	pair    *shapedDiscretionaryInput
}

func (b explicitBatch) measurable() error {
	if b.positions > 1 {
		return fmt.Errorf("%w: multiple explicit split positions in one textbox", ErrUnsupportedPolicy)
	}
	n := 0
	for _, r := range b.replacements {
		if r.penalty < MaxPenalty {
			n++
		}
	}
	if n > 1 {
		return fmt.Errorf("%w: multiple permitting explicit variants", ErrUnsupportedPolicy)
	}
	return nil
}

// explicitReplacements is serialized with every other operation on this owner.
// Measurement restrictions never limit the complete batches available to layout.
func (p *PreparedParagraph) explicitReplacements(ctx context.Context, at int, measurement bool) (explicitBatch, error) {
	if err := ctx.Err(); err != nil {
		return explicitBatch{}, err
	}
	if p == nil || p.owner == nil || p.hyphenation != ExplicitHyphenation || at < 0 || at >= len(p.knots) || p.knots[at].Kind != KTTextBox {
		return explicitBatch{}, ErrIllegalArguments
	}
	if batch, ok := p.explicit[at]; ok {
		if measurement {
			if err := batch.measurable(); err != nil {
				return explicitBatch{}, err
			}
		}
		return batch, nil
	}
	if at >= len(p.boxRefs) || p.boxRefs[at] <= 0 || int(p.boxRefs[at]) > len(p.shaping.boxes) || at >= len(p.shaping.sources) {
		return explicitBatch{}, ErrIllegalArguments
	}
	box := p.shaping.boxes[p.boxRefs[at]-1]
	if box.replacement != nil || box.context < 0 || box.context >= len(p.shaping.contexts) || box.source != p.shaping.sources[at].source {
		return explicitBatch{}, ErrIllegalArguments
	}
	run, source := p.shaping.contexts[box.context], box.source
	if source.scalarStart < run.source.scalarStart || source.scalarEnd > run.source.scalarEnd {
		return explicitBatch{}, ErrIllegalArguments
	}
	raw, err := io.ReadAll(p.paragraph.Raw().BoundedReader(source.byteStart, source.byteEnd))
	if err != nil {
		return explicitBatch{}, err
	}
	positions, err := explicitSplitPositions(ctx, string(raw))
	if err != nil {
		return explicitBatch{}, err
	}
	batch := explicitBatch{positions: len(positions)}
	if measurement && batch.positions > 1 {
		return explicitBatch{}, batch.measurable()
	}
	inputs, err := p.owner.prepareExplicitDiscretionariesContext(ctx, string(raw), source)
	if err != nil {
		return explicitBatch{}, err
	}

	if len(inputs) > 0 {
		params, err := replacementParameters(run, source)
		if err != nil {
			return explicitBatch{}, err
		}
		if measurement {
			permitted := 0
			for _, input := range inputs {
				if err := ctx.Err(); err != nil {
					return explicitBatch{}, err
				}
				if input.penalty < MaxPenalty {
					permitted++
				}
			}
			if permitted > 1 {
				return explicitBatch{}, fmt.Errorf("%w: multiple permitting explicit variants", ErrUnsupportedPolicy)
			}
		}
		cfg := p.owner.env.streaming
		if cfg == nil || cfg.NewShaper == nil {
			return explicitBatch{}, ErrStreamingDiscretionariesUnsupported
		}
		for _, input := range inputs {
			pair, err := shapeDiscretionaryInput(ctx, input.input, params, params, cfg.NewShaper, cfg.BufferOptions)
			if err != nil {
				return explicitBatch{}, err
			}
			batch.replacements = append(batch.replacements, explicitReplacement{input.variant, input.penalty, pair})
		}
	}
	if err := ctx.Err(); err != nil {
		return explicitBatch{}, err
	}
	if p.explicit == nil {
		p.explicit = make(map[int]explicitBatch)
	}
	p.explicit[at] = batch // Complete batch only, including all prohibited variants.
	return batch, nil
}

// Import at the normal provider seam, never during BindParagraph. Local/manual
// candidates have already taken precedence in DiscretionaryCandidates.
func (kq *Khipukamayuq) importExplicitBatch(k *Khipu, at int) ([]DiscretionaryCandidate, error) {
	p := k.prepared
	if p == nil || p.owner != kq || k.owner != kq || p.hyphenation != ExplicitHyphenation || k.hyphenation != ExplicitHyphenation || k.paragraph != p.paragraph || at < 0 || at >= len(p.knots) {
		return nil, ErrIllegalArguments
	}
	for _, n := range []int{len(k.W), len(k.MinW), len(k.MaxW), len(k.Penalty), len(k.Pos), len(k.Len), len(k.Kind), len(k.Flags), len(k.boxRefs), len(k.shaping.sources)} {
		if at >= n {
			return nil, ErrIllegalArguments
		}
	}
	if k.KnotByIndex(at) != p.knots[at] || k.boxRefs[at] != p.boxRefs[at] || k.shaping.sources[at] != p.shaping.sources[at] {
		return nil, fmt.Errorf("%w: changed explicit binding source", ErrIllegalArguments)
	}
	ref := k.boxRefs[at]
	if ref <= 0 || int(ref) > len(k.shaping.boxes) || int(ref) > len(p.shaping.boxes) || k.shaping.boxes[ref-1] != p.shaping.boxes[ref-1] {
		return nil, ErrIllegalArguments
	}
	// The public provider has no request context; retain its established limit.
	batch, err := p.explicitReplacements(context.Background(), at, false)
	if err != nil {
		return nil, err
	}
	staged := make([]stagedDiscretionary, 0, len(batch.replacements))
	for _, r := range batch.replacements {
		local, err := k.prepareDiscretionaryPublication(at, r.variant, r.penalty, r.pair)
		if err != nil {
			return nil, err
		}
		staged = append(staged, local)
	}
	for _, local := range staged {
		k.commitDiscretionary(at, local)
	}
	return k.DiscretionariesAt(at), nil
}
