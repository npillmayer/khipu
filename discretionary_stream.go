package khipu

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/npillmayer/khipu/contracts"
)

// shapedReplacement is a privately owned fragment, not an arena box. Glyph
// anchors address input.text in local scalar coordinates, never provenance.
// Equal anchors form a cluster; omitted input remains available in input.text.
// Width is fixed for now: script-specific adjustment is a later concern.
type shapedReplacement struct {
	input  replacementInput
	params runParameters
	glyphs []contracts.ShapedGlyph
	widths candidateWidths
}

type shapedDiscretionaryInput struct {
	source    runSourceRange
	split     SourceSplit
	pre, post shapedReplacement
}

// shapeDiscretionaryInput shapes a validated pair without discovering breaks,
// inserting a hyphen, creating paragraph trailers or changing a khipu/cache.
// Each side uses a fresh stream and its explicitly supplied context. Only a
// complete successful pair escapes; callers may later publish it atomically.
// The factory and shared font services must obey StreamingConfig's contracts.
func shapeDiscretionaryInput(ctx context.Context, input discretionaryInput, pre, post runParameters,
	newShaper func() (contracts.StreamingShaper, error), options contracts.ShapeBufferOptions) (*shapedDiscretionaryInput, error) {
	if ctx == nil || newShaper == nil {
		return nil, fmt.Errorf("%w: missing fragment context or factory", contracts.ErrIllegalArguments)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Validate both sides before invoking a backend. Range-limited features need
	// an explicit replacement mapping; even apparently unchanged spelling does
	// not establish that mapping at this layer.
	for _, side := range []struct {
		input  replacementInput
		params runParameters
	}{{input.pre, pre}, {input.post, post}} {
		if !utf8.ValidString(side.input.text) || side.input.scalarCount != uint64(utf8.RuneCountInString(side.input.text)) ||
			side.input.scalarCount > uint64(math.MaxUint32)+1 || nilRunService(side.params.shaping.Font) ||
			nilRunService(side.params.scale) || side.params.emSize <= 0 {
			return nil, fmt.Errorf("%w: invalid replacement input or context", contracts.ErrIllegalArguments)
		}
		for _, f := range side.params.shaping.Features {
			if f.Start > 0 || f.End > 0 {
				return nil, fmt.Errorf("%w: replacement requires whole-input features", contracts.ErrIllegalArguments)
			}
		}
	}
	a, err := streamReplacement(ctx, input.pre, pre, newShaper, options)
	if err != nil {
		return nil, fmt.Errorf("khipu: pre-break fragment: %w", err)
	}
	b, err := streamReplacement(ctx, input.post, post, newShaper, options)
	if err != nil {
		return nil, fmt.Errorf("khipu: post-break fragment: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &shapedDiscretionaryInput{source: input.source, split: input.split, pre: a, post: b}, nil
}

// streamReplacement keeps the input producer in a goroutine and drains glyphs
// on the calling path. Completion is separate from channel closure because a
// pre-Shape failure need not close output. Every launched producer is cancelled
// and joined before return, including malformed output and terminal failures.
func streamReplacement(ctx context.Context, input replacementInput, params runParameters,
	newShaper func() (contracts.StreamingShaper, error), options contracts.ShapeBufferOptions) (shapedReplacement, error) {
	if err := ctx.Err(); err != nil {
		return shapedReplacement{}, err
	}
	params.shaping.Features = slices.Clone(params.shaping.Features)
	shaper, err := newShaper()
	if err != nil {
		return shapedReplacement{}, err
	}
	if nilRunService(shaper) || shaper.Glyphs() == nil {
		return shapedReplacement{}, fmt.Errorf("%w: missing fragment shaper or output", contracts.ErrIllegalArguments)
	}
	output := shaper.Glyphs()
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		if err := shaper.SetInitialCluster(0); err != nil {
			done <- err
			return
		}
		if err := shaper.Shape(ctx, params.shaping, strings.NewReader(input.text), options); err != nil {
			done <- err
			return
		}
		if err := ctx.Err(); err != nil {
			done <- err
			return
		}
		done <- shaper.Finish()
	}()
	joined := false
	defer func() {
		cancel()
		if !joined {
			<-done
		}
	}()
	completion := done
	result := shapedReplacement{input: input, params: params}
	var width candidateWidth
	for output != nil || completion != nil {
		select {
		case <-ctx.Done():
			return shapedReplacement{}, ctx.Err()
		case err := <-completion:
			joined, completion = true, nil
			if err != nil {
				return shapedReplacement{}, err
			}
		case glyph, ok := <-output:
			if !ok {
				output = nil
				continue
			}
			if uint64(glyph.Cluster) >= input.scalarCount ||
				(len(result.glyphs) > 0 && glyph.Cluster < result.glyphs[len(result.glyphs)-1].Cluster) {
				return shapedReplacement{}, fmt.Errorf("%w: invalid replacement cluster anchor", contracts.ErrIllegalArguments)
			}
			if err := width.add([]contracts.ShapedGlyph{glyph}, params.scale); err != nil {
				return shapedReplacement{}, err
			}
			result.glyphs = append(result.glyphs, glyph)
		}
	}
	if err := ctx.Err(); err != nil {
		return shapedReplacement{}, err
	}
	if err := shaper.Err(); err != nil {
		return shapedReplacement{}, err
	}
	result.widths, err = width.finish(candidateText, contracts.SpacingFixed, Params{})
	if err != nil {
		return shapedReplacement{}, err
	}
	return result, nil
}
