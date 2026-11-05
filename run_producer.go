package khipu

import (
	"bufio"
	"context"
	"fmt"
	"math"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
)

// producePreparedRuns feeds a fresh, exclusively owned stream in logical order.
// The caller runs this function in a goroutine while consuming Glyphs on its
// own execution path. Publish the complete prepared-run table before starting
// either side and keep it and the paragraph immutable until both have finished.
// No styles, fonts, or bidi intervals are resolved here.
//
// Send the returned error through a buffered completion channel. The consumer
// must observe that channel as well as Glyphs: failures before an accepted
// Shape call need not close glyph output. Success requires draining output and
// checking completion and shaper.Err. On consumer failure, cancel ctx and join
// the producer before discarding output. Cancellation cannot interrupt every
// backend source read. The caller must not reuse the stream, even on failure.
func producePreparedRuns(ctx context.Context, para *styled.Paragraph, runs []preparedRun, shaper contracts.StreamingShaper, options contracts.ShapeBufferOptions) error {
	if ctx == nil || para == nil || nilRunService(shaper) {
		return fmt.Errorf("%w: producer requires context, paragraph and shaper", contracts.ErrIllegalArguments)
	}
	var byteEnd, scalarEnd uint64
	for i, run := range runs {
		if err := ctx.Err(); err != nil {
			return err
		}
		s := run.source
		// Preparation establishes UTF-8 and byte/scalar correspondence. Here
		// check ordering, source bounds and representability before narrowing.
		if s.byteStart < byteEnd || s.byteStart >= s.byteEnd || s.byteEnd > para.Raw().Len() ||
			s.scalarStart < scalarEnd || s.scalarStart >= s.scalarEnd || s.scalarEnd > uint64(math.MaxUint32)+1 {
			return fmt.Errorf("%w: invalid producer source range for run %d", contracts.ErrIllegalArguments, i)
		}
		// Reset every run, not just the first: omitted controls still occupy
		// original scalar positions but were never supplied to the shaper.
		if err := shaper.SetInitialCluster(uint32(s.scalarStart)); err != nil {
			return fmt.Errorf("khipu: setting cluster base for run %d: %w", i, err)
		}
		reader := bufio.NewReader(para.Raw().BoundedReader(s.byteStart, s.byteEnd))
		if err := shaper.Shape(ctx, run.params.shaping, reader, options); err != nil {
			return fmt.Errorf("khipu: shaping run %d: %w", i, err)
		}
		byteEnd, scalarEnd = s.byteEnd, s.scalarEnd
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := shaper.Finish(); err != nil {
		return fmt.Errorf("khipu: finishing stream: %w", err)
	}
	return nil
}
