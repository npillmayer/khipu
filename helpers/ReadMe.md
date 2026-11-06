# helpers

`github.com/npillmayer/khipu/helpers` provides deterministic implementations of
khipu's service contracts for tests, examples, and simple client setups. It
requires no font files, installed fonts, or external shaping backend.

The helpers use deliberately simple arithmetic. They do not implement real font
shaping, glyph outlines, or rendering. See [`contracts`](../contracts/ReadMe.md)
for the interfaces they implement.

## Available Helpers

`EncodeParagraph(ctx, para)` is the sole encoder; `Config{}` supplies synthetic
font, style and streaming defaults. Metric-only helpers have been removed.
See [the retirement inventory](../doc/RETIRE-METRICS.md).

| Helper | Constructor | Zero value |
| --- | --- | --- |
| `SyntheticFont` | `NewSyntheticFont()` | Ready to use |
| `DefaultStyleResolver` | `NewDefaultStyleResolver()` | Ready to use |
| `SyntheticStreamingShaper` | `NewSyntheticStreamingShaper(outputCapacity)`; nonnegative capacity required | Ready to use with unbuffered output |

The streaming constructor returns a pointer and an error. Zero output
capacity explicitly selects an unbuffered channel. Constructors start no
goroutines.

## Synthetic Cells for Paragraph Encoding

`Config{}` supplies 5 pt synthetic cells. To select 7 pt cells, configure a
synthetic font at 14 pt, since each ordinary synthetic glyph advances half an em:

```go
owner := khipu.NewKhipukamayuq(khipu.Config{Streaming: &khipu.StreamingConfig{
    NewShaper: func() (contracts.StreamingShaper, error) {
        return helpers.NewSyntheticStreamingShaper(0)
    },
    Resolver: helpers.DefaultStyleResolver{},
    Defaults: contracts.ResolvedRunStyle{
        Font: helpers.NewSyntheticFont(), EmSize: 14 * dimen.PT,
    },
}})
```

This fragment assumes imports of `khipu`, `khipu/contracts`, `khipu/dimen`, and `khipu/helpers` using
their full module paths. The [executable encoding example](../helpers_example_test.go)
shows complete setup and measures `aéz` as 21 pt with this configuration.

The encoder drives each fresh stream while draining its glyph output. This
example leaves spacing fixed; choose `SpacingElastic` in run defaults to enable
the owner's U+0020 stretch/shrink percentages.

## Synthetic Font and Style Defaults

[`SyntheticFont`](synthetic_font.go) implements `contracts.FontFace`:

| Metric | Font units |
| --- | ---: |
| Units per em | 1,000 |
| Ascent | 800 |
| Descent | -200 |
| Line gap | 0 |

`Scale(emSize)` requires a positive em size in `dimen.DU` and returns an immutable
conversion. `ToDU(u)` computes `float64(u) * float64(emSize) / 1000`, preserving
signs and fractional values subject to floating-point precision. The caller owns
rounding and range checks; conversion does not clamp results to the DU range.
At 10 pt em size, 500 font units convert to 5 pt.

The face has no glyphs, outlines, character map, or font file. Glyph generation
and advances belong to the matching synthetic streaming shaper.

[`DefaultStyleResolver`](default_style_resolver.go) implements
`contracts.StyleResolver` for nil styles. It validates a non-nil font, positive
em size, and declared spacing mode, then returns a complete copy of the supplied
defaults with independent feature-slice storage. It preserves zero-valued
script/language, fixed spacing, and empty features without inventing values.

Every non-nil application style is rejected. Resolution does not query the font,
create a scale, or check backend feature support. For example, a nonempty feature
list may resolve successfully and subsequently be rejected by synthetic shaping.
Pass independently inherited defaults for each run.

## Standalone Streaming Example

For paragraph encoding, configure the streaming path with these helpers:

```go
owner := khipu.NewKhipukamayuq(khipu.Config{
    Streaming: &khipu.StreamingConfig{
        NewShaper: func() (contracts.StreamingShaper, error) {
            return helpers.NewSyntheticStreamingShaper(16)
        },
        Resolver: helpers.DefaultStyleResolver{},
        Defaults: contracts.ResolvedRunStyle{
            Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT,
            Spacing: contracts.SpacingElastic,
        },
    },
})
khp, err := owner.EncodeParagraph(ctx, para)
```

The owner copies configuration and default feature storage. The factory must
return a fresh stream each time. Both breakers can consume `khp`; wire the owner
as their discretionary provider to enable configured lazy hyphenation.
`Config{}` supplies the same font/size/spacing defaults with unbuffered output.

`SyntheticStreamingShaper` implements `contracts.StreamingShaper`. It is used by
the paragraph encoder and discretionary reshaping.
This complete program resolves defaults, creates a scale, and shapes a run while
draining output concurrently:

