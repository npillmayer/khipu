# Testing Khipu

This guide describes the implemented pipeline and its verification as of
November 15, 2026. Proposed testing work is listed separately below.

## Run The Suite

From the module root:

```sh
go test ./... -count=1 -timeout=30s
go vet ./...
go build ./...
```

Use `go test -race ./... -count=1` when changing concurrency or ownership.
Benchmark workloads and profiling commands are in [PERFORMANCE.md](PERFORMANCE.md).
Public `Example...` functions are compiled and executed by the normal test suite.

## Current Pipeline And Defaults

`Config{}` selects a fresh synthetic streaming shaper for each encoding or
replacement request, a synthetic font at 10 pt, a resolver for unstyled text,
and default parameters. Ordinary synthetic cells advance 5 pt. No font files
are needed, and the synthetic backend does not provide real typography.

The supported workflows are:

- Text: `EncodeParagraph` → either `BreakParagraph` → `Freeze`.
- Mixed text/atomics: `PrepareParagraph` → optional `MeasureIntrinsic` →
  `BindParagraph` → either `BreakParagraph` → `Freeze`. Reuse a preparation with
  different atomic advances without reshaping base text. The
  [prepared-paragraph guide](PREPARED_PARAGRAPHS.md) specifies this workflow.

Pass the same resolved layout to breaking and freezing. Break indices and the
input's selected discretionaries describe one decision together; freeze before
another successful breaker call replaces those selections. Source, styles, and
font services must remain stable, and operations on an owner must be serialized.

Streaming shaping, style resolution, and frozen-result queries are integrated.
The metric-only `Shaper`, `FixedWidthShaper`, `KnotStyler`, and root compatibility
aliases have been removed. Import services from `contracts` and defaults from
`helpers`. See [service contracts](../contracts/ReadMe.md),
[helper usage](../helpers/ReadMe.md), and the
[frozen result contract](FROZEN-API.md).

## Guarantees And Evidence

| Area | Current guarantee | Representative evidence |
| --- | --- | --- |
| Configuration | Configuration values/features are copied; collaborators are retained. Nil selects documented defaults; supplied zero fields remain explicit. | [config tests](../config_test.go), [snapshot tests](../config_internal_test.go) |
| Streaming encoding | Original UTF-8 coordinates survive shaping. Errors return no partial paragraph; a final non-discardable fill carries the forced end break. | [encoder integration](../encoder_integration_test.go), [streaming tests](../encoding_stream_test.go) |
| Run preparation | Original style spans resolve independently; feature ranges are clipped/rebased for bidi intersections. Atomics bypass shaping and style resolution. | [run preparation](../run_preparation_test.go), [atomic integration](../atomic_integration_test.go) |
| Source/cluster mapping | Glyph anchors are original scalar IDs in logical order; cluster boundaries alone do not authorize line breaks. | [cluster tests](../glyph_clusters_test.go), [boundary tests](../boundary_preparation_test.go), [source endpoints](../breakpoint_positions_test.go) |
| Stream lifecycle | Consumer failures cancel/join the producer; channel closure and producer completion are checked separately. | [stream coordinator tests](../stream_shaper_test.go), [producer tests](../run_producer_test.go) |
| Discretionaries | Discovery and replacement publication are lazy and transactional; successful empty responses can be cached. Explicit markers require a configured material policy. | [provider tests](../discretionary_provider_test.go), [cache tests](../discretionary_cache_test.go), [explicit tests](../discretionary_explicit_test.go) |
| Prepared input | Anchors and supplied advances are validated; bindings own working planes and selections. | [input tests](../paragraph_input_test.go), [pipeline integration](../pipeline_integration_test.go) |
| Continuation | Certified preparation-bound paths retain FF/KP prefix boundaries; suffix search makes zero prefix visits or preparation-service calls. | [checkpoint guards](../checkpoint_test.go), [continuation regressions](../linebreak/firstfit/continue_test.go) |
| Intrinsic measurement | Ordinary and bounded explicit modes work without binding. Automatic intrinsic hyphenation and multiple eligible explicit splits/variants per textbox are unsupported. | [intrinsic tests](../intrinsic_test.go), [explicit measurement](../intrinsic_explicit_test.go), [preparation cache](../discretionary_prepared_test.go) |
| Breaking | Firstfit and Knuth–Plass have separate policies; successful calls publish choices, failures preserve earlier selections. | [firstfit ownership](../linebreak/firstfit/ownership_test.go), [Knuth–Plass passes](../linebreak/knuthplass/pass_test.go), [public examples](../example_test.go) |
| Freezing | Logical materialization, visual knot order, glue allocation and opaque queries are implemented. Fit shortfalls are outcomes; unsupported adjustment remains an error. | [public freeze API](../freeze_api_test.go), [width outcomes](../width_outcomes_test.go), [result queries](../frozen_result_test.go) |
| Cancellation | Encoding/preparation/freezing return no partial result on failure; synchronous collaborator calls have documented cancellation limits. | [freeze cancellation](../freeze_cancellation_test.go), [stream coordination](../stream_shaper_test.go) |

