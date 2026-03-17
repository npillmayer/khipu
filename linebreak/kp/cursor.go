package knuthplass

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

// FixedWidthCursor is a legacy testing helper that assigns a fixed width to
// letters and spaces while iterating through a KhipuAOS paragraph.
type FixedWidthCursor struct {
	cursor     Cursor
	glyphWidth dimen.DU
	stretch    int
}

var _ Cursor = &FixedWidthCursor{}

// NewFixedWidthCursor creates a FixedWidthCursor with a fixed glyph width for
// every knot it will read.
func NewFixedWidthCursor(cursor Cursor, glyphWidth dimen.DU, stretchFactor int) FixedWidthCursor {
	return FixedWidthCursor{
		cursor:     cursor,
		glyphWidth: glyphWidth,
		stretch:    stretchFactor,
	}
}

func (fwc FixedWidthCursor) Next() bool {
	ok := fwc.cursor.Next()
	if ok {
		knot := fwc.cursor.Knot()
		var isChanged bool
		knot, isChanged = fwc.setTextDimens(knot)
		if isChanged {
			pos := fwc.cursor.Mark().Position()
			fwc.cursor.Khipu().ReplaceKnot(pos, knot)
		}
	}
	return ok
}

func (fwc FixedWidthCursor) Knot() khipu.Knot {
	return fwc.cursor.Knot()
}

func (fwc FixedWidthCursor) Peek() (khipu.Knot, bool) {
	peek, ok := fwc.cursor.Peek()
	if ok {
		peek, _ = fwc.setTextDimens(peek)
	}
	return peek, ok
}

func (fwc FixedWidthCursor) Mark() khipu.Mark {
	return fwc.cursor.Mark()
}

func (fwc FixedWidthCursor) Khipu() *khipu.KhipuAOS {
	return fwc.cursor.Khipu()
}

func (fwc FixedWidthCursor) setTextDimens(knot khipu.Knot) (khipu.Knot, bool) {
	isChanged := false
	switch knot.Type() {
	case khipu.KTDiscretionary:
		d := knot.(khipu.Discretionary)
		isChanged = (d.Width != fwc.glyphWidth)
		d.Width = fwc.glyphWidth
	case khipu.KTTextBox:
		b := knot.(*khipu.TextBox)
		newW := dimen.DU(len(b.Text())) * fwc.glyphWidth
		isChanged = (b.Width != newW || b.Height != fwc.glyphWidth)
		b.Width = newW
		b.Height = fwc.glyphWidth
	case khipu.KTGlue:
		g := knot.(khipu.Glue)
		g[0] = max(1, fwc.glyphWidth)
		g[1] = 0
		g[2] = max(1, fwc.glyphWidth*dimen.DU(fwc.stretch))
		return g, true
	}
	return knot, isChanged
}

func max(d1, d2 dimen.DU) dimen.DU {
	if d1 > d2 {
		return d1
	}
	return d2
}
