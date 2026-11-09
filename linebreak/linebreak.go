/*
Package linebreak contains the paragraph shape abstraction shared by the
different line-breaking algorithms.
*/
package linebreak

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

type rectParShape dimen.DU

func (r rectParShape) LineLength(int32) dimen.DU {
	return dimen.DU(r)
}

// RectangularParShape returns a shared line shape for paragraphs of constant
// line length, including both physical side skips.
func RectangularParShape(linelen dimen.DU) khipu.LineShape {
	return rectParShape(linelen)
}
