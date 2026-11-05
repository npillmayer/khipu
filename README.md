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

This complete example uses the built-in fixed-width shaper. It is also exercised
by [the public examples](example_test.go).

```go
package main

import (
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
	khp, err := owner.EncodeParagraph(para, nil)
	if err != nil {
		panic(err)
	}
	shape := linebreak.RectangularParShape(100 * dimen.PT)
	greedy, err := firstfit.BreakParagraph(khp, shape, nil)
	if err != nil {
		panic(err)
	}
	optimal, err := knuthplass.BreakParagraph(khp, shape, nil)
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

Supply `Config.Shaper` for font-backed metrics and `Config.Hyphenator` for lazy
alternatives. The collaborators are retained, so a stateful shaper must not be
used concurrently with its owner. Omitting `Params` selects defaults; a supplied
`*Params` is copied in full, including zero fields.

To enable lazy discovery, pass the same owner as the breaker's provider:

```go
params := knuthplass.NewKPDefaultParameters()
params.DiscretionaryProvider = owner
breaks, err := knuthplass.BreakParagraph(khp, shape, params)
```

Firstfit has its own `Parameters.DiscretionaryProvider` field. The
[hyphenation example](example_test.go) demonstrates the complete roundtrip.
Without a provider, a breaker can use candidates already cached on `khp`.

`EncodeParagraph` appends a non-discardable fill knot carrying the final forced
break, including for empty paragraphs. Configure its dimensions through
`Config.ParFillSkip`; do not append another trailer. Text retains its original
bytes; normalize before constructing styled ranges if needed. Custom shapers
report run-relative byte offsets in logical order, including zero-width cluster
starts. See [the encoding contract](doc/MIGRATION.md#soa-encoder-to-breaker-contract).

The default shaper is a fixed-width metric provider, not a font shaping backend.
Linebreaking does not assign final glyph positions. Freezing, durable glyph
provenance, and script-sensitive spacing remain future work. The public
`helpers.FixedWidthShaper` supports a configurable positive cell width through
`helpers.NewFixedWidthShaper(width)`; its zero value uses the existing 5 pt default.
See the [executable encoding example](helpers_example_test.go).

The `helpers` package's `SyntheticFont`, `DefaultStyleResolver`, and
`SyntheticStreamingShaper` provide
standalone deterministic implementations for the draft font/style/streaming
interfaces. They require no installed fonts. Streaming is not yet integrated
into `Config` or paragraph encoding. See the [combined streaming example](helpers/synthetic_streaming_shaper_test.go)
for concurrent production/draining and the [testing guide](doc/TESTING.md)
for constructor choices, character policies, and lifecycle guarantees.

## Package Structure

| Package | Responsibility |
| --- | --- |
| `khipu` | Paragraph storage, encoding, discretionaries, configuration, and paragraph-specific `KnotStyler`/`Hyphenator` contracts |
| [`contracts`](contracts/ReadMe.md) | Font, metric/streaming shaping, and style interfaces, their value types and constants, and the shared argument-error sentinel |
| [`helpers`](helpers/ReadMe.md) | The four deterministic defaults, with their focused tests and standalone examples |
| `dimen` | Dimensions and units |
| `linebreak` and its subpackages | Paragraph shapes, width summaries, and linebreaking algorithms |

The engine imports `contracts` and `helpers`; `helpers` imports `contracts`.
Neither subpackage imports the root engine, and `contracts` does not import
implementations. New adapters can depend on `contracts` without importing
paragraph storage or default implementations.

The original root names remain as type aliases and forwarding constructors in
[compat.go](compat.go), including the original error identities. Prefer
`contracts` and `helpers` in new code. The split changes package ownership, not
`Config{}` behavior or the implemented shaping policies.

## Development And Migration

The module declares Go 1.25.6 and currently replaces `cords` and `uax` with sibling
checkouts (`../cords`, `../uax`). Validation uses those local dependencies:

```sh
go test ./... -count=1 -timeout=30s
go vet ./...
go build ./...
go mod tidy -diff
```

SOA is the only paragraph model. The legacy AOS APIs and `linebreak/kp` have been
removed. See [the migration guide](doc/MIGRATION.md) for API changes and release
limitations, and [the documentation index](doc/README.md) for implemented
contracts, design history, and future work.
