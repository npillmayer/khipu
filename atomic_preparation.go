package khipu

import (
	"fmt"
	"io"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
)

type preparedAtomic struct {
	id     contracts.AtomicID
	source runSourceRange
}

func prepareAtomicAnchors(para *styled.Paragraph, boundaries []sourceBoundary, anchors []contracts.AtomicAnchor) ([]preparedAtomic, error) {
	result := make([]preparedAtomic, 0, len(anchors))
	seen := make(map[contracts.AtomicID]bool, len(anchors))
	scalar := 0
	for i, anchor := range anchors {
		if anchor.ID == 0 || seen[anchor.ID] || (i > 0 && anchor.ByteOffset <= anchors[i-1].ByteOffset) {
			return nil, fmt.Errorf("%w: atomic IDs must be nonzero and unique, offsets strictly ordered", ErrIllegalArguments)
		}
		seen[anchor.ID] = true
		for scalar < len(boundaries) && boundaries[scalar].byteEnd <= anchor.ByteOffset {
			scalar++
		}
		start := uint64(0)
		if scalar > 0 {
			start = boundaries[scalar-1].byteEnd
		}
		if scalar == len(boundaries) || start != anchor.ByteOffset || boundaries[scalar].byteEnd-start != 3 {
			return nil, fmt.Errorf("%w: atomic anchor at byte %d does not select U+FFFC", ErrIllegalArguments, anchor.ByteOffset)
		}
		marker, err := io.ReadAll(para.Raw().BoundedReader(start, boundaries[scalar].byteEnd))
		if err != nil {
			return nil, err
		}
		if string(marker) != "\uFFFC" {
			return nil, fmt.Errorf("%w: atomic anchor does not select U+FFFC", ErrIllegalArguments)
		}
		result = append(result, preparedAtomic{anchor.ID, runSourceRange{start, start + 3, uint64(scalar), uint64(scalar + 1)}})
	}
	return result, nil
}

// Keep styleSource intact: feature ranges belong to the original style span.
func excludeAtomicIntersections(items []runIntersection, atomics []preparedAtomic) []runIntersection {
	if len(atomics) == 0 {
		return items
	}
	var result []runIntersection
	ai := 0
	for _, item := range items {
		for ai < len(atomics) && atomics[ai].source.scalarEnd <= item.source.scalarStart {
			ai++
		}
		for j := ai; j < len(atomics) && atomics[j].source.scalarStart < item.source.scalarEnd; j++ {
			s := atomics[j].source
			if item.source.scalarStart < s.scalarStart {
				prefix := item
				prefix.source.byteEnd, prefix.source.scalarEnd = s.byteStart, s.scalarStart
				result = append(result, prefix)
			}
			item.source.byteStart, item.source.scalarStart = s.byteEnd, s.scalarEnd
		}
		if item.source.scalarStart < item.source.scalarEnd {
			result = append(result, item)
		}
	}
	return result
}

func applyAtomicBoundaries(boundaries []sourceBoundary, atomics []preparedAtomic, overrides []AtomicBoundary) error {
	edges := make(map[uint64]uint64, 2*len(atomics))
	for _, atom := range atomics {
		edges[atom.source.byteStart] = atom.source.scalarStart
		edges[atom.source.byteEnd] = atom.source.scalarEnd
	}
	for i, override := range overrides {
		scalar, ok := edges[override.ByteOffset]
		if !ok || scalar == 0 || scalar == uint64(len(boundaries)) || (i > 0 && override.ByteOffset <= overrides[i-1].ByteOffset) {
			return fmt.Errorf("%w: boundary must uniquely address an interior atomic edge", ErrIllegalArguments)
		}
		b := &boundaries[scalar-1]
		if override.Action != AllowBoundary && override.Action != ProhibitBoundary {
			return fmt.Errorf("%w: unknown boundary action", ErrIllegalArguments)
		}
		if b.penalty <= MaxMerit {
			return fmt.Errorf("%w: atomic override would downgrade mandatory break", ErrUnsupportedPolicy)
		}
		b.penalty = MaxPenalty
		if override.Action == AllowBoundary {
			b.penalty = 0
		}
	}
	return nil
}
