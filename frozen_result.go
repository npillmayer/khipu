package khipu

import (
	"io"
	"slices"
	"strings"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
)

// ByteRange is a half-open UTF-8 byte interval, relative to a paragraph or the
// replacement input identified by the containing record. It is not a rune range.
type ByteRange struct{ Start, End uint64 }

// SourceRange identifies bytes and Unicode scalars in the same input.
type SourceRange struct {
	Bytes                  ByteRange
	ScalarStart, ScalarEnd uint64
}

// FrozenFit reports capacity sufficiency, not a linebreaker's demerit score.
type FrozenFit uint8

const (
	FitWithinCapacity FrozenFit = iota
	FitUnderfull
	FitOverfull
)

// FrozenLine is a detached value. Knot indices are zero-based in the result,
// not in the elastic input. Content is physical left-to-right in either base
// direction. Width includes both skips and may differ from TargetWidth.
// Residual = TargetWidth - Width: slack/protrusion is at the paragraph's trailing
// edge. Underfull finite overstretch can have zero residual. Ratio is the signed
// requested finite adjustment/capacity (infinite if absent); fill uses ratio zero.
// Badness is rounded 100*abs(Ratio)^3 capped at 10000; overfull is always 10000.
// FillOrder is zero or a dimen.Fil/Fill/Filll sentinel, never an assigned width.
type FrozenLine struct {
	FirstKnot, KnotCount         int
	Width, TargetWidth, Residual dimen.DU
	LeftSkip, RightSkip          dimen.DU
	Source                       ByteRange
	Fit                          FrozenFit
	Ratio                        float64
	Badness                      int
	FillOrder                    dimen.DU
}

// FrozenMaterial identifies applied material, not a discretionary opportunity.
type FrozenMaterial uint8

const (
	OriginalMaterial FrozenMaterial = iota
	PreBreakMaterial
	PostBreakMaterial
	ParagraphFillMaterial
)

// FrozenKnot contains final geometry and original paragraph provenance. Flags
// are retained metadata, not instructions to trim or reorder the result again.
// Generated fill has empty source; silent replacements may have nonempty source.
type FrozenKnot struct {
	W        dimen.DU
	Kind     KnotType
	Flags    KnotFlags
	Source   SourceRange
	Material FrozenMaterial
}

func publicSource(s runSourceRange) SourceRange {
	return SourceRange{ByteRange{s.byteStart, s.byteEnd}, s.scalarStart, s.scalarEnd}
}

// LineCount returns the number of lines in paragraph sequence; nil is empty.
func (f *FrozenKhipu) LineCount() int {
	if f == nil {
		return 0
	}
	return len(f.lines)
}

// KnotCount counts applied content knots, excluding side skips and unused variants.
func (f *FrozenKhipu) KnotCount() int {
	if f == nil {
		return 0
	}
	return len(f.w)
}

// Line returns a detached record, or false for any invalid zero-based index.
func (f *FrozenKhipu) Line(index int) (FrozenLine, bool) {
	if index < 0 || index >= f.LineCount() {
		return FrozenLine{}, false
	}
	l := f.lines[index]
	a := l.adjustment
	fit := FitWithinCapacity
	if a.fit == widthUnderfull {
		fit = FitUnderfull
	} else if a.fit == widthOverfull {
		fit = FitOverfull
	}
	return FrozenLine{FirstKnot: l.content.start, KnotCount: l.content.end - l.content.start,
		Width: l.width, TargetWidth: a.target, Residual: a.residual, LeftSkip: l.leftSkip, RightSkip: l.rightSkip,
		Source: ByteRange{l.byteStart, l.byteEnd}, Fit: fit, Ratio: a.ratio, Badness: a.badness, FillOrder: a.fill}, true
}

