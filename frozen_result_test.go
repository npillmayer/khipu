package khipu

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/uax/bidi"
)

func TestFrozenResultEmptyQueries(t *testing.T) {
	for _, f := range []*FrozenKhipu{nil, {}} {
		if f.LineCount() != 0 || f.KnotCount() != 0 || f.SourceLen() != 0 || f.SourceOffset() != 0 {
			t.Fatal("nonempty zero result")
		}
		for _, i := range []int{-1, 0, 1} {
			if _, ok := f.Line(i); ok {
				t.Fatal("invalid line")
			}
			if _, ok := f.Knot(i); ok {
				t.Fatal("invalid knot")
			}
			if _, ok := f.CopyText(i); ok {
				t.Fatal("invalid text")
			}
		}
		if r, err := f.SourceReader(ByteRange{}); r != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(r, err)
		}
	}
}

func TestFrozenResultEmptyParagraph(t *testing.T) {
	owner, k := logicalTestParagraph(t, "")
	f, err := owner.freezeParagraph(k, []int{len(k.Kind) - 1}, Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 10 * dimen.PT })})
	if err != nil {
		t.Fatal(err)
	}
	l, ok := f.Line(0)
	knot, _ := f.Knot(0)
	if !ok || f.LineCount() != 1 || f.KnotCount() != 1 || l.Source != (ByteRange{}) || knot.Material != ParagraphFillMaterial || knot.W != l.Width {
		t.Fatal(l, knot)
	}
	if _, ok := f.CopyText(0); ok {
		t.Fatal("fill has shaping")
	}
	r, err := f.SourceReader(ByteRange{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil || len(b) != 0 {
		t.Fatal(b, err)
	}
}

func TestFrozenResultValuesAndSnapshots(t *testing.T) {
	text := styled.TextFromString(strings.Repeat("x", 90) + "aa bb")
	para, err := styled.ParagraphFromText(&text, 90, 95, bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := NewKhipukamayuq(Config{})
	k, err := owner.EncodeParagraph(t.Context(), para)
	if err != nil {
		t.Fatal(err)
	}
	k.shaping.contexts[0].params.shaping.Features = []contracts.ShapeFeature{{Enabled: true, Start: 1, End: 2}}
	f, err := owner.freezeParagraph(k, []int{len(k.Kind) - 1}, Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 30 * dimen.PT })})
	if err != nil {
		t.Fatal(err)
	}
	l, ok := f.Line(0)
	if !ok || f.LineCount() != 1 || l.FirstKnot != 0 || l.KnotCount != f.KnotCount() || l.Width != 30*dimen.PT || l.TargetWidth != l.Width || l.Residual != 0 || l.Fit != FitWithinCapacity || l.FillOrder != dimen.Fill || l.Source != (ByteRange{0, 5}) {
		t.Fatal(l)
	}
	sum := l.LeftSkip + l.RightSkip
	for i := 0; i < f.KnotCount(); i++ {
		knot, ok := f.Knot(i)
		if !ok {
			t.Fatal(i)
		}
		sum += knot.W
		snap, hasText := f.CopyText(i)
		if knot.Kind != KTTextBox {
			if hasText {
				t.Fatal("glue exposes shaping")
			}
			continue
		}
		if !hasText || snap.IsReplacement || len(snap.Glyphs) != 2 || len(snap.Clusters) != 2 {
			t.Fatal(snap)
		}
		for j, c := range snap.Clusters {
			if c.FirstGlyph != j || c.GlyphCount != 1 {
				t.Fatal("arena indices leaked", snap)
			}
		}
		if snap.Glyphs[0].Cluster != uint32(knot.Source.ScalarStart) || snap.Context.Params.Features[0].Start != 1 {
			t.Fatal("wrong coordinate domain")
		}
		original, _ := f.CopyText(i)
		snap.Glyphs[0].GlyphID = 0
		snap.Clusters[0].FirstGlyph = 99
		snap.Context.Params.Features[0].Start = 99
		again, _ := f.CopyText(i)
		if !reflect.DeepEqual(original, again) {
			t.Fatal("snapshot aliases storage")
		}
		knot.W = 0
		againKnot, _ := f.Knot(i)
		if againKnot.W == 0 {
			t.Fatal("record aliases storage")
		}
	}
	if sum != l.Width {
		t.Fatal("nonconserving traversal")
	}
	l.Width = 0
	again, _ := f.Line(0)
	if again.Width == 0 {
		t.Fatal("line aliases storage")
	}
	fill, _ := f.Knot(f.KnotCount() - 1)
	if fill.Material != ParagraphFillMaterial || fill.Source.Bytes.Start != fill.Source.Bytes.End {
		t.Fatal(fill)
	}
	for _, i := range []int{-1, f.KnotCount()} {
		if _, ok := f.Knot(i); ok {
			t.Fatal("invalid knot")
		}
		if _, ok := f.CopyText(i); ok {
			t.Fatal("invalid shaping")
		}
	}
	if _, ok := f.Line(f.LineCount()); ok {
		t.Fatal("invalid line")
	}
	k.paragraph.Offset = 100 // offset was captured, not delegated to the paragraph
	unwireKhipu(k)
	if f.SourceOffset() != 90 || f.SourceLen() != 5 {
		t.Fatal("lost captured source")
	}
	r, err := f.SourceReader(ByteRange{1, 4})
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil || string(b) != "a b" {
		t.Fatal(string(b), err)
	}
}

