package firstfit

import (
	"errors"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
)

type kinx = int

const (
	noinx            = -1
	InfinityMerits   = -10000
	InfinityDemerits = 10000
)

// Parameters configures the new Khipu-based first-fit breaker.
// The model is intentionally small: first-fit only needs the paragraph skips
// for width accounting, not the richer demerit model of Knuth-Plass.
type Parameters struct {
	LeftSkip  khipu.KnotCore
	RightSkip khipu.KnotCore
}

// DefaultParameters are the standard first-fit parameters for the new Khipu path.
var DefaultParameters = &Parameters{
	LeftSkip:  glue(0, 0, 0),
	RightSkip: glue(0, 0, 0),
}

func glue(w, wmin int, wmax dimen.DU) khipu.KnotCore {
	return khipu.KnotCore{
		W:       dimen.DU(w) * dimen.BP,
		MinW:    dimen.DU(wmin) * dimen.BP,
		MaxW:    wmax,
		Penalty: 0,
		Kind:    khipu.KTGlue,
	}
}

// BreakParagraph finds first-fit breakpoints for a paragraph represented as the
// new SOA-style Khipu. It walks the paragraph once from left to right and keeps
// the most recent feasible breakpoint as a checkpoint. When a later feasible
// breakpoint overflows the line, the algorithm commits the remembered
// checkpoint and carries the already scanned post-break material over to the
// next line.
func BreakParagraph(khp *khipu.Khipu, parshape linebreak.ParShape, params *Parameters) ([]kinx, error) {
	if khp == nil || parshape == nil {
		return nil, errors.New("cannot break a paragraph without khipu or parshape")
	}
	if len(khp.Kind) == 0 {
		return nil, errors.New("cannot break an empty paragraph")
	}
	if params == nil {
		params = DefaultParameters
	}
	seg := &segmentState{}
	firstInLine := true
	lastFeasible := noinx
	breakpoints := make([]kinx, 0, 8)
	for i := range len(khp.Kind) {
		k := khp.KnotByIndex(i)
		tracer().Debugf("_______________ %d/%v ___________________", i, k)
		seg.append(k)
		// The first visible item of a line defines the carry origin. If we later
		// have to commit an earlier checkpoint, reset() restores exactly the
		// already scanned material after that checkpoint as the seed of the next
		// line.
		if firstInLine && !isDiscardable(k) {
			seg.trackcarry()
			firstInLine = false
		}
		if k.Penalty >= InfinityDemerits {
			tracer().Debugf("break prohibited at %d (p=%d)", i, k.Penalty)
			continue
		}
		linelen := parshape.LineLength(int32(len(breakpoints) + 1))
		segw := seg.width(params)
		tracer().Debugf("candidate %d: seg=%v len=%.2f", i, segw, linelen.Points())
		if k.Penalty <= InfinityMerits {
			// Forced breaks always end the current line. If the scanned material is
			// already overfull, first commit the last earlier feasible checkpoint
			// and then honor the forced break on the following line.
			if segw.Min > linelen && lastFeasible >= 0 {
				breakpoints = append(breakpoints, lastFeasible)
				seg.reset()
				firstInLine = !seg.seenContent
				lastFeasible = noinx
			}
			breakpoints = append(breakpoints, i)
			seg.clear()
			firstInLine = true
			lastFeasible = noinx
			continue
		}
		if segw.Min > linelen {
			// A candidate that cannot shrink enough no longer fits this line. The
			// first-fit decision is therefore to commit the last acceptable
			// checkpoint and continue with the carried post-break material.
			if lastFeasible >= 0 {
				breakpoints = append(breakpoints, lastFeasible)
				seg.reset()
				firstInLine = !seg.seenContent
				lastFeasible = noinx
			} else {
				tracer().Infof("Overfull box at line %d", len(breakpoints)+1)
				breakpoints = append(breakpoints, i)
				seg.clear()
				firstInLine = true
			}
			continue
		}
		// Every non-overflowing candidate becomes the new checkpoint. carry then
		// starts after this breakpoint candidate, ready in case a later knot
		// forces the line to break here.
		seg.trackcarry()
		lastFeasible = i
	}
	// The final node is expected to be a paragraph-final forced break. If no
	// explicit breakpoint has been committed yet, fall back to the last knot.
	if len(breakpoints) == 0 || breakpoints[len(breakpoints)-1] != len(khp.Kind)-1 {
		breakpoints = append(breakpoints, len(khp.Kind)-1)
	}
	return breakpoints, nil
}