This is an evidence map, not a claim of exhaustive Unicode conformance or
coverage of every external font backend. Bidi L1 uses the documented
[approximation](FREEZE.md#accepted-l1-approximation). Lazy breaker/provider calls
have no context argument; cancellation supplied to encoding or freezing does
not cancel an ordinary `BreakParagraph` call. `firstfit.ContinueParagraph` has
its own context and checks cancellation during its NoHyphenation suffix search.

## Deterministic Services And Fixtures

The three public defaults are `SyntheticFont`, `DefaultStyleResolver`, and
`SyntheticStreamingShaper`. All have usable zero values. Font metrics are 1000
units per em, ascent 800, descent -200, and line gap zero. Scales preserve signed
fractional DU until the consumer rounds. The default resolver rejects nonnil
application styles. The synthetic shaper emits logical-order scalar cells,
rejects nonempty features and nonzero input-buffer tuning, and does not model
ligatures, font fallback, or normalization. Exact behavior and examples belong
in the [helper guide](../helpers/ReadMe.md).

A stream's output capacity is independent of input buffer options. Drain glyphs
concurrently with synchronous `Shape`, then check producer completion and terminal
`Err`. Constructors launch no goroutines. Never reuse a completed/failed stream
or copy a mutable shaper after first use. Configuration factories must return
fresh instances, including for discretionary reshaping.

Private scripted fixtures exercise cases that synthetic one-scalar cells cannot:
multiple glyphs per cluster, omitted scalars, styled/bidi intersections, changed
replacement spelling, silent fragments, invalid anchors, and backend errors.
See [encoding fixtures](../encoding_fixtures_test.go),
[service integration](../service_integration_test.go), and
[discretionary streaming tests](../discretionary_stream_test.go). Keep reusable
whole-paragraph test helpers outside `helpers` if they would require an import
of the root engine.

## Dependency Policy

The root engine imports `contracts` for services and `helpers` for defaults.
`helpers` imports `contracts`, never the root engine. Service definitions import
neither the engine nor implementations. `contracts.ErrIllegalArguments` and
`khipu.ErrIllegalArguments` share sentinel identity; match wrapped errors with
`errors.Is`. Shaper lifecycle errors belong to `helpers`.

Ordinary core packages use `cords`, `uax`, `schuko`, standard-library and extended
Go modules. Font and shaping implementations belong behind the service
interfaces. Concrete backend types should not cross those boundaries.

Adapter integration tests may use concrete font implementations to check unit
conversion, source mappings, cluster semantics, errors, and ownership. Put such
tests with adapters or in a separate integration module if module-level dependency
isolation is required; build tags alone do not establish that boundary.

### Dependency Release Checks

The module declares Go 1.25.6 and replaces `cords` and `uax` with sibling
checkouts. Running the suite here validates that local dependency combination.
It does not prove that the versions in `require` expose the same APIs.

Before release, use an isolated checkout to select the intended dependency
versions, remove local replacements there, and run the normal tests, vet, build,
and `go mod tidy -diff`. Record the tested versions and resolve any dependency
API gaps before publishing. Do not silently remove replacements from a working
checkout merely to make a release check appear successful.

## Visual Diagnostics

`khipu-cli view` and `debug.WriteFrozenSVG` already produce deterministic SVG
schematics of frozen widths, glue, skips, fill, source ranges, and fit outcomes.
They use public frozen queries and do not render glyph outlines. The CLI uses
synthetic shaping and no hyphenator, so it does not select explicit or automatic
hyphenation alternatives. See the [CLI guide](../khipu-cli/README.md),
[writer](../debug/svg.go), and [SVG tests](../debug/svg_test.go).

Treat geometry/source assertions as the correctness oracle; SVGs help explain
failures. Actual glyph proofs need an external font/shaping/rendering backend
with pinned fixtures. Frozen glyph data is available via `CopyText`, but renderer
boxes and drawing remain outside this module.

## Future Testing Work

- Extract reusable adapter conformance tests from the existing scripted cases,
  with constructors supplied by each adapter. Such a public suite is not yet
  provided by this module.
- Extend invariant tests and fuzzing around source mappings, width arithmetic,
  cluster boundaries, cancellation, and transactional publication. Preserve
  minimized regressions and each breaker's distinct policy expectations.
- Add real-backend integration and glyph proofs in the appropriate adapter or
  integration module, keeping deterministic core tests independently runnable.
