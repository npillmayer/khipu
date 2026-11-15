/*
Package linebreak supplies rectangular paragraph shapes and elastic width
summaries shared by the firstfit and knuthplass subpackages. Those subpackages
choose breaks; this package does not run a linebreaking algorithm.

Shapes implement khipu.LineShape with one-based line numbers and target widths
including both physical side skips. WSS provides checked numerical bookkeeping;
freezing in package khipu assigns final widths and resolves symbolic fill order.
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
