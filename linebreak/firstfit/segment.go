package firstfit

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/linebreak"
)

// segmentState tracks one in-progress first-fit line together with the seed
// for the next line after the current checkpoint. `length` is the raw scanned
// material, `leadingTrim`/`trailingTrim` capture line-edge discardables, and
// `carry` remembers the already scanned post-break fragment that becomes the
// next line when a checkpoint is committed.
type segmentState struct {
	length       linebreak.WSS
	leadingTrim  linebreak.WSS
	trailingTrim linebreak.WSS
	carry        segmentSnapshot
	seenContent  bool
}

// segmentSnapshot is the checkpoint payload restored by reset(). It mirrors
// the visible-width bookkeeping of segmentState so the next line starts with
// the same trim semantics as if it had been scanned from scratch.
type segmentSnapshot struct {
	length       linebreak.WSS
	leadingTrim  linebreak.WSS
	trailingTrim linebreak.WSS
	seenContent  bool
}

func (s segmentState) segmentState() segmentState {
	return segmentState{
		length:       s.length,
		leadingTrim:  s.leadingTrim,
		trailingTrim: s.trailingTrim,
		carry:        s.carry,
		seenContent:  s.seenContent,
	}
}

type lineItemClass uint8

const (
	licContent lineItemClass = iota
	licTrimDiscardable
	licRetainedNeutral
)

func classifyLineItem(k khipu.KnotCore) lineItemClass {
	if k.IsDiscardable() {
		return licTrimDiscardable
	}
	if k.IsContent() {
		return licContent
	}
	return licRetainedNeutral
}

func (s *segmentState) append(k khipu.KnotCore) {
	w := linebreak.WSS{}.SetFromKnot(k)
	cls := classifyLineItem(k)
	s.appendToState(w, cls)
	s.appendToCarry(w, cls)
}

// appendToState advances the visible bookkeeping of the current line. Leading
// discardables accumulate until visible content arrives; afterwards discardable
// material becomes trailing trim at the candidate breakpoint.
func (s *segmentState) appendToState(w linebreak.WSS, cls lineItemClass) {
	s.length = s.length.Add(w)
	switch cls {
	case licTrimDiscardable:
		if s.seenContent {
			s.trailingTrim = s.trailingTrim.Add(w)
		} else {
			s.leadingTrim = s.leadingTrim.Add(w)
		}
	case licContent:
		s.seenContent = true
		s.trailingTrim = linebreak.WSS{}
	}
}

// appendToCarry records the same knot into the post-checkpoint snapshot. carry
// is reset whenever a new feasible breakpoint is accepted, so it always holds
// the material that would become the next line after that checkpoint.
func (s *segmentState) appendToCarry(w linebreak.WSS, cls lineItemClass) {
	s.carry.length = s.carry.length.Add(w)
	switch cls {
	case licTrimDiscardable:
		if s.carry.seenContent {
			s.carry.trailingTrim = s.carry.trailingTrim.Add(w)
		} else {
			s.carry.leadingTrim = s.carry.leadingTrim.Add(w)
		}
	case licContent:
		s.carry.seenContent = true
		s.carry.trailingTrim = linebreak.WSS{}
	}
}

// width returns the effective width of the current candidate line after
// trimming discardable material at both edges and adding paragraph side skips.
func (s *segmentState) width(params khipu.Params) linebreak.WSS {
	segw := s.length.Subtract(s.leadingTrim)
	segw = segw.Subtract(s.trailingTrim)
	segw = segw.Add(linebreak.WSS{}.SetFromKnot(params.LeftSkip))
	segw = segw.Add(linebreak.WSS{}.SetFromKnot(params.RightSkip))
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
	s.length = linebreak.WSS{}
	s.leadingTrim = linebreak.WSS{}
	s.trailingTrim = linebreak.WSS{}
	s.seenContent = false
	s.carry = segmentSnapshot{}
}

func isDiscardable(k khipu.KnotCore) bool {
	return k.IsDiscardable()
}

// segmentSeedFromKnot creates the initial state of the next line after a
// chosen discretionary break. The new line starts with the post-break fragment
// already present, so it can continue to accumulate physical knots normally.
func segmentSeedFromKnot(k khipu.KnotCore) segmentState {
	var seg segmentState
	seg.append(k)
	return seg
}
