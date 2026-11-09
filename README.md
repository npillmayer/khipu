# Khipu: From Text to Typesetting

Khipu encodes styled text into SOA paragraphs and breaks them into lines using
firstfit or Knuth–Plass. Widths, source spans, flags, and penalties are stored in
aligned arrays; discretionary alternatives are discovered lazily.

## Metaphor

> “Khipu were recording devices fashioned
> from strings historically used by a number of cultures in the region of
> Andean South America.
> Khipu is the word for »knot« in Cusco Quechua.
> A khipu usually consisted of cotton or camelid fiber strings. The Inca
> people used them for collecting data and keeping records, monitoring tax
> obligations, properly collecting census records, calendrical information,
> and for military organization. The cords stored numeric and other values
> encoded as knots, often in a base ten positional system. A khipu could
> have only a few or thousands of cords.”
> ––Excerpt from a Wikipedia article about khipus

The Khipukamayuq (Quechua for “knot-makers”) were the scribes of those
times, tasked with encoding tax figures and other administrative
information in knots.
We will use this analogy to call typesetting items “khipus” or “knots,”
and objects which produce khipus will be “Khipukamayuq”s (I admit this is a tough one).
Knots implement items for typesetting paragraphs.

## Create Khipus from Text

A Khipukamayuq is part of a typesetting pipeline and will transform text into khipus.
Khipus are the input for linebreakers. The overall process of creating
them and the interaction with line breaking is fairly complicated, especially if
one considers international scripts.

We will use a box-and-glue model and the various knot types more or less
implement the corresponding node types from the TeX typesetting system.
The diagram below depicts an approximation of the overall task flow for typesetting
a single paragraph.

<div style="width:480px;padding:5px;padding-bottom:10px">
<img alt="typeset a single paragraph" src="http://npillmayer.github.io/TySE/images/khipukamayuq.svg" width="480px">
</div>

## Encode And Break A Paragraph

This complete example uses the built-in synthetic streaming shaper. It is also exercised
by [the public examples](example_test.go).

```go
package main

import (
	"context"
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/khipu/linebreak/knuthplass"
	"github.com/npillmayer/uax/bidi"
)

func main() {
	text := styled.TextFromString("Hello world")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		panic(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	khp, err := owner.EncodeParagraph(context.Background(), para)
	if err != nil {
		panic(err)
	}
	shape := linebreak.RectangularParShape(100 * dimen.PT)
	greedyLayout := firstfit.DefaultLayout(shape)
	greedy, err := firstfit.BreakParagraph(khp, greedyLayout, nil)
	if err != nil {
		panic(err)
	}
	optimalLayout := knuthplass.DefaultLayout(shape)
	optimal, err := knuthplass.BreakParagraph(khp, optimalLayout, nil)
	if err != nil {
		panic(err)
	}
	last := khp.KnotByIndex(optimal[len(optimal)-1])
	fmt.Println("firstfit lines:", len(greedy))
	fmt.Println("Knuth-Plass lines:", len(optimal))
	fmt.Println("source end:", last.Pos, "trailer bytes:", last.Len)
}
```

Both breakers return zero-based indices of line-ending knots. The final index
identifies the generated paragraph trailer. Source knots have absolute UTF-8 byte
positions (`Pos`, including `Paragraph.Offset`) and byte lengths (`Len`). A
selected discretionary is an alternative inside a source textbox; consult
`SelectedDiscretionaries` as well as the returned indices. Rerunning a breaker
replaces those selections.

## Configure Shaping And Hyphenation

Supply `Config.Streaming` with a fresh-stream factory, style resolver and run
defaults for font-backed shaping, and `Config.Hyphenator` for lazy alternatives.
Omitting `Streaming` selects synthetic shaping for unstyled text; an explicitly
supplied configuration is not filled with synthetic defaults. Owners must not be
used concurrently. Omitting `Params` selects defaults; a supplied `*Params` is
copied in full, including zero fields.

To enable lazy discovery, pass the same owner as the breaker's provider:

```go
params := knuthplass.NewDefaultParameters()
params.DiscretionaryProvider = owner
layout := knuthplass.DefaultLayout(shape)
breaks, err := knuthplass.BreakParagraph(khp, layout, params)
```

Firstfit has its own `Parameters.DiscretionaryProvider` field. The
[hyphenation example](example_test.go) demonstrates the streaming roundtrip.
Automatic alternatives must supply an original
request-relative `SourceSplit`; see [hyphenation](doc/HYPHENATION.md).
Without a provider, a breaker can use candidates already cached on `khp`.

