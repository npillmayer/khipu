package khipu

import "github.com/npillmayer/khipu/dimen"

// LogicalLineForTest exposes value snapshots only to external integration tests.
// Keeping this bridge test-only avoids committing to a public freeze API before
// ordering, allocation and final ownership have been designed.
type LogicalLineForTest struct {
	Knots              []KnotCore
	NaturalWidth       dimen.DU
	ByteStart, ByteEnd uint64
}

func MaterializeLogicalLinesForTest(owner *Khipukamayuq, k *Khipu, breaks []int) ([]LogicalLineForTest, error) {
	p, err := owner.materializeLogicalLines(k, breaks)
	if err != nil {
		return nil, err
	}
	lines := make([]LogicalLineForTest, len(p.lines))
	for i, line := range p.lines {
		lines[i] = LogicalLineForTest{NaturalWidth: line.naturalWidth, ByteStart: line.byteStart, ByteEnd: line.byteEnd}
		for _, item := range line.knots {
			lines[i].Knots = append(lines[i].Knots, item.core)
		}
	}
	return lines, nil
}

// MaterializeVisualLinesForTest exposes only geometry/source value snapshots;
// it does not add a production freeze or renderer API.
func MaterializeVisualLinesForTest(owner *Khipukamayuq, k *Khipu, breaks []int) ([]LogicalLineForTest, error) {
	p, err := owner.materializeVisualLines(k, breaks)
	if err != nil {
		return nil, err
	}
	lines := make([]LogicalLineForTest, len(p.lines))
	for i, line := range p.lines {
		lines[i] = LogicalLineForTest{NaturalWidth: line.naturalWidth, ByteStart: line.byteStart, ByteEnd: line.byteEnd}
		for _, item := range line.knots {
			lines[i].Knots = append(lines[i].Knots, item.core)
		}
	}
	return lines, nil
}
