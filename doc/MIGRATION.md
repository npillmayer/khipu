# Migration To The SOA Paragraph Model

Status: November 4, 2026. The public configuration boundary, shared
infrastructure extraction, SOA encoder-to-breaker integration, coverage migration,
legacy implementation removal, and final documentation/validation review are
complete (steps 1–6). SOA is the only
paragraph representation. The removed APIs below are intentional source-level
breaks; this guide does not promise behavioral parity with the old breaker.

## Public Construction And Configuration

The constructor now accepts an exported configuration value:

```go
func NewKhipukamayuq(config Config) *Khipukamayuq

type Config struct {
    Shaper      Shaper
    Hyphenator  Hyphenator
    Params      *Params
    ParFillSkip *KnotCore
}
```

This is a source-level signature change from `NewKhipukamayuq(env typEnv)`.
The former argument type and its fields were private to this package, preventing
ordinary external callers from constructing a configured owner. Internal callers
now use `Config`; `typEnv` remains internal for SOA encoding.
Segmentation and bidi pipeline internals are not exposed through configuration.

| Field | When nil | Ownership when supplied |
| --- | --- | --- |
| `Shaper` | A fresh built-in fixed-width shaper per owner | The interface value is retained; its underlying implementation is not copied |
| `Hyphenator` | No lazy discovery; cached candidates remain usable | The interface value is retained; its underlying implementation is not copied |
| `Params` | A fresh set of existing defaults per owner | The pointed-to value is copied at construction |
| `ParFillSkip` | Zero natural/minimum width and `dimen.Fill` maximum width | The value is copied; only its three width fields configure the trailer |

A non-nil `Params` replaces the full parameter set. Zero fields are intentional
values, not requests for defaults. Changing the caller's parameters after
construction does not change the owner. Hyphenators receive the owner's parameter
copy as read-only input and should make a value copy if they retain it.

The existing defaults remain unchanged: English, Latin script, left-to-right
direction, 12 pt baseline skip, zero line skip and line-skip limit, `'-'` as the
hyphen character, hyphen penalty 10, and minimum hyphen length 2. These are existing
parameter values, not a claim that every parameter is consumed by the SOA path.
`Params` is passed to the supplied hyphenator; the module itself uses
`Hyphenpenalty` as the fallback fragment penalty. The remaining fields are not
automatically applied to shaping, spacing, or hyphenation filtering. Breaker
parameters are separate types owned by each linebreaking package.

A `Khipukamayuq` is not safe for concurrent use. It changes the shaper's source
and style during encoding and discretionary lookup. Do not use the same shaper
concurrently through another owner or directly. Collaborator interfaces have no
close operation; any backend resource lifetime remains the caller's responsibility.

Encoding and lazy fragment measurement use `Shaper`. `StreamingShaper` and the font
interfaces remain draft boundaries and are not wired into this constructor.

## Removed APIs And Migration Targets

The later service-package split (November 5, 2026) preserves source-level names
through aliases and forwarding constructors in the root package:

| Canonical package | Moved APIs |
| --- | --- |
| `contracts` | Font, shaping, and style interfaces; their parameter/result types and constants; `ErrIllegalArguments` |
| `helpers` | `SyntheticFont`, `FixedWidthShaper`, `DefaultStyleResolver`, `SyntheticStreamingShaper`, their constructors, and streaming lifecycle errors |

New code should import these packages directly. Existing root signatures remain
compatible because the aliases preserve type identity; error matching also
retains sentinel identity. Reflection reports the types' new defining packages.
`Config{}` still constructs a fresh 5 pt metric shaper. The draft streaming
services remain outside the active encoder. Paragraph-specific `KnotStyler`,
`Hyphenator`, and their associated model types stay in the root package.

The older removals described below are separate from this package split.

The chosen model consists of `Khipu`, `KnotCore`, `KnotType`, `KnotFlags`,
`KnotStyler`, and `Khipukamayuq`, with lazy `Hyphenator`/discretionary support.
Both `linebreak/firstfit` and `linebreak/knuthplass` operate on this model and share
`linebreak.ParShape` and `linebreak.WSS`.

| Removed API or usage | Migration target | Status / limitation |
| --- | --- | --- |
| `KhipuAOS`, `NewKhipu`, `KnotEncode`, package-level `EncodeParagraph` | Configure an owner and call `(*Khipukamayuq).EncodeParagraph` with a `styled.Paragraph` | SOA encoding exists; raw-reader and manual-list construction have no drop-in replacement |
| `Knot` and concrete `TextBox`, `Glue`, `Kern` objects | Indexed SOA planes and `KnotCore` values | Modern representation exists; `KnotCore` is a value snapshot, not a mutable reference into storage |
| `PenaltyItem`, standalone discretionary knots | `KnotCore.Penalty`, `DiscretionaryCandidate`, and `DiscretionarySelection` | Standalone penalty/discretionary kinds are removed |
| `Cursor`, `Mark`, `NewCursor`, `FixedWidthCursor` | Integer knot indices and `KnotByIndex` | Modern traversal exists; shaping supplies metrics upstream |
| `HyphenateTextBoxes`, concrete `HyphenateWord` helper | `Config.Hyphenator` and `(*Khipukamayuq).DiscretionaryCandidates` | Lazy interface exists; provide a concrete hyphenator implementation |
| `linebreak/kp.BreakParagraph` | `linebreak/knuthplass.BreakParagraph` with modern parameters | Returns indices and records selected discretionaries; costs and results may differ |
| `linebreak/kp.FindBreakpoints` | Modern single-best-path result | Enumeration of all line-count solutions is intentionally not provided |
| List types `HList`/`VList`/`MList`, list mutation, measurement, text extraction, and box height/depth helpers | Review each consumer against source spans, SOA widths, and the future frozen representation | No blanket compatibility API is planned; freezing and glyph provenance are still separate work |

