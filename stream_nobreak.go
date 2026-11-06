package khipu

import (
	"fmt"
	"math"
	"sort"

	"github.com/npillmayer/khipu/contracts"
)

// noBreakRun maps dense, marker-free input scalars back to original paragraph
// scalars. Nil means identity; a nonnil empty map means a marker-only run.
// Maps are transient, immutable producer/consumer metadata. Stored glyphs and
// prepared contexts continue to use original coordinates after restoration.
type noBreakRun struct {
	anchors  []uint32
	features []contracts.ShapeFeature
}

// The bidi segment iterator can isolate a boundary-neutral marker as a separate
// segment. Join across that segment only within the same original style span,
// direction and contiguous source. Do not generally merge equal-parameter runs:
// their boundaries can carry structural meaning unrelated to soft hyphens.
func coalesceMarkerIntersections(items []runIntersection, boundaries []sourceBoundary) []runIntersection {
	var result []runIntersection
	for _, item := range items {
		markerOnly := true
		for p := item.source.scalarStart; p < item.source.scalarEnd; p++ {
			markerOnly = markerOnly && boundaries[p].softHyphen
		}
		if len(result) > 0 {
			prev := &result[len(result)-1]
			if prev.styleSource == item.styleSource && prev.direction == item.direction &&
				prev.source.byteEnd == item.source.byteStart && prev.source.scalarEnd == item.source.scalarStart &&
				(markerOnly || boundaries[prev.source.scalarEnd-1].softHyphen) {
				prev.source.byteEnd, prev.source.scalarEnd = item.source.byteEnd, item.source.scalarEnd
				continue
			}
		}
		result = append(result, item)
	}
	return result
}

// prepareNoBreakRuns filters only U+00AD, without flattening paragraph text or
// splitting shaping calls at markers (which could change ligatures/joining).
// Boundary flags survive in the arena for later explicit-discretionary lookup.
// This encoding policy leaves raw Unicode analysis unchanged. Runs and boundary
// tables are private at this point.
func prepareNoBreakRuns(runs []preparedRun, boundaries []sourceBoundary) ([]*noBreakRun, error) {
	for i := range boundaries {
		if boundaries[i].softHyphen {
			boundaries[i].penalty = MaxPenalty
			boundaries[i].wordBoundary = false
		}
	}
	maps := make([]*noBreakRun, len(runs))
	for i, r := range runs {
		s := r.source
		if s.scalarStart >= s.scalarEnd || s.scalarEnd > uint64(len(boundaries)) || s.scalarEnd > uint64(math.MaxUint32)+1 {
			return nil, fmt.Errorf("%w: invalid no-break run range", contracts.ErrIllegalArguments)
		}
		found := false
		for p := s.scalarStart; p < s.scalarEnd; p++ {
			found = found || boundaries[p].softHyphen
		}
		if !found {
			continue
		}
		m := &noBreakRun{}
		for p := s.scalarStart; p < s.scalarEnd; p++ {
			if !boundaries[p].softHyphen {
				m.anchors = append(m.anchors, uint32(p))
			}
		}
		// A feature selects surviving scalars in its original half-open range.
		// Omit empty selections: [0,0) would mean the whole input to a shaper.
		for _, f := range r.params.shaping.Features {
			start, end := s.scalarStart, s.scalarEnd
			if f.Start > 0 {
				start += min(uint64(f.Start), s.scalarEnd-s.scalarStart)
			}
			if f.End > 0 {
				end = s.scalarStart + min(uint64(f.End), s.scalarEnd-s.scalarStart)
			}
			lo := sort.Search(len(m.anchors), func(j int) bool { return uint64(m.anchors[j]) >= start })
			hi := sort.Search(len(m.anchors), func(j int) bool { return uint64(m.anchors[j]) >= end })
			if lo >= hi {
				continue
			}
			f.Start, f.End = lo, hi
			m.features = append(m.features, f)
		}
		maps[i] = m
	}
	return maps, nil
}

// noBreakReader removes markers before shaping, not after glyph production.
// Returning the original byte size of each retained rune obeys RuneReader;
// streaming cluster IDs count scalars, not those byte sizes.
type noBreakReader struct{ contracts.RuneReader }

func (r noBreakReader) ReadRune() (rune, int, error) {
	for {
		c, n, err := r.RuneReader.ReadRune()
		if err != nil || c != '\u00ad' {
			return c, n, err
		}
	}
}

// restoreNoBreakAnchor runs before ordinary cluster assembly. Each backend run
// still starts at its original scalar base, leaving unambiguous gaps between
// runs. Anchors in a filtered run's unused tail are errors, not source markers.
func restoreNoBreakAnchor(g contracts.ShapedGlyph, runs []preparedRun, maps []*noBreakRun, runIndex *int) (contracts.ShapedGlyph, error) {
	anchor := uint64(g.Cluster)
	for *runIndex < len(runs) && anchor >= runs[*runIndex].source.scalarEnd {
		*runIndex = *runIndex + 1
	}
	if *runIndex >= len(runs) || anchor < runs[*runIndex].source.scalarStart {
		return g, fmt.Errorf("%w: glyph outside no-break runs", contracts.ErrIllegalArguments)
	}
	if m := maps[*runIndex]; m != nil {
		local := anchor - runs[*runIndex].source.scalarStart
		if local >= uint64(len(m.anchors)) {
			return g, fmt.Errorf("%w: glyph outside filtered input", contracts.ErrIllegalArguments)
		}
		g.Cluster = m.anchors[local]
	}
	return g, nil
}
