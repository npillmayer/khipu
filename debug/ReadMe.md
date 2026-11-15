# debug: inspect frozen layout

`github.com/npillmayer/khipu/debug` writes a standalone SVG showing the final
advance widths of a `FrozenKhipu`. Use it to investigate line fitting, trimming,
replacement material, and source coverage without a font renderer.

## Write a proof

After encoding, breaking and freezing with the same layout, pass the result to
`WriteFrozenSVG`:

```go
import "github.com/npillmayer/khipu/debug"

// writer is an io.Writer; frozen is a nonempty *khipu.FrozenKhipu.
err := debug.WriteFrozenSVG(writer, frozen, debug.SVGOptions{
    Title: "Paragraph proof",
    Scale: 2, // SVG units per big point; zero selects this default.
})
```

Handle `err`, including writer failures. The writer requires a nonnil writer and
result with at least one line. Scale must be finite and in `(0, 100]`, or zero
for the default. It does not mutate the result, reshape, or rebreak text.
See the [complete freezing example](../example_test.go) for constructing a result.

To generate a sample from the module root without writing an application:

```sh
printf 'Hello world, a structural proof.' | go run ./khipu-cli view --width 90 --output proof.svg
```

Open `proof.svg` in a browser and hover over the regions for source ranges and
widths. The [CLI guide](../khipu-cli/README.md) explains input and breaker options.

## Read the diagram

| Mark | Meaning |
| --- | --- |
| Blue / green | Textbox advance / glue |
| Grey / gold | Skips or kern/custom material / paragraph fill |
| Pink | Selected pre-break or post-break replacement |
| Red dashed line / dark tick | Requested target / actual width |

Knots appear in stored physical left-to-right order, including RTL paragraphs.
Indices identify frozen knots, not input knots. Each row reports fit, residual,
ratio, badness, and fill order. Zero-width material appears as a vertical mark.

These are advance-width schematics: heights, row positions, and label fonts are
illustrative. Rows share a diagnostic origin; they do not establish page
alignment. No source text, font data, or glyph outlines are embedded. Output is
deterministic and XML-escaped, but a failing writer can receive partial output.

For the data behind the picture, see the [frozen result contract](../doc/FROZEN-API.md)
and [writer implementation](svg.go).
