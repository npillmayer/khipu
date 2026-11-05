package khipu

import (
	"fmt"

	"github.com/npillmayer/khipu/contracts"
)

// glyphCluster is one completed logical cluster. Glyphs retain backend order,
// signed font-unit advances and offsets; no scaling or knotting happens here.
// runIndex selects immutable prepared-run parameters. source ends at the next
// anchor in that run, or at run end, and penalty belongs to that source endpoint.
// A finite penalty alone does not establish that a shaping boundary is safe.
type glyphCluster struct {
	runIndex int
	source   runSourceRange
	penalty  Penalty
	glyphs   []contracts.ShapedGlyph
}

// glyphClusterAssembler is owned by the glyph consumer. It retains one pending
// cluster until a different anchor or successful stream completion closes it.
// Runs and boundaries are borrowed immutable metadata for its entire lifetime.
// Emitted glyph slices are independently owned and never reused by the assembler.
//
// Missing anchors are not synthesized. Interior/trailing omitted scalars fall
// within the preceding cluster's inferred span; leading omitted scalars and
// completely silent runs remain uncovered source metadata. The glyph API cannot
// distinguish omitted scalars from ligature coverage. Later knotting must handle
// those uncovered ranges without losing controls or inventing glyphs.
//
// All returned clusters are provisional until the producer and consumer both
// succeed. After an error discard the paragraph, including earlier clusters.
type glyphClusterAssembler struct {
	runs       []preparedRun
	boundaries []sourceBoundary
	runIndex   int
	pending    *glyphCluster
	closed     bool
	err        error
}

func newGlyphClusterAssembler(runs []preparedRun, boundaries []sourceBoundary) (*glyphClusterAssembler, error) {
	a := &glyphClusterAssembler{runs: runs, boundaries: boundaries}
	var previousByte uint64
	for i, b := range boundaries {
		if b.scalarEnd != uint64(i)+1 || b.byteEnd <= previousByte || b.byteEnd-previousByte > 4 {
			return nil, fmt.Errorf("%w: invalid source-boundary table", contracts.ErrIllegalArguments)
		}
		previousByte = b.byteEnd
	}
	var previousEnd uint64
	for _, run := range runs {
		s := run.source
		if s.scalarStart < previousEnd || s.scalarStart >= s.scalarEnd || s.scalarEnd > uint64(len(boundaries)) ||
			s.byteStart != a.byteAt(s.scalarStart) || s.byteEnd != a.byteAt(s.scalarEnd) {
			return nil, fmt.Errorf("%w: inconsistent cluster run metadata", contracts.ErrIllegalArguments)
		}
		previousEnd = s.scalarEnd
	}
	return a, nil
}

// byteAt maps an exclusive scalar position (or a cluster anchor) to its byte
// position. Callers validate it against the complete source-boundary table.
func (a *glyphClusterAssembler) byteAt(scalar uint64) uint64 {
	if scalar == 0 {
		return 0
	}
	return a.boundaries[scalar-1].byteEnd
}

// push accepts nondecreasing cluster anchors in logical order, also for RTL.
// Repeated anchors append glyphs, not widths or source lengths. Advancing to a
// later run closes the previous cluster at its own run end, never across a gap.
func (a *glyphClusterAssembler) push(g contracts.ShapedGlyph) (*glyphCluster, error) {
	if a.err != nil {
		return nil, a.err
	}
	if a.closed {
		return nil, fmt.Errorf("%w: cluster assembler already finished", contracts.ErrIllegalArguments)
	}
	anchor := uint64(g.Cluster)
	fail := func() (*glyphCluster, error) {
		a.err = fmt.Errorf("%w: invalid or decreasing glyph cluster %d", contracts.ErrIllegalArguments, anchor)
		a.pending = nil
		return nil, a.err
	}
	if a.pending != nil && anchor < a.pending.source.scalarStart {
		return fail()
	}
	for a.runIndex < len(a.runs) && anchor >= a.runs[a.runIndex].source.scalarEnd {
		a.runIndex++
	}
	if a.runIndex == len(a.runs) || anchor < a.runs[a.runIndex].source.scalarStart {
		return fail()
	}
	if a.pending != nil && anchor == a.pending.source.scalarStart {
		a.pending.glyphs = append(a.pending.glyphs, g)
		return nil, nil
	}
	var completed *glyphCluster
	if a.pending != nil {
		end := anchor
		if a.pending.runIndex != a.runIndex {
			end = a.runs[a.pending.runIndex].source.scalarEnd
		}
		completed = a.complete(end)
	}
	a.pending = &glyphCluster{runIndex: a.runIndex, source: runSourceRange{byteStart: a.byteAt(anchor), scalarStart: anchor}, glyphs: []contracts.ShapedGlyph{g}}
	return completed, nil
}

func (a *glyphClusterAssembler) complete(end uint64) *glyphCluster {
	c := a.pending
	c.source.scalarEnd = end
	c.source.byteEnd = a.byteAt(end)
	c.penalty = a.boundaries[end-1].penalty
	a.pending = nil
	return c
}

// finish closes the final cluster only after output has been drained AND the
// producer and shaper report success. Channel closure alone is not sufficient.
// Empty output, including wholly silent runs, produces no fabricated cluster.
func (a *glyphClusterAssembler) finish() (*glyphCluster, error) {
	if a.err != nil {
		return nil, a.err
	}
	if a.closed {
		return nil, nil
	}
	a.closed = true
	if a.pending == nil {
		return nil, nil
	}
	return a.complete(a.runs[a.pending.runIndex].source.scalarEnd), nil
}