Step 5 removes these APIs without compatibility aliases. `NewGlue`/`NewFill`
and `NewKnot` factories are also removed: construct `KnotCore` values for widths
and flags, and configure paragraph fill with `Config.ParFillSkip`. The legacy
`TypesettingPipeline`/`PrepareTypesettingPipeline` API has no replacement; the SOA
encoder owns boundary detection and accepts an already styled paragraph.
Consumers outside this repository have not been audited.

## Implementation After Removal

| File | Responsibility |
| --- | --- |
| `khipu.go` | SOA storage, indexed knot values, and owner construction (formerly `khipu_soa.go`) |
| `environment.go` | Internal shaping/hyphenation collaborators, parameters, and paragraph fill |
| `encoding.go`, `encoding_boundaries.go`, `encoding_metrics.go` | Original-source encoding, paragraph-wide UAX#14 boundaries, and cluster metrics |
| `penalties.go` | Penalty type, sentinels, and bounded conversion of accumulated UAX contributions |
| `knot_types.go` | Supported SOA knot kinds |

The AOS storage, legacy encoder, cursors, eager hyphenation, normalization/word
pipeline, and `linebreak/kp` package have been deleted. Their environment fields
and two-part segmenter penalties are gone. The obsolete graph snapshots were
removed; the referenced Knuth–Plass paper is retained as [knuthplass.pdf](knuthplass.pdf).

`KTKern`, `KTGlue`, `KTTextBox`, and `KTUserDefined` retain values 0, 1, 2, and 5.
Values 3 and 4 remain reserved after removing `KTPenalty` and `KTDiscretionary`.
Penalties live on knots; hyphenation uses candidate/selection tables. Modern
breaker fixtures use custom retained knots where they need neutral markers.

`go mod tidy` removed the direct `opentype`, `hyphenate`, and `gods` requirements
and the unused `x/image` requirement. The SOA `Hyphenator`, `Shaper`, draft
`StreamingShaper`, and font interfaces remain backend-independent. Existing local
module replacements for `cords` and `uax` are unchanged.

## SOA Encoder-To-Breaker Contract

`EncodeParagraph` preserves the original UTF-8 bytes. `Pos` is an absolute byte
offset including `Paragraph.Offset`; `Len` is the byte length. Source spans cover
the paragraph exactly, including spaces and hard-break characters. There is no
implicit NFC normalization: normalize before constructing styled ranges if needed.

Line boundaries are evaluated with a fresh UAX#14 breaker over the complete
paragraph, then intersected with style runs and shaped cluster boundaries.
Style boundaries alone do not authorize breaks. A cluster spanning a possible
break remains intact without dropping subsequent text. The legacy word segmenter
and its retained state are no longer used by SOA encoding.

`Shaper.SetSource` resets run-relative byte offsets. `ReadClusterMetrics` reports
the first UTF-8 byte of each cluster in nondecreasing logical order. Repeated
offsets accumulate width; zero-width metrics still identify cluster starts.
EOF carries no metric. Invalid offsets, width overflow, invalid UTF-8, source
spans exceeding the `uint16` length field, and shaper errors return an error with
no partial encoded paragraph. The default fixed-width shaper now terminates at
EOF and resets between runs and paragraphs.

Penalties preserve the `±10000` mandatory/prohibited sentinels and finite values
without narrowing through `int8`. Every paragraph, including an empty one, ends
with one generated trailer: `KTGlue`, no discardable flag, zero source length,
and `MaxMerit`. Its position is the original source end. The last source knot's
penalty is prohibited so EOF is represented only by the trailer. Interior forced
breaks remain in place. `Config.ParFillSkip` overrides only trailer widths and must
satisfy `MinW <= W <= MaxW`; an explicit zero value is honored. Breaker parameters
do not append a second trailer. `Unskip` retains this non-discardable knot.

Both modern breakers consume this output directly. They return zero-based
line-ending knot indices, including the final trailer. Rerunning a breaker
replaces `SelectedDiscretionaries`; retain a copy when comparing runs. Configure
its `DiscretionaryProvider` with the same owner to enable lazy discovery. Without
a provider, already cached alternatives can still be considered. Width-summary arithmetic
saturates instead of wrapping when paragraph fill is added to finite text/skips.
Knuth–Plass retires earlier origins after a mandatory break, retains an infeasible
fallback when necessary, and accumulates costs in 64 bits for repeated fallbacks.
These are linebreaking summaries, not freeze-time allocation of fill orders.

