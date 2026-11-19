# Khipu: From Text to Typesetting

Context policy (2026-11-19): typesetting, geometry, retained proofs, replay,
and document execution are synchronous and have no context argument. Contexts
are confined to streaming coordination and font discovery/loading. Streaming
callers create a local background context; consumer failure still cancels and
joins the producer before cleanup. Errors publish no partial result and preserve
previous immutable output. TUI layout jobs finish serially; stale results are
discarded, while font work remains cancellable. This supersedes earlier
cancellation requirements for typesetting in this document.

Khipu prepares styled text for linebreaking, chooses line endings with firstfit
or Knuth–Plass, and freezes the decisions into final widths and visual knot order.
Its working paragraph stores widths, source spans, flags and penalties in aligned
arrays (SOA). Rendering and vertical box assembly belong to the application.

**The default backend is synthetic, not real font shaping.** `Config{}` works
without font files for examples and structural tests. Supply `Config.Streaming`
for application fonts and styles; font coverage and fallback are caller concerns.

Start with the example below, then [choose an input workflow](#choose-an-input-workflow).
The [package map](#package-structure) and [documentation index](doc/README.md)
lead to service contracts, detailed guides and developer history.

## Encode And Break A Paragraph

This complete text-only program encodes, breaks and freezes a paragraph. Its
body is also exercised by `ExampleKhipukamayuq_Freeze` in the
[public examples](example_test.go). Save it as `main.go` in a separate directory
in this checkout and run `go run .` there; see [local setup](#development-and-release).

```go
package main

import (
	"fmt"

	"github.com/npillmayer/cords/styled"
	"github.com/npillmayer/khipu"
	"github.com/npillmayer/khipu/dimen"
	"github.com/npillmayer/khipu/linebreak"
	"github.com/npillmayer/khipu/linebreak/firstfit"
	"github.com/npillmayer/uax/bidi"
)

func main() {
	text := styled.TextFromString("Hello world")
	para, err := styled.ParagraphFromText(&text, 0, text.Raw().Len(), bidi.LeftToRight, nil)
	if err != nil {
		panic(err)
	}
	owner := khipu.NewKhipukamayuq(khipu.Config{})
	input, err := owner.EncodeParagraph(para)
	if err != nil {
		panic(err)
	}
	// Resolve geometry once; freeze the chosen decisions before another breaker
	// run replaces the paragraph's selected discretionaries.
	layout := firstfit.DefaultLayout(linebreak.RectangularParShape(100 * dimen.PT))
	breaks, err := firstfit.BreakParagraph(input, layout, nil)
	if err != nil {
		panic(err)
	}
	frozen, err := owner.Freeze(input, breaks, layout)
	if err != nil {
		panic(err)
	}
	line, _ := frozen.Line(0)
	fmt.Println("lines:", frozen.LineCount())
	fmt.Println("target reached:", line.Width == line.TargetWidth)
	fmt.Println("source bytes:", line.Source.End-line.Source.Start)
}
```

This reports `lines: 1`, `target reached: true`, and `source bytes: 11`. Resolve
the layout once and pass it unchanged to breaking and freezing. For paragraph-wide
optimization, use `knuthplass.DefaultLayout` and `knuthplass.BreakParagraph`;
[the examples](example_test.go) also demonstrate both breakers.

## Choose An Input Workflow

| Need | Workflow | Guide |
| --- | --- | --- |
| One text-only layout | `EncodeParagraph` → break → `Freeze` | Quick start above |
| Intrinsic widths, inline objects or reusable shaping | `PrepareParagraph` → optional `MeasureIntrinsic` → `BindParagraph` → break → `Freeze` | [Prepared paragraphs](doc/PREPARED_PARAGRAPHS.md) |

### Public Lifecycle

![Public lifecycle: encode text or prepare and bind it, then break and freeze using the same layout parameters.](doc/diagrams/public-lifecycle.svg)

[Graphviz DOT source](doc/diagrams/public-lifecycle.dot).

Measurement does not consume the preparation. Each binding has independent
working decisions; a successful breaker rerun replaces that binding's selections.
Freeze uses both the indices and those selections. Owner operations are serialized,
and source/font services remain stable. The [ownership diagram](doc/PREPARED_PARAGRAPHS.md#ownership-and-reuse)
shows what is copied and what remains shared.

For fixed-prefix flow with `NoHyphenation`, certify a complete successful path
using `owner.Checkpoint(prepared, breaks, acceptedLines)`, bind the same
preparation again, and call `firstfit.ContinueParagraph`. The opaque checkpoint
locks logical boundaries; only the suffix is searched. KP prefixes are supported.
Keep the original width schedule for accepted lines when freezing the complete
result. See [checkpoint continuation](doc/PREPARED_PARAGRAPHS.md#checkpoint-continuation)
for the admission and lifetime contract. Frame heights and flow dispatch belong
to the caller; this API does not paginate documents.

### Fixed-Size Atomic Inline Items

Attach paragraph-local IDs to U+FFFC markers at their UTF-8 byte offsets. Prepare
once, then bind fixed signed advances for each occurrence. Rebinding reuses
shaped text and creates independent working decisions. Khipu keeps horizontal
advance and source identity; callers keep payloads and vertical geometry.
See the [anchor rules and complete example](doc/PREPARED_PARAGRAPHS.md#anchors-and-atomic-boundaries).

### Intrinsic Measurement

Measure a preparation without choosing line geometry or binding atomic widths.
Supply one min/max advance range per attached ID. Ordinary and bounded explicit
hyphenation are supported; automatic intrinsic hyphenation is not. See
[measurement semantics](doc/PREPARED_PARAGRAPHS.md#measurement-and-binding-contracts)
and [policy limits](doc/PREPARED_PARAGRAPHS.md#hyphenation-policies).

## Package Structure

| Package | Responsibility |
| --- | --- |
| `khipu` | Paragraph storage, encoding, discretionaries, configuration, and the paragraph-specific `Hyphenator` contract |
| [`contracts`](contracts/ReadMe.md) | Font, streaming shaping, and style interfaces, their value types and constants, and the shared argument-error sentinel |
| [`helpers`](helpers/ReadMe.md) | The three deterministic defaults, with their focused tests and standalone examples |
| [`dimen`](dimen/ReadMe.md) | Fixed-point dimensions, units, and page geometry |
| [`linebreak`](linebreak/ReadMe.md) | Paragraph shapes and width summaries |
| [`firstfit`](linebreak/firstfit/ReadMe.md) | Greedy linebreaking and local discretionary rescue |
| [`knuthplass`](linebreak/knuthplass/ReadMe.md) | Paragraph path search and scoring profiles |
| [`debug`](debug/ReadMe.md) | Frozen-width SVG diagnostics |
| [`khipu-cli`](khipu-cli/README.md) | Command-line synthetic layout proofs |

The engine imports `contracts` and `helpers`; `helpers` imports `contracts`.
Neither subpackage imports the root engine, and `contracts` does not import
implementations. New adapters can depend on `contracts` without importing
paragraph storage or default implementations.

Import service types and constants from `contracts`, and synthetic implementations,
constructors and shaper lifecycle errors from `helpers`. The root package does
not re-export these APIs. `khipu.ErrIllegalArguments` shares the contract sentinel.

## Configure Shaping And Hyphenation

Supply a complete `Config.Streaming` for font-backed shaping. Omitting it selects
`helpers.SyntheticFont` at 10 pt, `DefaultStyleResolver`, and a fresh
`SyntheticStreamingShaper` per paragraph or replacement. Ordinary synthetic
cells advance 5 pt, spaces are elastic, and application-defined styles are rejected.
An explicitly supplied configuration is not filled with synthetic defaults.

Lazy alternatives need `Config.Hyphenator` and the owner as the breaker's provider:

```go
params := knuthplass.NewDefaultParameters()
params.DiscretionaryProvider = owner
layout := knuthplass.DefaultLayout(shape)
breaks, err := knuthplass.BreakParagraph(input, layout, params)
```

This fragment assumes an encoded/bound `input`, a configured `owner` and `shape`.
Firstfit has its own provider field. A prepared paragraph also fixes its
[hyphenation mode](doc/PREPARED_PARAGRAPHS.md#hyphenation-policies) at preparation.
`Config{}` supplies no alternatives, including for soft hyphens. Without a
provider, breakers can use candidates already cached on their input. The
[hyphenation guide](doc/HYPHENATION.md) owns replacement/penalty rules and links
to an executable provider example.

## Integration And Ownership

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

## Freezing

Freeze the complete breakpoint list with its matching selected discretionaries,
before another successful breaker run replaces those selections. `FrozenKhipu`
provides line/knot records, atomic identities, copied shaping snapshots and
bounded source readers. Knots are in physical left-to-right order within each
line; glyph arrays within a box are not reversed.

Overfull and underfull lines are successful results with fit diagnostics. Only
glue adjusts; elastic non-glue material returns `ErrUnsupportedAdjustment`.
There is no unfreeze operation. Keep the original elastic input for another
break/freeze run. Read [the freezing guide](doc/FREEZE.md) for allocation and
[the result contract](doc/FROZEN-API.md) for queries, coordinates and lifetime.

## Visual Diagnostics

`khipu-cli view` runs synthetic encoding, line breaking and freezing and writes a
standalone SVG of final advance widths, glue, skips, fill and fit diagnostics:

```sh
printf 'Hello world, a structural proof.' | go run ./khipu-cli view --width 90 --output proof.svg
```

This is a structural diagram, not glyph rendering. It requires no OpenType or
installed fonts. Downstream clients can visualize their own frozen results with
`debug.WriteFrozenSVG`. See [the CLI guide](khipu-cli/README.md) for options,
interpretation and limitations.

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

## More Information

The name and knot terminology stem from Andean recording devices. Find out more
about the original
[metaphor and conceptual illustration](doc/history/BACKGROUND.md) in file
"`doc/history/BACKGROUND.md`".

Implementation records and unimplemented proposals are
listed separately in the [history index](doc/history/README.md).

## Font acceptance before shaping

`contracts.StreamingShaper` embeds `FontAcceptor.CheckFace`. Encoding checks every
resolved run on its actual stream before any glyph production; lazy replacements
check their own fresh streams. Acceptance is read-only and may support multiple
face types. Failed checks publish no partial paragraph/replacement, and acquired
streams are finished after any producer has joined. Synthetic defaults and
nongeneric prepared/frozen result APIs are unchanged. See
[the contract](contracts/ReadMe.md#face-compatibility) for backend obligations.


### Production continuation verification (P6)

Typography's owning-package benchmarks exercise production `Checkpoint` and
`firstfit.ContinueParagraph` through `Prepared.StartFlow`/`Continuation.Fit`, with
separate component measurements of checkpoint certification and suffix breaking.
See the [production cost report](../typography/doc/FLOW_COSTS.md) for five-process
evidence and the bounded application profile. No benchmark hook or serialized
checkpoint is added to khipu. Certification still checks the complete path;
seeded search skips accepted prefix knots, while binding/freezing may process the
whole paragraph. Release preparation and checkpoints when a paragraph completes
or is abandoned; retain only frozen fragments. The upstream synchronous bidi
preparation limitation remains unfixed and is not a frame-boundary defect.
