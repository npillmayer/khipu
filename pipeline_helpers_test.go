package khipu

import (
	"context"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
)

// prepareParagraphRuns prepares plain text fixtures without marker coalescing
// or atomic spans through the same preparation path used by mixed input.
func prepareParagraphRuns(para *styled.Paragraph, resolver contracts.StyleResolver, defaults contracts.ResolvedRunStyle) ([]preparedRun, error) {
	return prepareMixedRuns(para, resolver, defaults, nil, nil)
}

// producePreparedRuns exercises the producer without a no-break marker map.
func producePreparedRuns(ctx context.Context, para *styled.Paragraph, runs []preparedRun, shaper contracts.StreamingShaper, options contracts.ShapeBufferOptions) error {
	return produceNoBreakRuns(ctx, para, runs, nil, shaper, options)
}

// publishDiscretionary validates and commits a single candidate for fixtures.
// Production discovery validates complete batches before committing any member.
func (k *Khipu) publishDiscretionary(at int, variant uint16, penalty Penalty, pair *shapedDiscretionaryInput) (DiscretionaryCandidate, error) {
	staged, err := k.prepareDiscretionaryPublication(at, variant, penalty, pair)
	if err != nil {
		return DiscretionaryCandidate{}, err
	}
	return k.commitDiscretionary(at, staged), nil
}
