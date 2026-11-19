# firstfit: Greedy paragraph breaking

Use `BreakParagraph(request, params)` with an immutable `AtomicBinding` and its
matching `Discovery`. Nil parameters select independent algorithm defaults.
The complete result is an immutable `Tikray`, not a list paired with mutable
selections. See [construction examples](../../example_test.go).

```go
geometry := firstfit.DefaultLayout(shape)
request := khipu.BreakRequest{
    Binding: binding, Discovery: discovery,
    Geometry: khipu.GeometryRequest{
        Shape: geometry.LineShape,
        LeftSkip: geometry.LeftSkip, RightSkip: geometry.RightSkip,
    },
}
plan, err := firstfit.BreakParagraph(request, nil)
if err != nil { return err }
frozen, err := khipu.Freeze(plan)
```

The fragment assumes prepared `binding`, `discovery`, and a one-based `shape`.
Search memoizes each queried width. Freeze uses captured geometry without querying
the shape again; callbacks need remain stable only during search. Later trials
cannot alter an earlier plan. Errors return nil, while completed discovery batches
remain reusable. All calls are synchronous and context-free.

Fit uses trimmed minimum width, including skips. The latest feasible endpoint
is committed on overflow. Local hyphenation rescue uses an inclusive
`MinHyphenGain`; zero allows zero gain. Automatic rescue needs an earlier ordinary
checkpoint, while an explicit marker can rescue the first word. Ties retain
provider order. `DefaultLayout` supplies a physical right skip with fil stretch.
Overfull fallback may succeed; inspect frozen fit diagnostics.

`ContinueParagraph(request, params)` requires `request.Prefix`, created by
`khipu.Checkpoint(completePlan, acceptedLines)`. Fresh `BreakParagraph` requires a
nil prefix. Continuation supports `NoHyphenation` and preserves a first-fit or KP
prefix exactly, including its widths, skips and policy. It never queries prefix
widths or searches prefix knots. Changed suffix atomic advances are permitted;
changed consumed advances and foreign incarnations fail. Zero/full prefixes are
valid. Drop working handles after the paragraph fully flows; retain frozen output.

See [preparation and continuation](../../doc/PREPARED_PARAGRAPHS.md),
[the scan](break.go), [carry bookkeeping](segment.go) and
[discretionary rescue](discretionary.go).
