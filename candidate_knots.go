package khipu

import (
	"fmt"
	"math"
	"slices"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
)

// candidateKnot is an intermediate, not a persistent Khipu node. core has the
// existing linebreaker metrics; clusters own glyph slices and select immutable
// prepared-run metadata. breakAfter records the exact source boundary supplying
// the penalty, which can lie beyond core's glyph-bearing span in a source gap.
// The generated paragraph trailer has no clusters and a zero-length source.
type candidateKnot struct {
	core       KnotCore
	source     runSourceRange
	breakAfter sourceBoundary
	clusters   []glyphCluster
}

// candidateKnotAccumulator accepts completed logical clusters. It keeps results
// private until finish succeeds; callers must invoke finish only after producer,
// shaper and cluster assembly success. No source text or parameter tables are
// copied. Keep the paragraph, runs and boundaries immutable throughout use.
//
// Word hints, line opportunities, run changes and source-class changes separate
// knots. None authorizes splitting a cluster. Source gaps separate candidates,
// with their strongest line penalty attached to the preceding candidate at its
// original source position. Multiple optional positions with identical geometry
// collapse to that choice. Multiple mandatory breaks cannot collapse: those and
// leading glyphless mandatory breaks return an error pending blank-line support.
type candidateKnotAccumulator struct {
	para       *styled.Paragraph
	runs       []preparedRun
	boundaries []sourceBoundary
	params     Params
	fill       KnotCore
	result     []candidateKnot
	pending    *candidateKnot
	class      candidateSourceClass
	width      candidateWidth
	lastEnd    uint64
	closed     bool
	err        error
}

func newCandidateKnotAccumulator(para *styled.Paragraph, runs []preparedRun, boundaries []sourceBoundary, params Params, fill KnotCore) (*candidateKnotAccumulator, error) {
	if para == nil {
		return nil, fmt.Errorf("%w: missing candidate paragraph", contracts.ErrIllegalArguments)
	}
	if _, err := newGlyphClusterAssembler(runs, boundaries); err != nil {
		return nil, err
	}
	length := uint64(0)
	if len(boundaries) > 0 {
		length = boundaries[len(boundaries)-1].byteEnd
	}
	if length != para.Raw().Len() || length > math.MaxUint64-para.Offset || fill.MinW > fill.W || fill.W > fill.MaxW {
		return nil, fmt.Errorf("%w: invalid candidate paragraph or fill", contracts.ErrIllegalArguments)
	}
	if _, err := (candidateWidth{}).finish(candidateText, contracts.SpacingFixed, params); err != nil {
		return nil, err
	}
	return &candidateKnotAccumulator{para: para, runs: runs, boundaries: boundaries, params: params, fill: fill}, nil
}

func (a *candidateKnotAccumulator) fail(err error) error {
	a.err = err
	a.pending = nil
	a.result = nil
	return err
}

// gapBoundary considers the preceding endpoint and the following glyphless
// source range. It never searches inside a cluster. Keep the strongest penalty
// (earliest on ties); preserve a word hint independently of line feasibility.
func (a *candidateKnotAccumulator) gapBoundary(from, to uint64) (sourceBoundary, error) {
	best := sourceBoundary{penalty: MaxPenalty}
	start := from
	if start > 0 {
		start--
	}
	forced := 0
	for i := start; i < to; i++ {
		b := a.boundaries[i]
		if b.penalty <= MaxMerit {
			forced++
		}
		word := best.wordBoundary || b.wordBoundary
		if best.scalarEnd == 0 || b.penalty < best.penalty {
			best = b
		}
		best.wordBoundary = word
	}
	if forced > 1 || (from == 0 && forced > 0) {
		return sourceBoundary{}, fmt.Errorf("%w: glyphless mandatory breaks require blank-line representation", contracts.ErrIllegalArguments)
	}
	return best, nil
}

func (a *candidateKnotAccumulator) byteAt(s uint64) uint64 {
	if s == 0 {
		return 0
	}
	return a.boundaries[s-1].byteEnd
}

