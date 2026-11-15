package khipu

import "context"

import "github.com/npillmayer/khipu/dimen"

// LogicalLineForTest exposes value snapshots only to external integration tests.
// It permits inspection of intermediate stages without expanding the public API.
type LogicalLineForTest struct {
	Knots              []KnotCore
	NaturalWidth       dimen.DU
	ByteStart, ByteEnd uint64
}

func MaterializeLogicalLinesForTest(owner *Khipukamayuq, k *Khipu, breaks []int) ([]LogicalLineForTest, error) {
	p, err := owner.materializeLogicalLines(context.Background(), k, breaks)
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

// MaterializeVisualLinesForTest exposes intermediate geometry/source snapshots
// before final width allocation.
func MaterializeVisualLinesForTest(owner *Khipukamayuq, k *Khipu, breaks []int) ([]LogicalLineForTest, error) {
	p, err := owner.materializeVisualLines(context.Background(), k, breaks)
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

func MaterializeLineGeometryForTest(owner *Khipukamayuq, k *Khipu, breaks []int, params Params) ([]GeometryLineForTest, error) {
	p, err := owner.materializeLineGeometry(context.Background(), k, breaks, params)
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
	TargetWidth, Residual      dimen.DU
	Ratio                      float64
	Badness                    int
	Underfull, Overfull        bool
}

// FreezeParagraphForTest projects public Freeze results into per-line geometry
// and diagnostic snapshots for external integration tests.
func FreezeParagraphForTest(owner *Khipukamayuq, k *Khipu, breaks []int, params Params) ([]FrozenLineForTest, error) {
	p, err := owner.Freeze(context.Background(), k, breaks, params)
	if err != nil {
		return nil, err
	}
	lines := make([]FrozenLineForTest, len(p.lines))
	for i, line := range p.lines {
		out := FrozenLineForTest{LeftSkip: line.leftSkip, RightSkip: line.rightSkip, Width: line.width, ByteStart: line.byteStart, ByteEnd: line.byteEnd}
		out.TargetWidth, out.Residual = line.adjustment.target, line.adjustment.residual
		out.Ratio, out.Badness = line.adjustment.ratio, line.adjustment.badness
		out.Underfull, out.Overfull = line.adjustment.fit == widthUnderfull, line.adjustment.fit == widthOverfull
		out.Widths = append(out.Widths, p.w[line.content.start:line.content.end]...)
		for _, origin := range p.origins[line.content.start:line.content.end] {
			out.ParagraphFill = append(out.ParagraphFill, origin.paragraphFill)
		}
		lines[i] = out
	}
	return lines, nil
}
