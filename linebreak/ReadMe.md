# linebreak: shapes and width summaries

`github.com/npillmayer/khipu/linebreak` supplies shared helpers. Choose an algorithm
from a subpackage to actually break a paragraph:

| Package | Use |
| --- | --- |
| [firstfit](firstfit/ReadMe.md) | Greedy checkpoints with local overflow rescue |
| [knuthplass](knuthplass/ReadMe.md) | Minimum accumulated cost among candidates explored by successive passes |

## Describe the available width

`RectangularParShape(100 * dimen.BP)` returns a constant-width `khipu.LineShape`.
For varying widths, implement the same small interface:

```go
package main

import (
    "fmt"
    "github.com/npillmayer/khipu"
    "github.com/npillmayer/khipu/dimen"
)

type insetFirstLine struct{}

func (insetFirstLine) LineLength(line int32) dimen.DU {
    if line == 1 { return 80 * dimen.BP }
    return 100 * dimen.BP
}

func main() {
    var shape khipu.LineShape = insetFirstLine{}
    fmt.Println(shape.LineLength(1).Points()) // 80
}
```

Line numbers are **one-based**. Targets include both physical side skips. A shape
supplies widths, not x positions or indentation coordinates. It must remain stable
and side-effect free across breaking and freezing. Resolve an algorithm's
`DefaultLayout(shape)`, apply overrides, then pass that same `khipu.Params` to
both operations. Copying Params does not clone the shape service.

Knuth–Plass recognizes the built-in rectangle for state compaction; a custom
wrapper retains line-count alternatives even when observed widths happen to match.

## Use width summaries

`WSS` stores natural, minimum, and maximum widths. It is mainly useful when
implementing a breaker; normal clients can work with paragraph and frozen queries.

```go
// With imports for khipu, dimen, and linebreak:
knot := khipu.KnotCore{W: 10*dimen.BP, MinW: 8*dimen.BP, MaxW: 15*dimen.BP}
sum := linebreak.WSS{}.SetFromKnot(knot)
sum = sum.Add(sum) // Add, Subtract and SetFromKnot return values.
err := sum.Err()   // Handle int64 accumulation overflow before using the summary.
```

The zero value is an empty sum. Public fields saturate at DU's int32 bounds while
private excess allows later subtraction to recover the sum. Preserve the returned
value intact: editing its fields or reconstructing it from `Spread()` loses that
arithmetic state. Overflow persists until reset with a fresh value.

WSS uses numerical sums, including fill sentinels; it does not assign final widths
or resolve symbolic fill precedence. See [freezing](../doc/FREEZE.md),
[WSS source](wss.go), and the [complete pipeline example](../example_test.go).
