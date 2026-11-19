package firstfit

import (
	"errors"
	"github.com/npillmayer/khipu/internal/search"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing"
)

func breakParagraph(khp search.Paragraph, layout khipu.Params, params *Parameters, seed *breakSeed) ([]kinx, error) {
	if err := layout.ValidateGeometry(); err != nil {
		return nil, err
	}
	parshape := layout.LineShape
	if khp == nil {
		return nil, errors.New("cannot break a paragraph without khipu or parshape")
	}
	if khp.KnotCount() == 0 {
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
	lastFeasibleWidth := linebreak.WSS{}
	breakpoints := make([]kinx, 0, 8)
	start := 0
	if seed != nil && len(seed.prefix) > 0 {
		breakpoints = append(breakpoints, seed.prefix...)
		start = seed.prefix[len(seed.prefix)-1] + 1
	}
	trace := tracer()
	debug := trace.GetTraceLevel() >= tracing.LevelDebug
	for i := start; i < khp.KnotCount(); i++ {
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
		if err := search.WidthError(parshape); err != nil {
			return nil, err
		}

		segw := seg.width(layout)
		if err := segw.Err(); err != nil {
			return nil, err
		}
		if debug {
			trace.Debugf("candidate %d: seg=%v len=%.2f", i, segw, linelen.Points())
		}
		// Rescue applies to both permitted and prohibited ordinary endpoints:
		// the internal discretionary has its own penalty. Forced endpoints must
		// instead follow mandatory-break handling below, without provider calls.
		// Explicit markers can also rescue a first word without a checkpoint.
		if seed == nil && k.Penalty > InfinityMerits && k.Kind == khipu.KTTextBox && segw.Min > linelen &&
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
				lastFeasibleWidth = linebreak.WSS{}
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
				lastFeasibleWidth = linebreak.WSS{}
			} else {
				if trace.GetTraceLevel() >= tracing.LevelInfo {
					trace.Infof("Overfull box at line %d", len(breakpoints)+1)
				}
				breakpoints = append(breakpoints, i)
				seg.clear()
				firstInLine = true
				lastFeasibleWidth = linebreak.WSS{}
			}
			// The current candidate has already been scanned into the carry.
			// After committing the old checkpoint it may fit the next line;
			// retain it there instead of losing a legal break until later overflow.
			if breakpoints[len(breakpoints)-1] != i {
				nextWidth := seg.width(layout)
				if err := nextWidth.Err(); err != nil {
					return nil, err
				}
				nextLength := parshape.LineLength(int32(len(breakpoints) + 1))
				if err := search.WidthError(parshape); err != nil {
					return nil, err
				}

				if nextWidth.Min <= nextLength {
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
	if len(breakpoints) == 0 || breakpoints[len(breakpoints)-1] != khp.KnotCount()-1 {
		breakpoints = append(breakpoints, khp.KnotCount()-1)
	}

	// No remaining operation can fail. Replace the previous decisions, including
	// clearing stale selections when this successful run chose no discretionaries.
	clear(khp.Selections())
	for _, choice := range selections {
		khp.SelectDiscretionary(choice.Source, choice)
	}
	return breakpoints, nil
}

// tracer traces with key 'khipu.linebreak'.
func tracer() tracing.Trace {
	return tracing.Select("khipu.linebreak")
}