func TestFrozenResultReplacementCoordinates(t *testing.T) {
	for _, silent := range []bool{false, true} {
		k, pair := discretionaryStorageFixture(t)
		if silent {
			pair.pre.input.text = ""
			pair.pre.input.scalarCount = 0
			pair.pre.glyphs = nil
			pair.pre.widths = candidateWidths{}
		}
		if _, err := k.publishDiscretionary(0, 1, 0, pair); err != nil {
			t.Fatal(err)
		}
		k.SelectDiscretionary(0, DiscretionarySelection{0, 1})
		f, err := k.owner.freezeParagraph(k, []int{0, len(k.Kind) - 1}, Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 20 * dimen.PT }), RightSkip: KnotCore{MaxW: dimen.Fil}})
		if err != nil {
			t.Fatal(err)
		}
		for i, material := range []FrozenMaterial{PreBreakMaterial, PostBreakMaterial} {
			knot, _ := f.Knot(i)
			txt, ok := f.CopyText(i)
			if !ok || !txt.IsReplacement || knot.Material != material || txt.Context.Input.Bytes.Start != 0 {
				t.Fatal(knot, txt)
			}
			if i == 0 && silent {
				if txt.ReplacementText != "" || len(txt.Glyphs) != 0 || len(txt.Clusters) != 0 || knot.Source.Bytes != (ByteRange{0, 3}) {
					t.Fatal(knot, txt)
				}
				continue
			}
			if len(txt.Glyphs) == 0 || txt.Glyphs[0].Cluster != 0 || txt.Clusters[0].FirstGlyph != 0 || txt.Clusters[0].Input.Bytes.Start != 0 {
				t.Fatal("replacement not local", txt)
			}
			if i == 1 && (txt.ReplacementText != "z" || knot.Source.Bytes != (ByteRange{3, 4})) {
				t.Fatal(knot, txt)
			}
		}
	}
}

func TestFrozenSourceReadersAndConcurrentQueries(t *testing.T) {
	text := strings.Repeat("aé ", 400)
	owner, k := logicalTestParagraph(t, text)
	f, err := owner.freezeParagraph(k, []int{len(k.Kind) - 1}, Params{LineShape: geometryShapeFunc(func(int32) dimen.DU { return 10 * dimen.PT })})
	if err != nil {
		t.Fatal(err)
	}
	unwireKhipu(k)
	for _, span := range []ByteRange{{2, 1}, {0, uint64(len(text) + 1)}, {^uint64(0), ^uint64(0)}} {
		if r, err := f.SourceReader(span); r != nil || !errors.Is(err, ErrIllegalArguments) {
			t.Fatal(r, err)
		}
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, span := range []ByteRange{{0, 0}, {uint64(len(text)), uint64(len(text))}, {2, 1101}, {500, uint64(len(text))}} {
				r, err := f.SourceReader(span)
				if err != nil {
					t.Error(err)
					return
				}
				b, err := io.ReadAll(r)
				if err != nil || string(b) != text[span.Start:span.End] {
					t.Error("bounded source differs", err)
					return
				}
			}
			for i := 0; i < f.KnotCount(); i++ {
				f.Knot(i)
				if s, ok := f.CopyText(i); ok && len(s.Glyphs) > 0 {
					s.Glyphs[0].GlyphID = 0
				}
			}
			l, ok := f.Line(0)
			if !ok || l.Fit != FitOverfull || l.Width <= l.TargetWidth || l.Residual >= 0 {
				t.Error("lost overfull outcome")
			}
		}()
	}
	wg.Wait()
}