```go
package main

import (
    "context"
    "fmt"
    "strings"

    "github.com/npillmayer/khipu/contracts"
    "github.com/npillmayer/khipu/dimen"
    "github.com/npillmayer/khipu/helpers"
    "golang.org/x/text/unicode/bidi"
)

func main() {
    resolver := helpers.NewDefaultStyleResolver()
    style, err := resolver.ResolveStyle(nil, contracts.ResolvedRunStyle{
        Font: helpers.NewSyntheticFont(), EmSize: 10 * dimen.PT,
    })
    if err != nil {
        panic(err)
    }
    scale, err := style.Font.Scale(style.EmSize)
    if err != nil {
        panic(err)
    }
    stream, err := helpers.NewSyntheticStreamingShaper(1)
    if err != nil {
        panic(err)
    }
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    result := make(chan error, 1)
    go func() {
        params := contracts.ShapeParams{
            Font: style.Font, Direction: bidi.LeftToRight,
            Script: style.Script, Language: style.Language, Features: style.Features,
        }
        err := stream.Shape(ctx, params, strings.NewReader("aéz"), contracts.ShapeBufferOptions{})
        if err == nil {
            err = stream.Finish()
        }
        result <- err
    }()
    var clusters []uint32
    var width float64
    for glyph := range stream.Glyphs() {
        clusters = append(clusters, glyph.Cluster)
        width += scale.ToDU(glyph.XAdvance)
    }
    if err := <-result; err != nil {
        panic(err) // Discard accumulated output on failure.
    }
    fmt.Println("scalar clusters:", clusters)
    fmt.Println("width in pt:", width/float64(dimen.PT))
}
```

Output:

```text
scalar clusters: [0 1 2]
width in pt: 15
```

This is the pattern exercised by the
[streaming example test](synthetic_streaming_shaper_test.go).

## Character and Feature Policy

The shaper uses one cell per input rune/scalar. The following characters have
zero advance: U+000A, U+000D, U+000B, U+000C, U+0085, U+2028, U+2029, U+200B,
and U+2060. They still produce records and cluster anchors.

All other characters use one ordinary cell, including tabs, NBSP, combining
marks, and join controls. Tabs do not expand to tab stops. There is no
normalization, ligature formation, kerning, mark attachment, or visual reordering.
Consequently, decomposed `e\u0301` measures two cells and precomposed `é` one.

The streaming helper adds these explicit rules:

- Accept only `SyntheticFont` values or non-nil pointers to them.
- Emit glyph ID `uint32(rune)+1` and advance 500 font units per ordinary cell.
  All offsets and Y advances are zero; these IDs do not refer to font outlines.
- Accept LTR and RTL runs, emitting both in logical source order. Script and
  language do not alter output; mixed and neutral directions are rejected.
- Use continuing original scalar IDs for clusters, not UTF-8 byte offsets.
  `SetInitialCluster` overrides the next base while idle; overflow is an error.
- Reject malformed scalar reads and inconsistent UTF-8 lengths. Literal U+FFFD
  encoded in three bytes is valid; a one-byte decoding-error U+FFFD is not.
- Reject every nonempty feature list, including disabled entries.
- Accept both declared flush modes and zero input-buffer sizes. Reject unknown
  flush modes and nonzero watermark/maximum-buffer requests.

Output capacity counts glyph records and is independent of input buffer options.
Line-break opportunities remain the encoder's responsibility; width and cluster
boundaries alone do not authorize a break.

## Ownership, Completion, and Errors

Faces, scales, and the default resolver may be shared concurrently.
Do not copy a streaming shaper
after first use. Keep borrowed source and parameter data stable during `Shape`.

Drain streaming output concurrently even when the channel is buffered: any run
may exceed its capacity. A consumer that stops early must cancel the producer
context. Cancellation releases output backpressure but need not interrupt a
source read already in progress.

Successful `Shape` calls leave the channel open for further runs. `Finish` closes
an idle stream and can be repeated. An accepted shaping error closes output and
is retained in `Err`; already emitted glyphs can still be drained. Do not publish
accumulated results after a failed stream. Completed or failed instances cannot
be reused; construct a fresh instance for later shaping operations.

| Error | Meaning |
| --- | --- |
| Wrapped `contracts.ErrIllegalArguments` | Invalid constructor, resolver, or accepted shaping input, including unsupported features and cluster overflow |
| `ErrShaperBusy` | Overlapping `Shape`, or `Finish`/`SetInitialCluster` during an active call; the active call is unaffected |
| `ErrShaperClosed` | `Shape` or `SetInitialCluster` after completion/failure; the terminal result is unchanged |
| Reader/context error | Propagated from an accepted shaping call and retained as the terminal failure |

Use `errors.Is` for wrapped argument and context errors. After failure, `Finish`
returns the retained error; after successful completion, both `Finish` and `Err`
return nil.

## Package Boundary and Further Reading

`helpers` imports [`contracts`](../contracts/ReadMe.md), not the root paragraph
engine. Focused helper tests use these two packages. Examples that combine
helpers with paragraph encoding live in external root tests. Future fixtures
that inspect whole paragraphs belong in a separate testing package.

Existing root type names and constructors remain available through
[compatibility declarations](../compat.go); new code should import `helpers`
directly. For the broader plan and coverage, see [TESTING.md](../doc/TESTING.md).
Run the existing examples from the repository root with:

```sh
go test . ./helpers -run '^Example' -count=1 -timeout=30s
```
