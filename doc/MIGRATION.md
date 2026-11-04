# Migration To The SOA Paragraph Model

Status: November 4, 2026. The public configuration boundary and shared
infrastructure extraction are implemented (steps 1 and 2).
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

No legacy APIs are deleted by the configuration or extraction changes. Their eventual removal
will be an intentional API break. Known downstream callers must be checked before
release; consumers outside this repository have not been audited.

## Shared Infrastructure Extraction

Shared definitions now live outside the legacy storage and encoder files:

| File | Responsibility |
| --- | --- |
| `environment.go` | Internal encoding collaborators, parameters, and pipeline state |
| `pipeline.go` | NFC input normalization and segmenter setup |
| `penalties.go` | Segmenter penalty pairs, the SOA penalty type and sentinels, and existing conversion logic |
| `knot_types.go` | Knot-kind identifiers shared by both representations |

The unused line-wrapper field, redundant stored word-breaker pointer, and unused
`primaryBreak` helper have been removed. The word scanner remains because legacy
text encoding and eager hyphenation still use it; `typEnv.levels` likewise remains
for the legacy text encoder. These dependencies are marked for later retirement.

Public signatures, knot-kind numeric values, segmentation rules, and penalty
conversion behavior are unchanged by this extraction. Pipeline reuse still retains
existing segmenter state. Correcting reset behavior, penalty mapping, shaping, and
paragraph termination belongs to step 3, where integration tests will establish
the SOA encoder-to-breaker contract.

## Validation Boundary And Follow-Up

External-package tests exercise public construction, configured shaper error
propagation, and cached candidates without a hyphenator. Internal tests exercise
fragment measurement, configured hyphenator use, parameter-copy isolation, nil
defaults, explicit zero parameters, and independent default shapers. Existing
modern breaker tests remain applicable.

These steps do not resolve the default shaper's EOF/run lifecycle, UAX penalty
mapping, or automatic creation of the paragraph trailer expected by the breakers.
Those require encoder-to-breaker integration work before legacy retirement.
See [KP.md](KP.md) and [HYPHENATION.md](HYPHENATION.md) for current contracts.

A single unstyled paragraph containing `breaking`, with a custom shaper emitting
one nonzero metric per ASCII byte and a clean EOF, also exposed the encoder panic
`segment extends beyond metrics array`. Segmentation/boundary handling for that
case belongs to the encoder integration work; successful general encoding is not
an acceptance claim for this configuration change.

The known root-package baseline failures remain `TestKhipu` (unconditional failure)
and `TestBreaking1` (legacy item-count expectation). They are not hidden or removed
by the API migration. Freezing remains future work described in [FREEZE.md](FREEZE.md).
