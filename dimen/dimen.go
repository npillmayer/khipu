/*
Package dimen provides signed fixed-point dimensions and simple page geometry.
One DU is 1/65536 of a big point (BP, 1/72 inch). PT uses printer points;
Points converts to BP. PX is an alias in value for BP, not a device conversion.

Arithmetic on DU is ordinary int32 arithmetic and is not checked. Callers must
check widened sums and rounded conversions before narrowing. Khipu reserves
Fil, Fill and Filll for glue stretch orders; they are not physical lengths.
The finite layout domain has magnitude below Fil; Infinity is not a fill order.

# License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2022 Norbert Pillmayer <norbert@pillmayer.com>
*/
package dimen

import (
	"fmt"
	"math"
)

// Online dimension conversion for print:
// http://www.unitconversion.org/unit_converter/typography-ex.html

// DU is a signed dimension in units of 1/65536 big point. Its underlying int32
// arithmetic does not check overflow; layout reserves the positive fill sentinels.
type DU int32

// Some pre-defined dimensions
const (
	Zero DU = 0
	SP   DU = 1       // scaled point = BP / 65536
	BP   DU = 65536   // big point (PDF) = 1/72 inch
	PX   DU = 65536   // "pixels"
	PT   DU = 65291   // printers point 1/72.27 inch
	MM   DU = 185771  // millimeters
	CM   DU = 1857710 // centimeters
	IN   DU = 4718592 // inch
)

// Infinity is the largest possible dimension
const Infinity = math.MaxInt32

// Fil is first-order infinite glue stretch, lower than Fill and Filll.
// Fill orders are layout sentinels, not physical lengths or shrink capacities.
const Fil DU = Infinity - 3

// Fill is second-order infinite glue stretch, higher than Fil and lower than Filll.
const Fill DU = Infinity - 2

// Filll is the highest supported infinite glue stretch order.
const Filll DU = Infinity - 1

// DINA4 is the 210 by 297 mm paper preset. Copy it before customization;
// this exported variable is shared mutable state.
var DINA4 = Point{210 * MM, 297 * MM}

// DINA5 is the 148 by 210 mm paper preset. Copy it before customization.
var DINA5 = Point{148 * MM, 210 * MM}

// USLetter is the legacy rounded 216 by 279 mm preset, not an exact inch size.
// Copy it before customization.
var USLetter = Point{216 * MM, 279 * MM}

// USLegal is the legacy 216 by 357 mm preset, not an exact 8.5 by 14 inch size.
// Copy it before customization.
var USLegal = Point{216 * MM, 357 * MM}

// Stringer implementation.
func (d DU) String() string {
	return fmt.Sprintf("%dsp", int32(d))
}

// Points returns a dimension in big (PDF) points.
func (d DU) Points() float64 {
	return float64(d) / float64(BP)
}

// Point is a point on a page.
//
// TODO see methods in https://golang.org/pkg/image/#Point
type Point struct {
	X, Y DU
}

// Origin is origin
var Origin = Point{0, 0}

// Shift a point along a vector.
func (p *Point) Shift(vector Point) *Point {
	p.X += vector.X
	p.Y += vector.Y
	return p
}

// Rect is a rectangle (on a page).
type Rect struct {
	TopL, BotR Point
}

// Width returns the width of a rectangle, i.e. the difference between x-coordinates
// of bottom-right and top-left corner.
func (r Rect) Width() DU {
	return r.BotR.X - r.TopL.X
}

// Height returns the height of a rectangle, i.e. the difference between y-coordinates
// of bottom-right and top-left corner.
func (r Rect) Height() DU {
	return r.BotR.Y - r.TopL.Y
}
