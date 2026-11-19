# knuthplass: Paragraph cost optimization

Use `BreakParagraph(request, params)` with an immutable `AtomicBinding` and its
matching `Discovery`. Nil parameters select independent algorithm defaults.
The complete result is an immutable `Tikray`, not a list paired with mutable
selections. See [construction examples](../../example_test.go).

```go
geometry := knuthplass.DefaultLayout(shape)
request := khipu.BreakRequest{
    Binding: binding, Discovery: discovery,
    Geometry: khipu.GeometryRequest{
        Shape: geometry.LineShape,
        LeftSkip: geometry.LeftSkip, RightSkip: geometry.RightSkip,
    },
}
plan, err := knuthplass.BreakParagraph(request, nil)
if err != nil { return err }
frozen, err := khipu.Freeze(plan)
```

The fragment assumes prepared `binding`, `discovery`, and a one-based `shape`.
Search memoizes each queried width. Freeze uses captured geometry without querying
the shape again; callbacks need remain stable only during search. Later trials
cannot alter an earlier plan. Errors return nil, while completed discovery batches
remain reusable. All calls are synchronous and context-free.

KP chooses the minimum-cost complete path among candidates explored in the first
successful pass. Pass 1 uses pretolerance without discovery; pass 2 uses tolerance
and lazy alternatives. Positive finite `EmergencyStretch` enables pass 3 and
affects scoring only. The final configured pass permits fallback. Errors abort
the call; they do not trigger another pass. `DefaultLayout` supplies a physical
right skip with 50 BP finite stretch. Built-in rectangles retain state compaction;
custom shapes keep separate line-count alternatives.

| Profile | PreTolerance | Tolerance | EmergencyStretch |
| --- | ---: | ---: | ---: |
| `NewDefaultParameters()` | 100 | 200 | 0 |
| `NewTolerantParameters()` | 100 | 5000 | 50 BP |

The tolerant profile reduces final-hyphen demerits from 10000 to 50. Score fields
use `Merits` (int64); convert typed configuration values explicitly. Callers must
keep squared/accumulated costs within int64. Fitness classes, consecutive-hyphen
penalties and exact symbolic fill arithmetic in search remain unsupported.

`Tikray.Diagnostics` exposes the selected pass and score with availability flags.
`PolicyForLine` maps complete-paragraph line indexes to resolved policy records.
Seeded KP returns `ErrUnsupportedPolicy`; use first-fit continuation to retain a
KP prefix. No `ParFillSkip` or discretionary-provider option lives in Parameters:
fill is preparation material, discovery belongs to the request.

See [pass orchestration](passes.go), [scoring](scoring.go),
[geometry](geometry.go), [search](search.go) and [result collection](results.go).
