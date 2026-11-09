# Khipu Structural Diagnostics

`khipu-cli` uses `commando`, like the downstream OpenType diagnostic tools, but
does not import OpenType. This first version is a synthetic layout exerciser,
not a typography renderer or a font loader.

## Usage

From the module root:

```sh
go run ./khipu-cli view --help
printf 'Hello world, a structural proof.' | go run ./khipu-cli view --width 90 --output proof.svg
go run ./khipu-cli view --input paragraph.txt --breaker knuthplass --direction rtl --output proof.svg
go build -o /tmp/khipu-cli ./khipu-cli
```

Input and output default to stdin/stdout (`-`). Input must be UTF-8 and is limited
to 1 MiB; bytes, including terminal newlines, are preserved. The whole input is
treated as one paragraph. Width is an integer in **big points** (1/72 inch), not
TeX/printer points. Directions are `ltr` and `rtl`. Breakers are `firstfit`
(default, ragged-right layout) and `knuthplass` (default justified layout).

The command uses `Config{}` synthetic shaping. It wires the owner as discretionary
provider, so explicit soft hyphens can be selected; no dictionary hyphenator is
installed. It freezes using the same layout as the breaker. Ctrl-C cancels
encoding/freezing cooperatively, but cannot interrupt the synchronous breaker
or a blocked input read. Failed layout does not overwrite the destination.
Filesystem write failures can still leave a partial file.

## Reading The Diagram

- Blue regions are final textbox **advance widths**, not ink bounds.
- Green denotes glue; grey denotes physical side skips and kern/custom items.
- Gold denotes paragraph fill; pink denotes applied pre-/post-break material.
- Knot numbers index the frozen result, not the elastic input. Hover titles
  contain original source byte ranges, widths and flags.
- Red dashed lines mark target widths; dark ticks mark actual extents.
- Each row reports target/actual/residual width, capacity status, ratio, badness
  and winning fill order. Underfull finite overstretch can have zero residual.

Rows traverse stored physical knot order, including for RTL. Every row is
anchored at a common **diagnostic origin**: this is not page alignment or a claim
about which physical edge receives an RTL residual. Heights and spacing are
schematic. Source text, glyph outlines and fonts are not embedded; SVG label fonts
have no bearing on the measured widths. Zero-width knots appear as vertical marks.

## Reuse With A Real Backend

The separate `github.com/npillmayer/khipu/debug` package accepts a real frozen
result using public queries only:

```go
err := debug.WriteFrozenSVG(writer, frozen, debug.SVGOptions{
    Title: "Paragraph diagnostic",
    Scale: 2, // SVG units per big point; zero uses this default
})
```

Output is deterministic and XML-escaped. The function does not mutate, reshape,
rebreak, or access private arenas. A downstream module can supply real fonts and
shaping while reusing this writer. Machine-readable snapshots, input-plane views,
glyph proofs and a browser workbench are intentionally deferred.
