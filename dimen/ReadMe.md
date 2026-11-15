# dimen: dimensions and units

`github.com/npillmayer/khipu/dimen` supplies the signed fixed-point `DU` used for
layout widths, plus `Point` and `Rect` page geometry.

## Start with explicit units

```go
package main

import (
    "fmt"
    "github.com/npillmayer/khipu/dimen"
)

func main() {
    width := 72 * dimen.BP
    fmt.Println(width.Points()) // 72 big points = one inch
    fmt.Println(width == dimen.IN) // true
    box := dimen.Rect{BotR: dimen.Point{X: width, Y: 36 * dimen.BP}}
    fmt.Println(box.Width().Points(), box.Height().Points()) // 72 36
}
```

| Constant | Meaning |
| --- | --- |
| `SP` | One DU; 1/65536 big point |
| `BP` | Big point, 1/72 inch; 65536 DU |
| `PT` | Printer point, approximately 1/72.27 inch; 65291 DU |
| `PX` | Numerically equal to `BP`; no screen-density conversion is performed |
| `MM`, `CM`, `IN` | Precomputed physical-unit multipliers; millimetre values are rounded |

`Points()` returns **big points**, even if a value was constructed using `PT`.
`String()` prints the raw integer with an `sp` suffix. Font-unit positions from
a shaping backend need its `FontScale.ToDU` conversion first.

## Arithmetic and fill orders

`DU` is an `int32`, with storage range -2147483648 through 2147483647. Ordinary
addition and multiplication do not return overflow errors. Use widened arithmetic
and validate before narrowing; a float-to-DU conversion does not provide checked
rounding. When accumulating fractional font advances, preserve fractions until
the completed width and round once (the encoder uses nearest DU, ties away from
zero). Reject nonfinite or out-of-range values before conversion.

Khipu's finite layout values have magnitude below `Fil`, a narrower domain than
raw int32 storage. Positive `Fil`, `Fill`, and `Filll` are ordered stretch
sentinels for glue maxima, not enormous physical widths. `Infinity` is the
largest int32 value and is not a supported fill order. Finite natural widths and
shrink minima must not use these sentinels. See [width allocation](../doc/FREEZE.md#width-allocation).

`Point.Shift` mutates and returns its receiver. `Rect.Width`/`Height` subtract
corners without normalizing reversed rectangles. The paper-size and `Origin`
variables are shared mutable values; copy them before customizing them. Paper
sizes use the literal dimensions listed in [dimen.go](dimen.go). There is no
public string parser or CSS-unit conversion API.
