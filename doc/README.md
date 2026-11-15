# Documentation Guide

Start with a client workflow, then follow its contract reference when you need
details. This index describes the supported module as of November 15, 2026;
implementation records and proposals are listed separately below.

## Start Here

| Task | Read |
| --- | --- |
| Encode, break and freeze text | [Runnable quick start](../README.md#encode-and-break-a-paragraph) |
| Use inline objects, measure widths or reuse shaping | [Prepared paragraphs](PREPARED_PARAGRAPHS.md), including a complete prepare/measure/bind/break/freeze example |
| Supply fonts, shaping or styles | [Integration and ownership](../README.md#integration-and-ownership), then [service contracts](../contracts/ReadMe.md) |
| Choose a breaker | [First-fit](../linebreak/firstfit/ReadMe.md) or [Knuth–Plass](../linebreak/knuthplass/ReadMe.md) |
| Configure scores or migrate legacy names | [Typed Knuth–Plass settings](../linebreak/knuthplass/ReadMe.md#configure-typed-values) and [compatibility guidance](../linebreak/knuthplass/ReadMe.md#compatibility-names) |
| Inspect final widths | [CLI diagnostics](../khipu-cli/README.md) or [debug library](../debug/ReadMe.md) |

`Config{}` uses synthetic shaping and supplies no hyphenation material. It is
suitable for examples and structural tests, not real typography. Complete public
examples in [example_test.go](../example_test.go) run as part of `go test`.
The [package map](../README.md#package-structure) includes units, helpers and
shared linebreaking utilities.

## Contract References

Each row owns the detailed rules for its topic. Other guides summarize the
workflow and link here; staged plans do not override these contracts. Public
declaration comments describe the exact call signatures and error behavior.

| Canonical home | Scope |
| --- | --- |
| [Prepared paragraphs](PREPARED_PARAGRAPHS.md) | Anchors, atomic boundary overrides, prepare/measure/bind, intrinsic semantics, supported policies and preparation lifetime |
| [Service contracts](../contracts/ReadMe.md) | Font/style/shaping interfaces, scalar coordinates, stream ordering and lifecycle; [helpers](../helpers/ReadMe.md) supplies deterministic defaults |
| [Hyphenation](HYPHENATION.md) | Provider responsibilities, replacement source coordinates, penalties, caches and selections |
| [First-fit](FIRSTFIT.md) | Greedy checkpoint/carry rules, fit, trimming, discretionary rescue and limits |
| [Knuth–Plass](KNUTH+PLASS.md) | Passes, retirement, emergency search, compaction, parameter profiles and limits |
| [Scoring](KNUTH_PLASS_SCORING.md) | Implemented cost formulas and differences from TeX |
| [Freezing](FREEZE.md) | Materialization, bidi approximation, physical width allocation and fit diagnostics |
| [Frozen result contract](FROZEN-API.md) | Shared layout inputs, result queries, coordinates and immutable-result lifetime |
| [Whitespace](WHITESPACE.md) | Candidate classification, elasticity and line-edge trimming |

## Architecture And Development

- [Pipeline integration design](PIPELINE_INTEGRATION.md): Khipu’s role in the typesetting
  stack, preparation/measurement/layout boundaries, atomic input and ownership.
- [Overall algorithm](KHIPU_ALGORITHM.md): phase-oriented design reference,
  implementation map and explicitly identified limits. Visual representation is
  outside this module; pseudocode is not a client API.
- [Testing](TESTING.md): guarantees and test evidence, deterministic services,
  dependency release checks and separately labeled future work.
- [Performance](PERFORMANCE.md): benchmark workloads, profiling commands and
  recorded measurements.

## Diagram Index

Diagrams are repository-owned assets beside the contracts they explain.
The public lifecycle uses a checked-in [SVG](diagrams/public-lifecycle.svg),
generated from [Graphviz DOT](diagrams/public-lifecycle.dot); its source includes
the regeneration command. The other diagrams use Mermaid in their guides.
The accompanying text and tables also describe each example.

| Question | Diagram |
| --- | --- |
| Which public calls produce which objects? | [Public lifecycle](../README.md#public-lifecycle) |
| What is copied, shared or reusable? | [Ownership and reuse](PREPARED_PARAGRAPHS.md#ownership-and-reuse) |
| When can streaming output be committed or discarded? | [Completion and failure](../contracts/ReadMe.md#stream-completion-and-failure) |
| How do document bytes, source scalars and backend anchors differ? | [Coordinate ladder](../contracts/ReadMe.md#coordinate-ladder-multibyte-text-and-soft-hyphens) |
| Where do knot metrics, glyphs and provenance live? | [Retained storage](KHIPU_ALGORITHM.md#retained-storage-map) |
| What changes between logical and visual lines? | [Materialization and RTL example](FROZEN-API.md#from-logical-material-to-a-visual-line) |
| Which coordinates describe generated replacement text? | [Replacement coordinates](FROZEN-API.md#replacement-coordinates) |
| How does first-fit resume after committing a checkpoint? | [Checkpoint and carry](FIRSTFIT.md#checkpoint-and-carry-transitions) |
| When does Knuth–Plass advance, discover, commit and retire? | [Boundary transaction and lifetimes](KNUTH+PLASS.md#boundary-transaction-and-lifetimes) |
| How are final widths and fit outcomes assigned? | [Allocation and rounding](FREEZE.md#allocation-decisions-and-rounding) |

## History And Unimplemented Designs

The [history index](history/README.md) separates completed implementation records
from active contracts. It includes the [pipeline integration record](history/INTEGRATION_HISTORY.md),
[frozen API delivery notes](history/FROZEN_API_DELIVERY.md),
[scoring history and fitness-class proposal](history/SCORING_DESIGNS.md),
[project background](history/BACKGROUND.md), and [client-readiness review](REVIEW.md).
The integration record retains developer-workspace links and its original
section anchors. Client guides use links within this repository.

The [Knuth–Plass paper](knuthplass.pdf) is background for the algorithm references,
not a claim of full TeX parity.