## Legacy Coverage Migration

Step 4 preserves useful legacy scenarios as SOA assertions. Tests no longer
construct AOS lists or depend on legacy cursors, marks, concrete knots, or eager
hyphenation. The two formerly failing root tests have been replaced, not skipped.

| Previous coverage | SOA coverage / disposition |
| --- | --- |
| `TestKhipu`: append kern, glue, and text; unconditional failure | `TestKhipuKnotSnapshots`: indexed values, stored dimensions/flags/source spans, snapshot isolation, and out-of-range access |
| `TestBreaking1`, `TestBreaking2`: counts of separate penalty items | `TestEncodedParagraphTextRegressions`: the same source cases, legal byte boundaries, content/space kinds, and discardability |
| `TestText`, `TestExHyphen`: source extraction and literal hyphen | `TestEncodedParagraphTextRegressions`: exact reconstruction from source spans, a permitted break after the literal hyphen, and no discretionary replacement |
| `TestParaSimple`: styled paragraph smoke test | `TestEncodedParagraphStyleAdaptation`: exact styled span and style-dependent widths on each source knot |
| `TestDimen` | `dimen.TestBigPointString`, colocated with dimension code |
| Legacy `TestGraph1`: breakpoint registration/lookup | Existing modern `TestPathTableAddBreakpoint`, with additional predecessor/cost/tie coverage in `path_test.go` |
| Legacy underfull, exact-fit, and wrapping cases | `TestModernBreakersParagraphRegressions`: both modern breakers, complete/legal break sequences, expected line counts, width bounds, and source preservation |
| Legacy king/princess paragraphs with flexible spaces | The same corpus in `TestModernBreakersParagraphRegressions`, with elasticity supplied through `KnotStyler` |

Legacy line-count enumeration, cursor-driven metric mutation, GraphViz test output,
and the test-only justification printer are not supported SOA contracts and were
retired with their fixtures. The commented-out HTML/DOM test had no executable
coverage and was removed; styled input is exercised directly. The redundant
test shaper was replaced by the repaired default shaper. Lazy hyphenation and
selection remain covered by the step-3 integration and discretionary tests.

Modern Knuth–Plass no longer declares the unused `feasibleBreakpoint` and
`provisionalMark` types or carries obsolete cursor-loop comments. Its runtime
algorithm was unchanged by step 4. Step 5 deleted the legacy implementation
after its useful coverage had moved to the SOA path.

## Validation Boundary And Follow-Up

External-package tests exercise public construction, configured shaper error
propagation, and cached candidates without a hyphenator. Internal tests exercise
fragment measurement, configured hyphenator use, parameter-copy isolation, nil
defaults, explicit zero parameters, and independent default shapers. Existing
modern breaker tests remain applicable.

Integration tests now cover default encoding into both modern breakers, empty and
whitespace-only paragraphs, UTF-8/decomposed text, mixed bidi runs, styled source
sections, repeated encoding, forced/prohibited boundaries, zero-width and merged
clusters, custom fill, and lazy discretionary discovery and selection. The earlier
`breaking` encoder panic is covered by the passing contract test.
See [KP.md](KP.md) and [HYPHENATION.md](HYPHENATION.md) for current contracts.

This establishes the SOA pipeline contract, not complete international-script
typesetting. Script-sensitive discardability, font-backed/streaming shaping, and
durable discretionary glyph/source provenance remain separate work.
See [the documentation index](README.md) for the boundary between implemented
contracts and future designs.

## Final Validation And Release Limits

Step 6 reviewed the surviving public API, documentation links, and examples.
The quickstart runs both breakers; a second example exercises configured lazy
hyphenation. Both are executable Go examples in `example_test.go`.

The final local validation commands are:

```sh
go test ./... -count=1 -timeout=30s
go vet ./...
go build ./...
go mod tidy -diff
git diff --check
```

The legacy package is absent and the coverage map above records its test
replacements. Supported Go source no longer declares or imports the removed
AOS APIs or `linebreak/kp`. Historical references remain explicitly labeled in
the design notes. Repository-local documentation links resolve.

These checks use the current sibling `cords` and `uax` checkouts selected by
`go.mod`, with Go 1.25.6 as the declared minimum. They do not establish a build
against published dependency versions or compatibility for downstream callers.
Before publishing a release, select and verify published dependency versions
that supply the required styled-paragraph and UAX APIs, and migrate affected
consumers. No dependency versions, local replacements, release tags, or remote
state were changed by this final review.

This is an intentional API-breaking migration, with no AOS compatibility layer.
The storage migration is complete; freezing, rendering, font-backed shaping,
script-specific spacing, and robust discretionary provenance are separate work
rather than promises of this release. See [FREEZE.md](FREEZE.md).