// WSS is shared with other linebreakers through the root linebreak package.
type WSS = linebreak.WSS

// segmentState tracks one in-progress first-fit line together with the seed
// for the next line after the current checkpoint. `length` is the raw scanned
// material, `leadingTrim`/`trailingTrim` capture line-edge discardables, and
// `carry` remembers the already scanned post-break fragment that becomes the
// next line when a checkpoint is committed.
type segmentState struct {
	length       WSS
	leadingTrim  WSS
	trailingTrim WSS
	carry        segmentSnapshot
	seenContent  bool
}

// segmentSnapshot is the checkpoint payload restored by reset(). It mirrors
// the visible-width bookkeeping of segmentState so the next line starts with
// the same trim semantics as if it had been scanned from scratch.
type segmentSnapshot struct {
	length       WSS
	leadingTrim  WSS
	trailingTrim WSS
	seenContent  bool
}

type lineItemClass uint8

const (
	LICContent lineItemClass = iota
	LICTrimDiscardable
	LICRetainedNeutral
)

func classifyLineItem(k khipu.KnotCore) lineItemClass {
	if k.IsDiscardable() {
		return LICTrimDiscardable
	}
	switch k.Kind {
	case khipu.KTTextBox, khipu.KTDiscretionary:
		return LICContent
	case khipu.KTPenalty:
		return LICRetainedNeutral
	default:
		return LICRetainedNeutral
	}
}

func (s *segmentState) append(k khipu.KnotCore) {
	w := WSS{}.SetFromKnot(k)
	cls := classifyLineItem(k)
	s.appendToState(w, cls)
	s.appendToCarry(w, cls)
}

// appendToState advances the visible bookkeeping of the current line. Leading
// discardables accumulate until visible content arrives; afterwards discardable
// material becomes trailing trim at the candidate breakpoint.
func (s *segmentState) appendToState(w WSS, cls lineItemClass) {
	s.length = s.length.Add(w)
	switch cls {
	case LICTrimDiscardable:
		if s.seenContent {
			s.trailingTrim = s.trailingTrim.Add(w)
		} else {
			s.leadingTrim = s.leadingTrim.Add(w)
		}
	case LICContent:
		s.seenContent = true
		s.trailingTrim = WSS{}
	}
}

// appendToCarry records the same knot into the post-checkpoint snapshot. carry
// is reset whenever a new feasible breakpoint is accepted, so it always holds
// the material that would become the next line after that checkpoint.
func (s *segmentState) appendToCarry(w WSS, cls lineItemClass) {
	s.carry.length = s.carry.length.Add(w)
	switch cls {
	case LICTrimDiscardable:
		if s.carry.seenContent {
			s.carry.trailingTrim = s.carry.trailingTrim.Add(w)
		} else {
			s.carry.leadingTrim = s.carry.leadingTrim.Add(w)
		}
	case LICContent:
		s.carry.seenContent = true
		s.carry.trailingTrim = WSS{}
	}
}

// width returns the effective width of the current candidate line after
// trimming discardable material at both edges and adding paragraph side skips.
func (s *segmentState) width(params *Parameters) WSS {
	segw := s.length.Subtract(s.leadingTrim)
	segw = segw.Subtract(s.trailingTrim)
	segw = segw.Add(WSS{}.SetFromKnot(params.LeftSkip))
	segw = segw.Add(WSS{}.SetFromKnot(params.RightSkip))
	return segw
}

// reset commits the current checkpoint. The next line starts from the carried
// post-break snapshot rather than from an empty segment.
func (s *segmentState) reset() {
	s.length = s.carry.length
	s.leadingTrim = s.carry.leadingTrim
	s.trailingTrim = s.carry.trailingTrim
	s.seenContent = s.carry.seenContent
	s.carry = segmentSnapshot{}
}

// trackcarry starts a fresh post-checkpoint snapshot immediately after a newly
// accepted feasible breakpoint.
func (s *segmentState) trackcarry() {
	s.carry = segmentSnapshot{}
}

// clear discards both the current line and any pending carried state. It is
// used after forced breaks and overfull fallback breaks.
func (s *segmentState) clear() {
	s.length = WSS{}
	s.leadingTrim = WSS{}
	s.trailingTrim = WSS{}
	s.seenContent = false
	s.carry = segmentSnapshot{}
}

func isDiscardable(k khipu.KnotCore) bool {
	return k.IsDiscardable()
}
