/*
Package linebreak contains the paragraph shape abstraction shared by the
different line-breaking algorithms.
*/
package linebreak

import (
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
)

// ParShape is the historical name for the shared paragraph shape contract.
// Line numbers are one-based and widths include both physical side skips.
type ParShape = khipu.LineShape

type rectParShape dimen.DU

func (r rectParShape) LineLength(int32) dimen.DU {
	return dimen.DU(r)
}

// RectangularParShape returns a ParShape for paragraphs of constant line
// length.
func RectangularParShape(linelen dimen.DU) ParShape {
	return rectParShape(linelen)
}
