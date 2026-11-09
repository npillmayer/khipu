package firstfit

import (
	"errors"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing"
)

type kinx = int

const (
	noinx            = -1
	InfinityMerits   = -10000
	InfinityDemerits = 10000
)

// Parameters configures the SOA first-fit breaker.
// Geometry lives exclusively in khipu.Params, shared with freezing.
type Parameters struct {
	// MinHyphenGain is the inclusive minimum natural-width gain over the last
	// ordinary checkpoint. Zero permits zero improvement, not just positive gain.
	MinHyphenGain         dimen.DU
	DiscretionaryProvider DiscretionaryProvider
}

// NewDefaultParameters returns independent algorithm defaults: zero minimum
// hyphenation gain and no provider. Nil parameters to BreakParagraph use this
// profile. Paragraph geometry is resolved separately with DefaultLayout.
func NewDefaultParameters() *Parameters { return &Parameters{} }

// DefaultLayout resolves a ragged-right paragraph profile. The physical right
// skip absorbs positive slack at fil order, leaving finite interword glue at
// natural width. Higher-order paragraph fill still wins on the final line.
// This is not a no-compression policy: when natural width exceeds the target,
// the breaker may use the shrink capacity explicitly supplied by the knots.
// Apply overrides after this call and reuse the returned Params for freezing.
// No defaults are reapplied by BreakParagraph, even for explicitly zero skips.
func DefaultLayout(shape khipu.LineShape) khipu.Params {
	layout := khipu.DefaultParams()
	layout.LineShape = shape
	layout.LeftSkip = glue(0, 0, 0)
	layout.RightSkip = glue(0, 0, dimen.Fil)
	return layout
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
// SOA Khipu. It walks the paragraph once from left to right and keeps
// the most recent feasible breakpoint as a checkpoint. When a later feasible
// breakpoint overflows the line, the algorithm commits the remembered
// checkpoint and carries the already scanned post-break material over to the
// next line.
//
// Fit means effective MinW <= target, after edge trimming and including skips.
// Natural-width overflow alone does not force a break when shrink is available.
// Clients wanting fixed-width content must supply MinW = W = MaxW for that
// content; a fil side skip can still absorb positive slack without compressing it.
// Breaking chooses endpoints only; freezing performs the actual adjustment.
//
// Results are zero-based indices of line-ending knots, including the final
// trailer. SelectedDiscretionaries is replaced only on success. Errors preserve
// earlier selections, though providers may populate candidate caches during a
// failed call. Nil params selects NewDefaultParameters for algorithm controls only.
// Layout must be resolved
// explicitly (for example with DefaultLayout) and reused unchanged for freezing.
// Configure DiscretionaryProvider for lazy discovery.
func BreakParagraph(khp *khipu.Khipu, layout khipu.Params, params *Parameters) ([]kinx, error) {
	if err := layout.ValidateGeometry(); err != nil {
		return nil, err
	}
	parshape := layout.LineShape
	if khp == nil {
		return nil, errors.New("cannot break a paragraph without khipu or parshape")
	}
	if len(khp.Kind) == 0 {
		return nil, errors.New("cannot break an empty paragraph")
	}
	if params == nil {
		params = NewDefaultParameters()
	}
	// Stage decisions separately: a later provider failure must not publish a
	// partial result or erase selections from an earlier successful run.
	var selections []khipu.DiscretionarySelection
	seg := &segmentState{}
	firstInLine := true
	lastFeasible := noinx
	lastFeasibleWidth := WSS{}
	breakpoints := make([]kinx, 0, 8)
	trace := tracer()
	debug := trace.GetTraceLevel() >= tracing.LevelDebug
	for i := range len(khp.Kind) {
		k := khp.KnotByIndex(i)
		prevSeg := seg.segmentState()
		if debug {
			trace.Debugf("_______________ %d/%v ___________________", i, k)
		}
		seg.append(k)
		// The first visible item of a line defines the carry origin. If we later
		// have to commit an earlier checkpoint, reset() restores exactly the
		// already scanned material after that checkpoint as the seed of the next
		// line.
		if firstInLine && !isDiscardable(k) {
			seg.trackcarry()
			firstInLine = false
		}
		linelen := parshape.LineLength(int32(len(breakpoints) + 1))
		segw := seg.width(layout)
		if debug {
			trace.Debugf("candidate %d: seg=%v len=%.2f", i, segw, linelen.Points())
		}
		// Rescue applies to both permitted and prohibited ordinary endpoints:
		// the internal discretionary has its own penalty. Forced endpoints must
		// instead follow mandatory-break handling below, without provider calls.
		// Explicit markers can also rescue a first word without a checkpoint.
		if k.Penalty > InfinityMerits && k.Kind == khipu.KTTextBox && segw.Min > linelen &&
			(lastFeasible >= 0 || khp.HasExplicitDiscretionary(i)) {
			cand, nextSeg, ok, err := tryLooseHyphenation(khp, i, prevSeg, lastFeasibleWidth, linelen, layout, params)
			if err != nil {
				return nil, err
			}
			if ok {
				breakpoints = append(breakpoints, i)
				// Only the chosen discretionary is persisted as linebreaking
				// result. Other cached candidates remain opportunity data on the
				// Khipu but are not part of the committed breakpoint sequence.
				selections = append(selections, khipu.DiscretionarySelection{
					Source:  i,
					Variant: cand.Variant,
				})
				*seg = nextSeg
				firstInLine = !seg.seenContent
				lastFeasible = noinx
				lastFeasibleWidth = WSS{}
				continue
			}
		}
		if k.Penalty >= InfinityDemerits {
			if debug {
				trace.Debugf("break prohibited at %d (p=%d)", i, k.Penalty)
			}
			continue
		}
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
				lastFeasibleWidth = WSS{}
			} else {
				if trace.GetTraceLevel() >= tracing.LevelInfo {
					trace.Infof("Overfull box at line %d", len(breakpoints)+1)
				}
				breakpoints = append(breakpoints, i)
				seg.clear()
				firstInLine = true
				lastFeasibleWidth = WSS{}
			}
			// The current candidate has already been scanned into the carry.
			// After committing the old checkpoint it may fit the next line;
			// retain it there instead of losing a legal break until later overflow.
			if breakpoints[len(breakpoints)-1] != i {
				nextWidth := seg.width(layout)
				if nextWidth.Min <= parshape.LineLength(int32(len(breakpoints)+1)) {
					seg.trackcarry()
					lastFeasible = i
					lastFeasibleWidth = nextWidth
				}
			}
			continue
		}
		// Every non-overflowing candidate becomes the new checkpoint. carry then
		// starts after this breakpoint candidate, ready in case a later knot
		// forces the line to break here.
		seg.trackcarry()
		lastFeasible = i
		lastFeasibleWidth = segw
	}
	// The final node is expected to be a paragraph-final forced break. If no
	// explicit breakpoint has been committed yet, fall back to the last knot.
	if len(breakpoints) == 0 || breakpoints[len(breakpoints)-1] != len(khp.Kind)-1 {
		breakpoints = append(breakpoints, len(khp.Kind)-1)
	}
	// No remaining operation can fail. Replace the previous decisions, including
	// clearing stale selections when this successful run chose no discretionaries.
	clear(khp.SelectedDiscretionaries)
	for _, choice := range selections {
		khp.SelectDiscretionary(choice.Source, choice)
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
	LICContent lineItemClass = iota
	LICTrimDiscardable
	LICRetainedNeutral
)

