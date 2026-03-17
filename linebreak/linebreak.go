/*
Package linebreak contains the paragraph shape abstraction shared by the
different line-breaking algorithms.
*/
package linebreak

import "github.com/npillmayer/khipu/dimen"

// ParShape returns the target line length for a given line number.
type ParShape interface {
	LineLength(int32) dimen.DU
}

type rectParShape dimen.DU

func (r rectParShape) LineLength(int32) dimen.DU {
	return dimen.DU(r)
}

// RectangularParShape returns a ParShape for paragraphs of constant line
// length.
func RectangularParShape(linelen dimen.DU) ParShape {
	return rectParShape(linelen)
}