`EncodeParagraph` appends a non-discardable fill knot carrying the final forced
break, including for empty paragraphs. Configure its dimensions through
`Config.ParFillSkip`; do not append another trailer. Text retains its original
bytes; normalize before constructing styled ranges if needed. Streaming shapers
report logical-order glyphs with scalar cluster anchors. Use `BreakpointPositions`
to map the chosen breaks back to original paragraph byte endpoints.

The default backend uses `helpers.SyntheticFont` at 10 pt,
`DefaultStyleResolver`, and a fresh `SyntheticStreamingShaper` per paragraph or
fragment. Ordinary cells are 5 pt and spaces are elastic. It requires no installed
fonts, rejects application-defined styles, and is not real font shaping.
Glyph/source provenance is retained. Logical/visual line materialization and
glue-only freezing exist internally; the public freezing result contract remains
future work. See the [testing guide](doc/TESTING.md) for helper policies.

The metric encoder, `Config.Shaper`, `FixedWidthShaper` and `KnotStyler` have
been removed. The sole encoding entry point is `EncodeParagraph(ctx, para)`;
there is no width-adaptation replacement hook.

### Integration And Ownership

Supply a complete `Config.Streaming` for an application backend. Its factory must
return a fresh stream for every encoding or reshaping request; missing services
or invalid defaults fail rather than silently falling back. The resolver maps
styles to font, size, script, language, features and spacing. Defaults apply
independently to original style spans before clipping to bidi intersections.
See [service contracts](contracts/ReadMe.md) for scalar anchors, glyph ordering,
font-unit conversion and stream lifecycle requirements.

Configuration feature storage, Params and ParFillSkip are copied; collaborator
instances and factories are retained. Serialize owner operations and keep source
text/styles and services stable while paragraphs may be reshaped. Nil Params
selects defaults, while supplied values preserve explicit zeros. Nil ParFillSkip
selects zero natural/minimum width and infinite maximum width; supplied values
preserve explicit zero widths.

Encoding preserves original UTF-8 source bytes. Normalize before establishing
styled ranges if desired. Paragraph-wide UAX#14/UAX#29 boundaries and logical
bidi/style intersections feed the streaming shaper. Cluster boundaries alone do
not authorize line breaks. Textboxes retain glyphs, shaping contexts and source
spans; glue retains metrics and source metadata, not glyphs. Invalid UTF-8,
anchors, width overflow or backend failures return no partial paragraph.
Unsupported mandatory breaks fail explicitly. Every paragraph, including empty
input, ends with a non-discardable fill carrying a forced-break penalty.

U+00AD is suppressed before shaping while its source position is retained.
Discretionary alternatives are shaped lazily through the owner, using retained
run parameters, and published transactionally. Breakers return knot indices and
publish selected variants only on success; failed runs preserve prior selections.
`BreakpointPositions` maps decisions to original paragraph-relative byte endpoints,
including whitespace and glyphless gaps. Selected variants require `SourceSplit`,
independent of replacement spelling. Manual knots without retained provenance
cannot be mapped; Pos/Len is not a fallback. See [hyphenation](doc/HYPHENATION.md).

## Package Structure

| Package | Responsibility |
| --- | --- |
| `khipu` | Paragraph storage, encoding, discretionaries, configuration, and the paragraph-specific `Hyphenator` contract |
| [`contracts`](contracts/ReadMe.md) | Font, streaming shaping, and style interfaces, their value types and constants, and the shared argument-error sentinel |
| [`helpers`](helpers/ReadMe.md) | The three deterministic defaults, with their focused tests and standalone examples |
| `dimen` | Dimensions and units |
| `linebreak` and its subpackages | Paragraph shapes, width summaries, and linebreaking algorithms |

The engine imports `contracts` and `helpers`; `helpers` imports `contracts`.
Neither subpackage imports the root engine, and `contracts` does not import
implementations. New adapters can depend on `contracts` without importing
paragraph storage or default implementations.

Import service types and constants from `contracts`, and synthetic implementations,
constructors and shaper lifecycle errors from `helpers`. The root package does
not re-export these APIs. `khipu.ErrIllegalArguments` shares the contract sentinel.

## Development And Release

The module declares Go 1.25.6 and currently replaces `cords` and `uax` with sibling
checkouts (`../cords`, `../uax`). Validation uses those local dependencies:

```sh
go test ./... -count=1 -timeout=30s
go vet ./...
go build ./...
go mod tidy -diff
```

SOA is the only paragraph model. The legacy AOS APIs and `linebreak/kp` have been
removed. See the [testing guide](doc/TESTING.md#dependency-release-checks) for release
limitations, and [the documentation index](doc/README.md) for implemented
contracts, design history, and future work.
