package khipu

import "github.com/npillmayer/khipu/dimen"

// LogicalLineForTest exposes value snapshots only to external integration tests.
// Keeping this bridge test-only avoids committing to a public freeze API while
// the supported realization policies are still being developed.
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

type GeometryLineForTest struct {
	Content                          LogicalLineForTest
	LeftSkip, RightSkip              KnotCore
	TargetWidth, NaturalWidth, Delta dimen.DU
}

func MaterializeLineGeometryForTest(owner *Khipukamayuq, k *Khipu, breaks []int, shape lineShape, left, right KnotCore) ([]GeometryLineForTest, error) {
	p, err := owner.materializeLineGeometry(k, breaks, shape, left, right)
	if err != nil {
		return nil, err
	}
	lines := make([]GeometryLineForTest, len(p.lines))
	for i, line := range p.lines {
		content := LogicalLineForTest{NaturalWidth: line.content.naturalWidth, ByteStart: line.content.byteStart, ByteEnd: line.content.byteEnd}
		for _, item := range line.content.knots {
			content.Knots = append(content.Knots, item.core)
		}
		lines[i] = GeometryLineForTest{content, line.leftSkip, line.rightSkip, line.targetWidth, line.naturalWidth, line.delta}
	}
	return lines, nil
}

type FrozenLineForTest struct {
	Widths                     []dimen.DU
	ParagraphFill              []bool
	LeftSkip, RightSkip, Width dimen.DU
	ByteStart, ByteEnd         uint64
}

// FreezeParagraphForTest exercises the internal end-to-end constructor without
// introducing public output accessors for downstream box creation.
func FreezeParagraphForTest(owner *Khipukamayuq, k *Khipu, breaks []int, shape lineShape, left, right KnotCore) ([]FrozenLineForTest, error) {
	p, err := owner.freezeParagraph(k, breaks, shape, left, right)
	if err != nil {
		return nil, err
	}
	lines := make([]FrozenLineForTest, len(p.lines))
	for i, line := range p.lines {
		out := FrozenLineForTest{LeftSkip: line.leftSkip, RightSkip: line.rightSkip, Width: line.width, ByteStart: line.byteStart, ByteEnd: line.byteEnd}
		out.Widths = append(out.Widths, p.w[line.content.start:line.content.end]...)
		for _, origin := range p.origins[line.content.start:line.content.end] {
			out.ParagraphFill = append(out.ParagraphFill, origin.paragraphFill)
		}
		lines[i] = out
	}
	return lines, nil
}
