package khipu

import (
	"context"
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
)

// candidateParagraph retains the metadata referenced by candidate run indices.
// It is not a Khipu and is published only on complete pipeline success. Glyph
// arrays and prepared feature slices are owned; source text remains borrowed
// from paragraph and must stay stable for later processing.
type candidateParagraph struct {
	paragraph  *styled.Paragraph
	runs       []preparedRun
	boundaries []sourceBoundary
	knots      []candidateKnot
}

// streamCandidateParagraph connects preparation, shaping and knot accumulation.
// The caller supplies a fresh, exclusively owned shaper; this function does not
// change public Config or install defaults. Do not reuse the shaper afterward.
// Preparation completes before launching the single producer goroutine. Glyph
// consumption and candidate construction run on the calling execution path.
//
// Every exit after launch cancels and joins the producer. As with the shaping
// contract, cancellation cannot interrupt an arbitrary backend read. Preparation
// is synchronous: cancellation is checked between its stages, not inside them.
// No partial candidate paragraph is returned, including on terminal Err or
// finalization failure after apparently successful glyph delivery.
func streamCandidateParagraph(ctx context.Context, para *styled.Paragraph, resolver contracts.StyleResolver,
	defaults contracts.ResolvedRunStyle, params Params, fill KnotCore,
	shaper contracts.StreamingShaper, options contracts.ShapeBufferOptions) (*candidateParagraph, error) {
	return streamMixedCandidateParagraph(ctx, para, resolver, defaults, params, fill, shaper, options, nil, nil)
}

func streamMixedCandidateParagraph(ctx context.Context, para *styled.Paragraph, resolver contracts.StyleResolver,
	defaults contracts.ResolvedRunStyle, params Params, fill KnotCore,
	shaper contracts.StreamingShaper, options contracts.ShapeBufferOptions, anchors []contracts.AtomicAnchor, overrides []AtomicBoundary) (*candidateParagraph, error) {
	if ctx == nil || nilRunService(shaper) {
		return nil, fmt.Errorf("%w: missing streaming context or shaper", contracts.ErrIllegalArguments)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	boundaries, err := prepareParagraphBoundaries(para)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	atomics, err := prepareAtomicAnchors(para, boundaries, anchors)
	if err != nil {
		return nil, err
	}
	runs, err := prepareMixedRuns(para, resolver, defaults, boundaries, atomics)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	maps, err := prepareNoBreakRuns(runs, boundaries)
	if err != nil {
		return nil, err
	}
	if err := applyAtomicBoundaries(boundaries, atomics, overrides); err != nil {
		return nil, err
	}
	assembler, err := newGlyphClusterAssembler(runs, boundaries)
	if err != nil {
		return nil, err
	}
	accumulator, err := newCandidateKnotAccumulator(para, runs, boundaries, params, fill)
	if err != nil {
		return nil, err
	}
	inputRun := 0
	atomicIndex := 0
	pushThrough := func(end uint64) error {
		for atomicIndex < len(atomics) && atomics[atomicIndex].source.scalarStart < end {
			if err := accumulator.pushAtomic(atomics[atomicIndex]); err != nil {
				return err
			}
			atomicIndex++
		}
		return nil
	}
	pushCluster := func(c *glyphCluster) error {
		if err := pushThrough(c.source.scalarStart); err != nil {
			return err
		}
		return accumulator.push(c)
	}
	err = consumeShaper(ctx, shaper, func(ctx context.Context) error {
		return produceNoBreakRuns(ctx, para, runs, maps, shaper, options)
	}, func(glyph contracts.ShapedGlyph) error {
		glyph, err := restoreNoBreakAnchor(glyph, runs, maps, &inputRun)
		if err != nil {
			return err
		}
		cluster, err := assembler.push(glyph)
		if err != nil {
			return err
		}
		if cluster != nil {
			return pushCluster(cluster)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	last, err := assembler.finish()
	if err != nil {
		return nil, err
	}
	if last != nil {
		if err := pushCluster(last); err != nil {
			return nil, err
		}
	}
	if err := pushThrough(uint64(len(boundaries))); err != nil {
		return nil, err
	}
	knots, err := accumulator.finish()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &candidateParagraph{
		paragraph:  para,
		runs:       runs,
		boundaries: boundaries,
		knots:      knots,
	}, nil
}
