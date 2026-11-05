package helpers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"unicode/utf8"

	"github.com/npillmayer/khipu/contracts"
	"golang.org/x/text/unicode/bidi"
)

var (
	// ErrShaperBusy rejects overlapping shaping or a lifecycle change during Shape.
	// It does not terminate the active operation or become the stream's Err.
	ErrShaperBusy = errors.New("khipu: shaper is busy")
	// ErrShaperClosed rejects Shape or SetInitialCluster after completion/failure.
	// The stream's Err remains its original terminal result.
	ErrShaperClosed = errors.New("khipu: shaper is closed")
)

// SyntheticStreamingShaper implements contracts.StreamingShaper for SyntheticFont only.
// It emits one glyph per scalar, with ID uint32(rune)+1 and advance 500 font
// units except for the same zero-width controls as FixedWidthShaper. Other
// advances and offsets are zero. Combining marks are independent cells. Both
// LTR and RTL runs are emitted in logical order, without normalization.
// Script and language do not affect output. Nonempty Features are rejected.
//
// The zero value is ready to use with an unbuffered output channel. A constructor
// can select a bounded capacity. Drain Glyphs concurrently with Shape, which is
// synchronous and applies backpressure; cancel its context if draining stops.
// Sequential runs share scalar numbering until Finish. Finished or failed
// instances cannot be reused. Never copy an instance after first use.
//
// This standalone helper does not wire streaming into khipu.Config or the encoder.
type SyntheticStreamingShaper struct {
	mu          sync.Mutex
	glyphs      chan contracts.ShapedGlyph
	active      bool
	closed      bool
	err         error
	nextCluster uint64 // permits the exclusive end beyond math.MaxUint32
}

// NewSyntheticStreamingShaper selects output capacity; zero means unbuffered.
// Negative capacity returns an error wrapping contracts.ErrIllegalArguments. Output
// capacity is independent of contracts.ShapeBufferOptions, which controls input buffering.
func NewSyntheticStreamingShaper(outputCapacity int) (*SyntheticStreamingShaper, error) {
	if outputCapacity < 0 {
		return nil, fmt.Errorf("%w: negative output capacity", contracts.ErrIllegalArguments)
	}
	return &SyntheticStreamingShaper{glyphs: make(chan contracts.ShapedGlyph, outputCapacity)}, nil
}

// initOutput is called with mu held, so zero-value instances have one channel.
func (s *SyntheticStreamingShaper) initOutput() {
	if s.glyphs == nil {
		s.glyphs = make(chan contracts.ShapedGlyph)
	}
}

// Glyphs returns the same channel throughout the stream's lifetime.
func (s *SyntheticStreamingShaper) Glyphs() <-chan contracts.ShapedGlyph {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initOutput()
	return s.glyphs
}

// Err returns the terminal shaping error, or nil before failure and after
// successful completion. Errors rejecting busy/closed operations are not stored.
func (s *SyntheticStreamingShaper) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// SetInitialCluster overrides the next scalar base while the stream is idle.
func (s *SyntheticStreamingShaper) SetInitialCluster(base uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return ErrShaperBusy
	}
	if s.closed {
		return ErrShaperClosed
	}
	s.nextCluster = uint64(base)
	return nil
}

// Finish closes an idle healthy stream. Repeated calls are harmless; after a
// shaping failure it returns that failure. It never disrupts an active Shape.
func (s *SyntheticStreamingShaper) Finish() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return ErrShaperBusy
	}
	if !s.closed {
		s.initOutput()
		s.closed = true
		close(s.glyphs)
	}
	return s.err
}

// Shape consumes one uniform run. Source and parameters must remain stable
// during the call; they are not retained afterwards. Only LTR/RTL directions,
// SyntheticFont faces, empty Features, and zero input buffer sizes are supported.
// Both declared flush modes emit complete one-scalar clusters without markers.
// Successful runs leave output open. Once accepted, any error terminates the
// stream and closes output, retaining already emitted records for draining.
// Reader errors are propagated; malformed scalars, invalid parameters, and
// cluster overflow wrap contracts.ErrIllegalArguments. Cancellation cannot interrupt a
// source read already in progress, but interrupts output backpressure.
func (s *SyntheticStreamingShaper) Shape(ctx context.Context, params contracts.ShapeParams, source contracts.RuneReader, opts contracts.ShapeBufferOptions) (err error) {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return ErrShaperBusy
	}
	if s.closed {
		s.mu.Unlock()
		return ErrShaperClosed
	}
	s.initOutput()
	s.active = true
	next, output := s.nextCluster, s.glyphs
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.active = false
		s.nextCluster = next
		if err != nil {
			s.err = err
			s.closed = true
			close(output)
		}
	}()

	if isNilInterface(ctx) || isNilInterface(source) {
		return fmt.Errorf("%w: shaping requires a context and source", contracts.ErrIllegalArguments)
	}
	switch font := params.Font.(type) {
	case SyntheticFont:
	case *SyntheticFont:
		if font == nil {
			return fmt.Errorf("%w: nil synthetic font", contracts.ErrIllegalArguments)
		}
	default:
		return fmt.Errorf("%w: synthetic shaper requires SyntheticFont", contracts.ErrIllegalArguments)
	}
	if params.Direction != bidi.LeftToRight && params.Direction != bidi.RightToLeft {
		return fmt.Errorf("%w: shaping requires an LTR or RTL run", contracts.ErrIllegalArguments)
	}
	if len(params.Features) != 0 {
		return fmt.Errorf("%w: synthetic shaper does not support features", contracts.ErrIllegalArguments)
	}
	if (opts.Flush != contracts.ShapeFlushRun && opts.Flush != contracts.ShapeFlushCluster) ||
		opts.HighWatermark != 0 || opts.LowWatermark != 0 || opts.MaxBuffer != 0 {
		return fmt.Errorf("%w: unsupported synthetic input buffering", contracts.ErrIllegalArguments)
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		r, size, readErr := source.ReadRune()
		if err := ctx.Err(); err != nil {
			return err
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
		if !utf8.ValidRune(r) || size != utf8.RuneLen(r) {
			return fmt.Errorf("%w: invalid scalar or UTF-8 length", contracts.ErrIllegalArguments)
		}
		if next > math.MaxUint32 {
			return fmt.Errorf("%w: synthetic cluster overflow", contracts.ErrIllegalArguments)
		}
		glyph := contracts.ShapedGlyph{GlyphID: uint32(r) + 1, Cluster: uint32(next), XAdvance: 500}
		if syntheticZeroWidth(r) {
			glyph.XAdvance = 0
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case output <- glyph:
			next++
		}
	}
}

var _ contracts.StreamingShaper = (*SyntheticStreamingShaper)(nil)