// push borrows a cluster only for the call and copies its glyph records. A run
// boundary is structural even when all parameter values happen to be equal.
func (a *candidateKnotAccumulator) push(c *glyphCluster) error {
	if a.err != nil {
		return a.err
	}
	if a.closed {
		return fmt.Errorf("%w: candidate accumulator finished", contracts.ErrIllegalArguments)
	}
	bad := func() error {
		return a.fail(fmt.Errorf("%w: invalid candidate cluster", contracts.ErrIllegalArguments))
	}
	if c == nil || c.runIndex < 0 || c.runIndex >= len(a.runs) || len(c.glyphs) == 0 {
		return bad()
	}
	s := c.source
	r := a.runs[c.runIndex].source
	if s.scalarStart < a.lastEnd || s.scalarStart >= s.scalarEnd || s.scalarStart < r.scalarStart || s.scalarEnd > r.scalarEnd ||
		s.byteStart != a.byteAt(s.scalarStart) || s.byteEnd != a.byteAt(s.scalarEnd) {
		return bad()
	}
	for _, g := range c.glyphs {
		if uint64(g.Cluster) != s.scalarStart {
			return bad()
		}
	}
	class, err := classifyCandidateSource(a.para.Raw().BoundedReader(s.byteStart, s.byteEnd))
	if err != nil {
		return a.fail(err)
	}
	boundary, err := a.gapBoundary(a.lastEnd, s.scalarStart)
	if err != nil {
		return a.fail(err)
	}
	if a.pending != nil {
		prevRun := a.pending.clusters[len(a.pending.clusters)-1].runIndex
		if boundary.penalty < MaxPenalty || boundary.wordBoundary || prevRun != c.runIndex || class != a.class || s.scalarStart != a.lastEnd {
			if err := a.flush(boundary); err != nil {
				return a.fail(err)
			}
		}
	}
	if a.pending == nil {
		a.pending = &candidateKnot{source: s}
		a.class = class
		a.width = candidateWidth{}
	}
	if err := a.width.add(c.glyphs, a.runs[c.runIndex].params.scale); err != nil {
		return a.fail(err)
	}
	copyCluster := *c
	copyCluster.glyphs = slices.Clone(c.glyphs)
	a.pending.clusters = append(a.pending.clusters, copyCluster)
	a.pending.source.byteEnd = s.byteEnd
	a.pending.source.scalarEnd = s.scalarEnd
	a.lastEnd = s.scalarEnd
	return nil
}

func (a *candidateKnotAccumulator) flush(boundary sourceBoundary) error {
	if a.pending == nil {
		return nil
	}
	c := a.pending
	if c.source.byteEnd-c.source.byteStart > math.MaxUint16 {
		return fmt.Errorf("%w: candidate source span exceeds knot length capacity", contracts.ErrIllegalArguments)
	}
	run := a.runs[c.clusters[0].runIndex]
	w, err := a.width.finish(a.class, run.params.spacing, a.params)
	if err != nil {
		return err
	}
	kind, flags, _ := a.class.kindAndFlags()
	c.core = KnotCore{Pos: a.para.Offset + c.source.byteStart, Len: uint16(c.source.byteEnd - c.source.byteStart),
		W: w.W, MinW: w.MinW, MaxW: w.MaxW, Kind: kind, Flags: flags, Penalty: boundary.penalty}
	c.breakAfter = boundary
	a.result = append(a.result, *c)
	a.pending = nil
	return nil
}

// finish publishes candidates only after all upstream stages succeed. The
// original EOF penalty belongs to the trailer, not a second break on content.
// It is a one-shot transfer of owned results; later calls are rejected.
func (a *candidateKnotAccumulator) finish() ([]candidateKnot, error) {
	if a.err != nil {
		return nil, a.err
	}
	if a.closed {
		return nil, fmt.Errorf("%w: candidate accumulator finished", contracts.ErrIllegalArguments)
	}
	a.closed = true
	// Exclude the final source boundary's EOF contribution when checking gaps.
	end := uint64(len(a.boundaries))
	gapEnd := end
	if gapEnd > a.lastEnd {
		gapEnd--
	}
	b, err := a.gapBoundary(a.lastEnd, gapEnd)
	if err != nil {
		return nil, a.fail(err)
	}
	if gapEnd > a.lastEnd && b.penalty <= MaxMerit {
		return nil, a.fail(fmt.Errorf("%w: trailing glyphless mandatory break requires blank-line representation", contracts.ErrIllegalArguments))
	}
	if err := a.flush(b); err != nil {
		return nil, a.fail(err)
	}
	if len(a.result) > 0 {
		a.result[len(a.result)-1].core.Penalty = MaxPenalty
		a.result[len(a.result)-1].breakAfter.penalty = MaxPenalty
	}
	fill := a.fill
	fill.Pos = a.para.Offset + a.para.Raw().Len()
	fill.Len = 0
	fill.Kind = KTGlue
	fill.Flags = 0
	fill.Penalty = MaxMerit
	a.result = append(a.result, candidateKnot{core: fill, source: runSourceRange{a.para.Raw().Len(), a.para.Raw().Len(), end, end},
		breakAfter: sourceBoundary{byteEnd: a.para.Raw().Len(), scalarEnd: end, penalty: MaxMerit}})
	result := a.result
	a.result = nil
	return result, nil
}
