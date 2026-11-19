package debug_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/debug"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/uax/bidi"
)

func fixture(t *testing.T) *khipu.FrozenKhipu {
	t.Helper()
	text := styled.TextFromString("hello world")
	p, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		t.Fatal(err)
	}
	o := khipu.NewKhipukamayuq(khipu.Config{})
	k, d, err := o.EncodeParagraph(p)
	if err != nil {
		t.Fatal(err)
	}
	l := firstfit.DefaultLayout(linebreak.RectangularParShape(100 * dimen.BP))
	binding, err := khipu.BindParagraph(k, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := firstfit.BreakParagraph(khipu.BreakRequest{Binding: binding, Discovery: d, Geometry: khipu.GeometryRequest{Shape: l.LineShape, LeftSkip: l.LeftSkip, RightSkip: l.RightSkip}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f, err := khipu.Freeze(b)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestSVGGeometryAndEscaping(t *testing.T) {
	f := fixture(t)
	var b bytes.Buffer
	if err := debug.WriteFrozenSVG(&b, f, debug.SVGOptions{Title: "<script>&", Scale: 2}); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	for _, want := range []string{"&lt;script&gt;&amp;", "target=100.000 actual=100.000", "paragraph fill", "class=\"target\" d=\"M 224.00", "source bytes [0,11)"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(s, "hello") {
		t.Fatal("unexpected source disclosure")
	}
	if err := debug.WriteFrozenSVG(failingWriter{}, f, debug.SVGOptions{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("writer error: %v", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestSVGInvalidArguments(t *testing.T) {
	f := fixture(t)
	for _, scale := range []float64{-1, math.NaN(), math.Inf(1), 101} {
		if err := debug.WriteFrozenSVG(io.Discard, f, debug.SVGOptions{Scale: scale}); err == nil {
			t.Fatal("accepted scale", scale)
		}
	}
	if err := debug.WriteFrozenSVG(io.Discard, nil, debug.SVGOptions{}); err == nil {
		t.Fatal("accepted nil result")
	}
	if err := debug.WriteFrozenSVG(nil, f, debug.SVGOptions{}); err == nil {
		t.Fatal("accepted nil writer")
	}
}