// Knot returns a detached final knot. Its source remains original provenance
// even when CopyText returns shaping coordinates local to replacement text.
func (f *FrozenKhipu) Knot(index int) (FrozenKnot, bool) {
	if index < 0 || index >= f.KnotCount() {
		return FrozenKnot{}, false
	}
	o := f.origins[index]
	material := OriginalMaterial
	switch {
	case o.paragraphFill:
		material = ParagraphFillMaterial
	case o.fragment == logicalPreBreak:
		material = PreBreakMaterial
	case o.fragment == logicalPostBreak:
		material = PostBreakMaterial
	}
	return FrozenKnot{f.w[index], f.kind[index], f.flags[index], publicSource(o.source), material}, true
}

// FrozenText is an independently owned shaping snapshot. Glyph IDs and signed
// advances/offsets remain in font units and retained shaping order, not physical
// pen positions. Font/scale services are shared immutable objects. An empty
// Glyphs slice does not mean absent shaping: silent boxes are valid snapshots.
type FrozenText struct {
	Glyphs          []contracts.ShapedGlyph
	Clusters        []FrozenCluster
	Context         FrozenTextContext
	IsReplacement   bool
	ReplacementText string
}

// FrozenCluster indexes the snapshot's Glyphs, not an arena. Input and glyph
// scalar anchors are paragraph-relative for ordinary text, replacement-local
// for generated text. They are not a generated-to-original character mapping.
type FrozenCluster struct {
	Input                  SourceRange
	FirstGlyph, GlyphCount int
}

// FrozenTextContext describes the original shaping run (possibly larger than
// one knot), or a replacement-local run. Feature coordinates are relative to
// that run. This metadata is not a ready-to-replay reshaping request.
type FrozenTextContext struct {
	Input  SourceRange
	Params contracts.ShapeParams
	EmSize dimen.DU
	Scale  contracts.FontScale
}

// CopyText copies glyphs, clusters and features only for the requested knot.
// It returns false for invalid indices or absent retained shaping, including
// glue/fill. Repeated calls yield independent slices. Cost is linear in the
// requested glyph, cluster and feature counts; no whole-arena copy is made.
func (f *FrozenKhipu) CopyText(index int) (FrozenText, bool) {
	if index < 0 || index >= f.KnotCount() {
		return FrozenText{}, false
	}
	id := f.origins[index].box
	if id == 0 {
		return FrozenText{}, false
	}
	b := f.shaping.boxes[id-1]
	r := f.shaping.contexts[b.context]
	params := r.params.shaping
	params.Features = slices.Clone(params.Features)
	t := FrozenText{Glyphs: slices.Clone(f.shaping.glyphs[b.glyphs.start:b.glyphs.end]),
		Context: FrozenTextContext{publicSource(r.source), params, r.params.emSize, r.params.scale}}
	for _, c := range f.shaping.clusters[b.clusters.start:b.clusters.end] {
		t.Clusters = append(t.Clusters, FrozenCluster{publicSource(c.source), c.glyphs.start - b.glyphs.start, c.glyphs.end - c.glyphs.start})
	}
	if b.replacement != nil {
		t.IsReplacement = true
		t.ReplacementText = b.replacement.text
	}
	return t, true
}

// SourceOffset is the captured byte offset in the enclosing document. All
// result source ranges are paragraph-relative; add this only for absolute positions.
func (f *FrozenKhipu) SourceOffset() uint64 {
	if f == nil {
		return 0
	}
	return f.sourceOffset
}

// SourceLen is the original paragraph's byte length, not replacement-text length.
func (f *FrozenKhipu) SourceLen() uint64 {
	if f == nil {
		return 0
	}
	return f.source.Len()
}

// SourceReader returns a fresh bounded reader without flattening the cord.
// Byte boundaries may be arbitrary. Each reader has its own cursor; the reader
// itself is not promised concurrent use. Invalid ranges and nil/zero results
// return ErrIllegalArguments; a valid empty interval reads EOF immediately.
func (f *FrozenKhipu) SourceReader(span ByteRange) (io.Reader, error) {
	if f == nil || f.paragraph == nil || span.Start > span.End || span.End > f.SourceLen() {
		return nil, ErrIllegalArguments
	}
	if span.Start == span.End {
		return strings.NewReader(""), nil
	}
	return f.source.BoundedReader(span.Start, span.End), nil
}
