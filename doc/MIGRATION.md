# Migration To The SOA Paragraph Model

Status: November 4, 2026. The public configuration boundary, shared
infrastructure extraction, and SOA encoder-to-breaker integration are implemented
(steps 1–3).
Legacy storage, encoding, and the `linebreak/kp` package remain available until
later removal steps. This document distinguishes today's API from planned
retirement; it does not promise equivalent behavior between the two breakers.

## Public Construction And Configuration

The constructor now accepts an exported configuration value:

```go
func NewKhipukamayuq(config Config) *Khipukamayuq

type Config struct {
    Shaper     Shaper
    Hyphenator Hyphenator
    Params     *Params
    ParFillSkip *KnotCore
}
```

This is a source-level signature change from `NewKhipukamayuq(env typEnv)`.
The former argument type and its fields were private to this package, preventing
ordinary external callers from constructing a configured owner. Internal callers
now use `Config`; `typEnv` remains internal for the existing implementations.
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

A `Khipukamayuq` is not safe for concurrent use. It changes the shaper's source
and style during encoding and discretionary lookup. Do not use the same shaper
concurrently through another owner or directly. Collaborator interfaces have no
close operation; any backend resource lifetime remains the caller's responsibility.

The active encoding path still uses `Shaper`. `StreamingShaper` and the font
interfaces remain draft boundaries and are not wired into this constructor.

## Surviving API And Planned Replacements

The chosen model consists of `Khipu`, `KnotCore`, `KnotType`, `KnotFlags`,
`KnotStyler`, and `Khipukamayuq`, with lazy `Hyphenator`/discretionary support.
Both `linebreak/firstfit` and `linebreak/knuthplass` operate on this model and share
`linebreak.ParShape` and `linebreak.WSS`.

| Legacy API or usage | Migration target | Status / limitation |
| --- | --- | --- |
| `KhipuAOS`, `NewKhipu`, `KnotEncode`, package-level `EncodeParagraph` | Configure an owner and call `(*Khipukamayuq).EncodeParagraph` with a `styled.Paragraph` | SOA encoding exists; raw-reader and manual-list construction have no drop-in replacement |
| `Knot` and concrete `TextBox`, `Glue`, `Kern` objects | Indexed SOA planes and `KnotCore` values | Modern representation exists; `KnotCore` is a value snapshot, not a mutable reference into storage |
| `PenaltyItem`, standalone discretionary knots | `KnotCore.Penalty`, `DiscretionaryCandidate`, and `DiscretionarySelection` | Modern storage exists; legacy kinds still await cleanup |
| `Cursor`, `Mark`, `NewCursor`, `FixedWidthCursor` | Integer knot indices and `KnotByIndex` | Modern traversal exists; shaping supplies metrics upstream |
| `HyphenateTextBoxes`, concrete `HyphenateWord` helper | `Config.Hyphenator` and `(*Khipukamayuq).DiscretionaryCandidates` | Lazy interface exists; provide a concrete hyphenator implementation |
| `linebreak/kp.BreakParagraph` | `linebreak/knuthplass.BreakParagraph` with modern parameters | Returns indices and records selected discretionaries; costs and results may differ |
| `linebreak/kp.FindBreakpoints` | Modern single-best-path result | Enumeration of all line-count solutions is intentionally not provided |
| Legacy list mutation, measurement, text extraction, and box height/depth helpers | Review each consumer against source spans, SOA widths, and the future frozen representation | No blanket compatibility API is planned; freezing and glyph provenance are still separate work |

No legacy APIs are deleted by steps 1–3. Their eventual removal
will be an intentional API break. Known downstream callers must be checked before
release; consumers outside this repository have not been audited.

## Shared Infrastructure Extraction

Shared definitions now live outside the legacy storage and encoder files:

| File | Responsibility |
| --- | --- |
| `environment.go` | Internal encoding collaborators, parameters, and pipeline state |
| `pipeline.go` | Legacy NFC input normalization and segmenter setup |
| `penalties.go` | Segmenter penalty pairs, SOA penalty type and sentinels, and bounded conversion |
| `knot_types.go` | Knot-kind identifiers shared by both representations |

The unused line-wrapper field, redundant stored word-breaker pointer, and unused
`primaryBreak` helper have been removed. The word scanner remains because legacy
text encoding and eager hyphenation still use it; `typEnv.levels` likewise remains
for the legacy text encoder. These dependencies are marked for later retirement.

Step 2 preserved public signatures, knot-kind numeric values, segmentation rules,
and penalty conversion behavior. Step 3 replaces the SOA encoder's use of that
legacy pipeline with the contract below. Legacy segmenter reuse is unchanged.

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

Both modern breakers consume this output directly. Width-summary arithmetic
saturates instead of wrapping when paragraph fill is added to finite text/skips.
Knuth–Plass retires earlier origins after a mandatory break, retains an infeasible
fallback when necessary, and accumulates costs in 64 bits for repeated fallbacks.
These are linebreaking summaries, not freeze-time allocation of fill orders.

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
durable discretionary glyph/source provenance remain separate work. Legacy
coverage migration and implementation removal follow in later steps.

The known root-package baseline failures remain `TestKhipu` (unconditional failure)
and `TestBreaking1` (legacy item-count expectation). They are not hidden or removed
by the API migration. Freezing remains future work described in [FREEZE.md](FREEZE.md).