func classifyLineItem(k khipu.KnotCore) lineItemClass {
	if k.IsDiscardable() {
		return LICTrimDiscardable
	}
	switch k.Kind {
	case khipu.KTTextBox:
		return LICContent
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
func (s *segmentState) width(params khipu.Params) WSS {
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

// DiscretionaryProvider supplies hyphenation alternatives for one textbox
// knot. firstfit uses it only as a local overflow rescue for the loose-line
// case.
type DiscretionaryProvider interface {
	DiscretionaryCandidates(khp *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error)
}

// discretionaryCandidates is the local provider seam for firstfit. In
// production this usually delegates to Khipukamayuq; tests may also satisfy it
// directly from the Khipu cache.
func discretionaryCandidates(khp *khipu.Khipu, at kinx, params *Parameters) ([]khipu.DiscretionaryCandidate, error) {
	if khp == nil || at < 0 || at >= len(khp.Kind) {
		return nil, khipu.ErrIllegalArguments
	}
	if params != nil && params.DiscretionaryProvider != nil {
		return params.DiscretionaryProvider.DiscretionaryCandidates(khp, int(at))
	}
	return khp.DiscretionariesAt(int(at)), nil
}

// tryLooseHyphenation evaluates discretionary variants for the overflowing
// textbox itself. A candidate is accepted only if its pre-break penalty permits
// breaking, its minimum width fits the current line, and its natural-width gain
// over the last non-hyphenated checkpoint is at least MinHyphenGain.
//
// The selection rule is intentionally greedy and local:
//   - reject PreBreak.Penalty >= InfinityDemerits, even if its width is ideal
//   - keep only variants whose PreBreak fits this line at minimum width
//   - reject variants that do not improve the line enough
//   - among the survivors, prefer the widest PreBreak
//   - use lower breakpoint penalty as a tie-break
//
// Finite penalties do not compete with width: a wider eligible variant wins
// regardless of its finite penalty. Exact width/penalty ties keep provider order.
// Zero MinHyphenGain permits zero improvement. This is a local gain filter, not
// a hyphen-frequency limit or a demerit model; discovery itself stays lazy.
func tryLooseHyphenation(khp *khipu.Khipu, at kinx, base segmentState, checkpointWidth WSS, linelen dimen.DU, layout khipu.Params, params *Parameters) (khipu.DiscretionaryCandidate, segmentState, bool, error) {
	cands, err := discretionaryCandidates(khp, at, params)
	if err != nil || len(cands) == 0 {
		return khipu.DiscretionaryCandidate{}, segmentState{}, false, err
	}
	bestWidth := dimen.DU(-1)
	bestPenalty := khipu.Penalty(InfinityDemerits)
	var best khipu.DiscretionaryCandidate
	var bestSeed segmentState
	for _, cand := range cands {
		// Prohibition is eligibility, not a preference used only to break ties.
		// Apply it equally to provider-returned and already cached alternatives.
		if cand.PreBreak.Penalty >= InfinityDemerits {
			continue
		}
		lineSeg := base.segmentState()
		lineSeg.append(cand.PreBreak)
		segw := lineSeg.width(layout)
		if segw.Min > linelen {
			continue
		}
		gain := segw.W - checkpointWidth.W
		if gain < params.MinHyphenGain {
			continue
		}
		if segw.W > bestWidth || (segw.W == bestWidth && cand.PreBreak.Penalty < bestPenalty) {
			best = cand
			bestWidth = segw.W
			bestPenalty = cand.PreBreak.Penalty
			bestSeed = segmentSeedFromKnot(cand.PostBreak)
		}
	}
	if bestWidth < 0 {
		return khipu.DiscretionaryCandidate{}, segmentState{}, false, nil
	}
	return best, bestSeed, true, nil
}

// segmentSeedFromKnot creates the initial state of the next line after a
// chosen discretionary break. The new line starts with the post-break fragment
// already present, so it can continue to accumulate physical knots normally.
func segmentSeedFromKnot(k khipu.KnotCore) segmentState {
	var seg segmentState
	seg.append(k)
	return seg
}
