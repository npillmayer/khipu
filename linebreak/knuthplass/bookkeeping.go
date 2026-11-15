package knuthplass

import (
	"fmt"

	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/schuko/tracing"
)

type bookkeeping struct {
	segment      linebreak.WSS // sum of widths from this breakpoint up to current knot
	totalcost    merits        // sum of costs for segment up to this breakpoint
	worstbadness merits        // highest line badness encountered along the path
	leadingTrim  linebreak.WSS // discardable width before the first retained line item
	trailingTrim linebreak.WSS // discardable width after the most recent retained line item
	seenContent  bool          // does this segment contain a non-discardable content item?
}

func (path bookkeeping) String() string {
	return fmt.Sprintf("P(%.2f TC:%d MB:%d)", path.segment.W.Points(), path.totalcost, path.worstbadness)
}

type lineItemClass uint8

const (
	licContent lineItemClass = iota
	licTrimDiscardable
	licRetainedNeutral
)

// classifyLineItem derives line-edge trimming behavior entirely from knot
// flags and visible-content kind. The paragraph-final ParFillSkip node must be
// guaranteed to arrive without KFDiscardable set; otherwise it would be
// trimmed like ordinary interword space.
func classifyLineItem(k khipu.KnotCore) lineItemClass {
	if k.IsDiscardable() {
		return licTrimDiscardable
	}
	if k.IsContent() {
		return licContent
	}
	return licRetainedNeutral
}

func (book *bookkeeping) appendItem(cls lineItemClass, w linebreak.WSS) {
	book.segment = book.segment.Add(w)
	switch cls {
	case licTrimDiscardable:
		if book.seenContent {
			book.trailingTrim = book.trailingTrim.Add(w)
		} else {
			book.leadingTrim = book.leadingTrim.Add(w)
		}
	case licContent:
		book.seenContent = true
		book.trailingTrim = linebreak.WSS{}
	case licRetainedNeutral:
		// width stays in segment, but does not contribute to trim bookkeeping
	}
}

func (book bookkeeping) effectiveWidth(params khipu.Params) linebreak.WSS {
	segw := book.segment
	segw = segw.Subtract(book.leadingTrim)
	segw = segw.Subtract(book.trailingTrim)
	w := linebreak.WSS{}.SetFromKnot(params.LeftSkip)
	segw = segw.Add(w)
	w = linebreak.WSS{}.SetFromKnot(params.RightSkip)
	segw = segw.Add(w)
	return segw
}

func (b bookkeeping) segmentState() bookkeeping {
	return bookkeeping{
		segment:      b.segment,
		leadingTrim:  b.leadingTrim,
		trailingTrim: b.trailingTrim,
		seenContent:  b.seenContent,
	}
}

func (kp *linebreaker) updatePath(bp BreakRef, idx kinx, k khipu.KnotCore) {
	if bp.IsDiscretionary() && idx <= bp.At {
		return
	}
	kp.boundaryEvals.invalidate(bp)
	wss := linebreak.WSS{}.SetFromKnot(k) // get dimensions of knot
	cls := classifyLineItem(k)            // classify for line-edge trimming
	trace := tracer()
	debug := trace.GetTraceLevel() >= tracing.LevelDebug
	lines := kp.paths[bp]
	for line, path := range lines {
		if debug {
			trace.Debugf("   --- path of %s/%v = %v", bp.String(), origin{bp, line}, path)
		}
		path.appendItem(cls, wss)
		if err := path.segment.Err(); err != nil {
			kp.arithmeticErr = err
		}
		if debug {
			trace.Debugf("K&R: extending segment from %s to %v", bp.String(), path.segment)
		}
		lines[line] = path
	}
}
