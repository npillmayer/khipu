# Testing Khipu Knotting

## Purpose and Status

Reviewed November 8, 2026. The sole encoder is `EncodeParagraph(ctx, para)`.
`Config{}` enables synthetic streaming shaping. Metric-only shaping,
`KnotStyler`, and their isolated tests have been retired; see
[RETIRE-METRICS.md](RETIRE-METRICS.md) for the coverage migration and required
sibling cords fixes. Existing manual knot fixtures remain useful independent
linebreaker tests.

Clients can exercise supported encoding and linebreaking without installed fonts.
Synthetic tests establish layout, ownership and lifecycle contracts, not real
typography. Backend integration needs separate font-backed tests. Helpers and
contracts are implemented; reusable adapter conformance suites, visual proofs
and a shared testing package remain proposals.

## Existing Guarantees and Evidence

- `linebreak/knuthplass/scratch_test.go` checks buffer reuse, growth, smaller and
  empty results, independence across breakers, and nested pass-2 candidate
  processing without overwriting outstanding ordinary evaluations. Horizon
  snapshots stay sorted and independent of live membership changes. Fixed
  fixtures verify zero steady-state allocations for scoring and snapshots
  after warmup with diagnostic logging disabled.
- `linebreak/knuthplass/index_test.go` checks breakpoint-bucket isolation,
  multiple line counts, distinct discretionary variants, line-specific rescan
  seeds, missing buckets, and replacement without altering sibling states.
  Rectangular and varying-shape corpus results were captured before indexing;
  exact breakpoints, quality and predecessor counts remain unchanged. Final
  working-state/seed counts verify that no retirement policy changed.
- `linebreak/knuthplass/logging_test.go` checks that disabled hot-loop debug
  calls and final info dumps never reach the tracer. Enabled dumps remain
  available, and ordinary/discretionary paths, costs and predecessor tables
  remain identical across error, info and debug levels.
- `stream_shaper_test.go` checks the shared paragraph/replacement lifecycle:
  buffered output draining, producer and terminal errors, cancellation, joining
  on consumer failure, and rejection before producer launch. The existing
  paragraph and discretionary integration tests exercise both callers.
- `linebreak/knuthplass/path_test.go` checks canonical equal-cost predecessor
  replacement, synchronized live seeds, and terminal selection by cost then
  line count. `compat_test.go` checks the ParShape/LineShape alias identity;
  `khipu_test.go` checks allocation sizing and slice growth beyond capacity.
- `layout_params_test.go` checks complete independent default profiles, shared
  Params across breaking/freezing, explicit zero-skip overrides, firstfit's fil
  ragged-right behavior, finite shrink, terminal fill precedence and physical
  right skips in RTL paragraphs. Invalid/nil shapes and invalid skip bounds
  fail before querying the shape or changing breaker selections. Configuration
  tests distinguish value snapshots from shared LineShape service lifetime.
- `logical_lines_test.go` covers complete-path validation, source coverage,
  retained-neutral trimming, selected-fragment carry, silent replacements,
  arena lifetime and atomic errors. `logical_lines_integration_test.go` consumes
  ordinary and explicit/automatic discretionary results from both breakers.
  The materializer stays internal; `export_test.go` provides a test-only bridge
  so integration coverage does not require a premature public freeze API.
- `line_ordering_test.go` checks the trailing-source-glue L1 approximation before
  trimming, actual LTR/RTL resets, successive line splits inside an isolate,
  nonzero paragraph offsets, style-split and multibyte glue, empty/final lines,
  discretionary barriers, source gaps, invalid input and repeatable ordering
  without consuming the original resolution. NBSP, internal tabs and omitted
  controls explicitly record accepted approximation limits. These tests do not
  claim full UAX#9 L1 conformance or visual knot permutation.
- `knot_ordering_test.go` checks the partial permutation of original source-backed
  knots: LTR/RTL/mixed segments, nested isolates, source gaps, trimmed segments,
  style-split RTL boxes and unchanged glyph storage. It also checks local index
  semantics, neutral source-backed knots, invalid/ambiguous spans and atomic
  errors. Original zero-source items remain unresolved at this mapping stage;
  generated placement and visual construction are tested separately.
- `knot_ordering_fragments_test.go` checks selected fragment placement by source
  provenance, mixed directions, shaped LTR/RTL consecutive alternatives, silent
  replacements, preserved glyph/context/identity data and atomic rejection of
  ambiguous or zero-source fragments. The provider-backed changed-spelling
  fixture in `knot_ordering_test.go` is also placed, not deferred. No reshaping or
  generated-fill placement is part of this stage.
- `generated_placement_test.go` completes partial mappings with the explicitly
  marked final fill at the paragraph's trailing physical edge. It checks both
  directions, contrary-direction text runs, empty/all-space and wrapped lines,
  discretionary carry, finite custom fill, invalid mappings and unchanged input.
  Paragraph direction is passed explicitly; side skips and public visual-line
  construction are not implemented by these tests.
- `visual_lines_test.go` checks the integrated internal constructor: authoritative
  `ParagraphDirection()`, nested isolates, empty/all-space input, independent knot
  slices, retained arena lifetime, retry safety and no partial result on later-line
  failure. `visual_lines_integration_test.go` asserts visual sequences through
  both breakers, including mixed directions and forced RTL line breaks. Existing
  logical integration cases also run through visual construction for trimming and
  automatic/explicit choices; fragment tests exercise silent and consecutive
  alternatives through the complete pipeline. Width allocation and side skips
  remain out of scope.
- `line_geometry_test.go` covers the subsequent elastic geometry stage: one-based
  target snapshots, exact skip values and physical edge slots, signed delta,
  final-fill accounting, negative finite skips, malformed widths, overflow,
  input independence and later-line failure/retry. `line_geometry_integration_test.go`
  uses both breakers with varying targets and nonzero skips; existing logical
  integration fixtures also verify geometry for empty, trimmed and discretionary
  lines. This geometry stage does not allocate widths or check capacity.
- `width_allocation_test.go` covers the pure allocator's exact conservation,
  proportional quotas, deterministic remainder ties, symbolic fill precedence,
  finite shrink, signed widths, capacity errors and near-limit arithmetic. A
  deterministic randomized suite checks 2,000 feasible finite cases. Kind
  independence includes hypothetical textboxes without implying glyph realization.
  `width_plan_test.go` checks geometry adaptation, side skips, LTR/RTL final fill,
  competing fills, stale geometry and unchanged glyphs. Target-width reshaping
  remains unimplemented; numeric textbox proposals are not glyph realization.
- `frozen_test.go` checks final SOA alignment, exact target conservation, finite
  glue stretch/shrink, side skips/fill, unchanged metadata/glyphs, independent
  output planes, retained arena lifetime, selected changed-spelling and silent
  fragments, empty/trimmed input, unsupported non-glue elasticity, and later-line
  failure/retry without partial publication. `frozen_integration_test.go` covers
  both breakers, varying targets, nonzero skips and LTR/RTL final-fill placement.
  Construction is internal, with only a test bridge for external integration.

- `encoding_stream_test.go` and `encoding_stream_internal_test.go` cover defaults,
  configuration snapshots, repeated calls, fresh factories, retained glyphs,
  cancellation, errors and explicit-zero parameters/fill.
- `encoder_integration_test.go` covers scalar anchors, repeated-anchor width
  accumulation, zero advances, invalid anchors, invalid UTF-8, overflow and
  atomic failure. Useful old metric-collector assertions migrated here.
- `paragraph_regression_test.go` preserves corpus line-width assertions and
  verifies whitespace classification after cord chunk boundaries.
- `penalties_test.go` independently tests shared penalty normalization.
- `run_preparation_test.go`, run-resolution and producer tests cover logical
  bidi/style intersections, original scalar coordinates, parameter ownership
  and producer completion. See the corresponding source/test files.
- Boundary, cluster, candidate and shaping-storage tests isolate each pipeline
  stage. Final paragraph publication occurs only after successful completion.

`breakpoint_positions_test.go` covers original-source endpoint mapping for
encoded paragraphs: multibyte text, nonzero paragraph offsets,
whitespace, empty/final fills, glyphless gaps, pre-marker breaks, selected
changed-spelling alternatives, and invalid or missing provenance. Unknown
discretionary splits fail instead of using replacement lengths.
Both public breaker integration suites now verify mapped explicit/automatic
split positions; ordinary narrow-line contract tests check source coverage too.

`discretionary_eligibility_test.go` covers the metadata-only automatic-streaming
eligibility prerequisite: complete source segments, multibyte/combining/RTL text,
style boundaries at word ends, rejected style/direction/control-gap word fragments,
clusters spanning multiple word segments, explicit-marker exclusion, absent or
invalid metadata, and unchanged source/caches. Numbers and punctuation test the
distinction between structural eligibility and linguistic hyphenatability.
The provider now applies this gate before automatic discovery.

`discretionary_automatic_test.go` covers lazy automatic lookup, retained run
language/script/direction, changed-spelling provenance, unique variant assignment,
malformed response rejection before shaping, batch failure/retry, cached results,
and rejected partial-word requests. Explicit and automatic requests coexist in
one paragraph without sharing fragment identity or discovery routes.
`stream_automatic_linebreak_test.go` exercises selection and cache reuse through
both breakers under their existing lookup policies, plus wide-line no-lookup
behavior and unchanged base knots. Policies must now provide SourceSplit for
automatic alternatives.

`stream_explicit_linebreak_test.go` exercises public streaming encoding through
both breakers with the owner configured as provider: wide lines avoid discovery,
explicit variants can split a first word or extend a preceding line, multiple
markers offer alternative splits, non-fitting variants stay unselected, caches
are reused and selections are cleared on a later run. Base knots remain unchanged.
`discretionary_provider_test.go` verifies retained replacement boxes, batch
atomicity/retry after a later factory failure, feature-policy rejection, cache
reuse and declining marker-edge boxes without automatic fallback.

`stream_nobreak_test.go` covers public streaming U+00AD suppression, unchanged
natural width versus marker-free text, original glyph anchors and marker source
positions, multiple/leading/trailing/marker-only inputs, mixed directions,
ligature-like clusters spanning markers, feature-range remapping, invalid backend
anchors and preservation of style boundaries and pre-marker space/hard breaks.
The encoding stage does not discover candidates; linebreaker provider lookup now does.

`discretionary_explicit_test.go` covers private U+00AD preparation: marker-only
policy requests, complete left/right replacement strings, multiple positions
and material variants, UTF-8 provenance, empty material, owner penalty fallback,
declined requests and invalid/error results. A recording backend verifies that
the common shaper receives `co-`, not an isolated hyphen. End-to-end explicit
linebreaking and no-break suppression are covered separately above.

`discretionary_input_test.go` covers explicit source splits independently of
replacement spelling. `discretionary_stream_test.go` covers standalone fragment
pair shaping: fresh streams, buffered/unbuffered output, local scalar anchors,
original provenance, empty/silent fragments, retained glyph positions, fractional
width accumulation, owned features/glyphs, whole-input feature policy, backend
failures, invalid output and cancellation with producer joining. Failure of the
second fragment returns no pair. This is private-stage coverage, not arena
publication, soft-hyphen discovery or active provider integration.

`discretionary_storage_test.go` separately covers private pair publication into
the arena/cache: original versus replacement coordinates, byte/scalar clusters,
retained metrics and penalties, independent glyph/feature ownership, stable IDs
after arena growth, selection lookup, empty/silent fragments and arena teardown.
Malformed second fragments and duplicate variants leave the existing arena and
cache unchanged. Manually supplied metric-only candidates retain absent box references. Discovery
and active provider integration remain outside this coverage.

`stream_linebreak_contract_test.go` checks both breakers at narrow widths using
streaming-encoded paragraphs: increasing legal knot indices, terminal trailer,
forced breaks, trimmed minimum widths, no discretionary choices and unchanged
base knots. It includes repeated spaces, bidi, NBSP and fixed typographic spaces.
The tests exposed a firstfit carry-checkpoint omission, now covered independently
in the firstfit package. The implemented discretionary bridge is described
in [HYPHENATION.md](HYPHENATION.md).

The knotting correctness audit adds `knotting_regression_test.go` for mandatory
breaks inside clusters, omitted newline glyphs, silent runs, trailing source
gaps, and style changes without new break opportunities. Public breaker tests
also cover CRLF, repeated newlines, mixed direction, NBSP, typographic spaces,
zero-width space and combining marks. Boundary preparation now explicitly
preserves UAX#14 hard-break precedence: additive upstream contributions had
weakened breaks between consecutive hard separators. The CRLF exception is
retained. `ErrUnrepresentableMandatoryBreak` documents rejected inputs
whose required breaks need cluster splitting or glyphless blank-line support.
Real-font/backend adapter integration remains a separate validation gap.


## Dependency Policy

`contracts` owns service interfaces and value types; it imports neither the root
engine nor helpers. `helpers` imports contracts, not the root engine, and requires
no installed fonts. Root encoding composes them. Paragraph-specific `Hyphenator`
and linebreaker provider interfaces remain with their consumers.

Core tests must not require OpenType adapters, external executables or font
discovery. Backend integration belongs in adapter-specific suites with pinned
fonts and explicit expected shaping. Test metric arithmetic independently from
linguistic policy. Do not weaken line-width or source-coverage assertions to
accommodate a broken dependency.

The repository uses sibling `cords` and `uax` replacements. Tests exercise those
checkouts, not necessarily published dependency versions. The bounded-reader and
iterator corrections documented in [RETIRE-METRICS.md](RETIRE-METRICS.md) must
accompany release of the migration.

## Useful Default Implementations

The [helper guide](../helpers/ReadMe.md) documents constructors, units, exact
character policy and lifecycle; [contracts](../contracts/ReadMe.md) documents the
adapter boundary. Executable examples live in root and helper test files.

| Service | Implemented policy | Evidence |
| --- | --- | --- |
| `SyntheticFont` | 1000 units/em, ascent 800, descent -200; immutable signed fractional scaling, positive em size required | `helpers/synthetic_font_test.go` |
| `DefaultStyleResolver` | Nil styles only; validates complete defaults and copies feature storage; does not infer script/language | `helpers/default_style_resolver_test.go` |
| `SyntheticStreamingShaper` | Logical-order scalar glyphs; fresh one-shot stream; 500-unit ordinary advance; no real font shaping | `helpers/synthetic_streaming_shaper_test.go` |

At 10 pt em, ordinary cells measure 5 pt. At 14 pt they measure 7 pt. The
constructor's default spacing is elastic; explicit run defaults may select fixed
spacing. U+0020 uses Params stretch/shrink percentages. Other spaces follow
[WHITESPACE.md](WHITESPACE.md). A supplied `StreamingConfig` is complete, not
merged with synthetic defaults.

Zero-advance controls are U+000A, U+000D, U+000B, U+000C, U+0085, U+2028,
U+2029, U+200B and U+2060. They still emit anchors. All other scalars use a cell,
including NBSP, tabs, combining marks and join controls. Tabs do not expand.
The standalone helper also gives U+00AD a cell; paragraph encoding removes it
before shaping and retains its source position. Shared character expectations
live in `helpers/fixtures_test.go`.

Synthetic glyph IDs are scalar value plus one, not font-outline identifiers.
Only synthetic faces, LTR/RTL direction, empty feature lists, supported flush
modes and zero input-buffer limits are accepted. Invalid scalar reads, cluster
overflow and unsupported options fail explicitly.

### Ownership and Streaming Lifecycle

- Font faces/scales and the default resolver are immutable and shareable.
- The owner is not concurrency-safe. Configuration/default feature storage and
  Params are copied; service objects and factories are retained.
- The factory returns a fresh stream for each paragraph or replacement fragment.
  Completed or failed streams cannot be reused.
- Shape is synchronous and may block on output. Encoding runs the producer in a
  goroutine while the calling path drains glyphs. Output capacity does not
  eliminate the need for concurrent draining.
- Successful Shape leaves output open for more runs. Finish closes after the
  final run. Accepted errors/cancellation close output and retain terminal Err.
- A consumer stopping early cancels and joins the producer. Cancellation need not
  interrupt an already executing source read. Do not publish partial output.
- Concurrent Shape calls and Finish/SetInitialCluster during Shape are rejected.
  Glyphs and Err may be queried concurrently. Do not copy an active stream.

Tests cover buffered/unbuffered backpressure, cancellation, repeated Finish,
reader errors, rejected concurrent operations and terminal-state stability.
Run the race suite as well as ordinary tests when changing this protocol.

## Reusable Test Fixtures and Contract Tests

### Package Placement

Defaults intended for ordinary clients should be available through a small
public API. Reusable scripted fixtures and adapter contract tests could live
in a public `khiputest` package. Helpers in `_test.go` files or `internal`
packages cannot provide the same facility to downstream clients.

Keep `khiputest` out of production imports. If it imports the root package,
root-package tests that use it should be external `package khipu_test` tests
to avoid an import cycle. Keep small private fixtures where they are useful;
do not extract every helper merely for uniformity.

### Scripted Fixtures

Fixed-width defaults make ordinary cases predictable. Scripted fixtures make
hard cases reproducible. The existing `recordedShaper` and
`encodingClusterShaper` are starting points for reusable fixtures covering:

- several metrics or glyphs sharing one cluster
- zero advances and nonzero glyph offsets
- a cluster spanning a potential line-break position
- multibyte source characters and scalars producing no glyph output
- injected failures after some output
- malformed cluster positions for testing consumer validation
- predetermined hyphenation alternatives, with calls recorded to check laziness

Distinguish valid synthetic behavior from deliberately malformed fixture output.
A fixture that injects bad output tests the consumer's defenses; it is not an
example of a conforming adapter.

### Coordinate and Lifecycle Contracts

The existing and proposed shaper APIs use different coordinates:

| API | Cluster coordinate | Example anchors for `aéz` |
| --- | --- | --- |
| `Shaper` | Run-relative UTF-8 byte offset | `0, 1, 3` |
| `StreamingShaper` | Continuing original Unicode scalar ID | `0, 1, 2` with initial base zero |

Test this distinction explicitly, including multiple runs, initial-cluster
overrides, omitted output, and representable-range limits. Streaming cluster
IDs advance by input scalar count, not emitted glyph count. Cluster boundaries
alone do not authorize line breaks.

Reusable streaming contract tests should construct a fresh implementation for
each scenario and verify:

- successful `Shape` calls leave the output channel open
- `Finish` closes output after the final run
- failure or cancellation terminates the stream and exposes the terminal error
- already emitted glyphs remain drainable after failure
- cancellation releases a producer blocked on output backpressure
- rejected lifecycle operations during shaping do not disrupt the active call
- consumers can query `Glyphs` and `Err` concurrently as documented

Use explicit synchronization to establish blocked states instead of timing
assumptions. Cancellation is not required to interrupt an executing source read.
At the owner level, verify that failed shaping does not publish a partial
paragraph. Run concurrency tests with the race detector.

## Test Layers and Assertions

### Deterministic Core Tests

Keep the ordinary suite independent of real fonts and rendering. Build on
existing tests for source preservation, cluster boundaries, forced breaks,
trimming, penalties, discretionary caching and selection, and error recovery.

Extend invariant tests and fuzzing to check:

- aligned lengths of all struct-of-arrays planes
- valid source spans and exact preservation of original bytes
- correct handling of paragraph offsets and the terminal fill/forced break
- width bounds, arithmetic overflow, and legal break positions
- deterministic results for the same input and configuration
- documented errors for malformed or unrepresentable inputs

Exercise multibyte text, combining sequences, bidi controls, empty paragraphs,
and source sections. Assert mappings at the correct stage: run intersection may
leave control-only gaps even though encoded paragraph spans preserve the source.

Firstfit and Knuth-Plass should satisfy shared structural invariants, but they
need not choose the same breaks. Use algorithm-specific expected results where
their policies differ. Freeze tests separately check line materialization and
conservation of assigned width; script-sensitive glyph realization still needs
its own future backend contracts and coverage.

### Real-Backend Integration

Use a small, pinned set of redistributable font fixtures, recording their
versions or checksums and licenses. Avoid reliance on installed system fonts or
network downloads during test execution.

Cover proportional advances, ligatures, mark attachment, and RTL shaping as
supported by the chosen backend. A real monospace font can be one fixture, but
cannot represent all of these behaviors. Check adapter output and its
integration with khipu against independently specified expectations; synthetic
and real shaping are not expected to produce identical metrics.

Reuse the applicable contract tests across synthetic and real implementations.
Keep backend-specific glyph expectations separate from generic lifecycle and
mapping assertions.

## Visual Debugging

Typography benefits from visual inspection, and diagnostic output can also help
clients understand khipu's intermediate representation. Separate structural
diagnostics from actual glyph rendering.

### Structural Diagnostics Without a Font Backend

Start with a CLI that emits SVG and a machine-readable dump. A useful first
diagram would show:

- textboxes at their measured natural widths
- distinct glue regions with stretch/shrink capacities
- source spans, knot indices, and penalties
- forced and selected breakpoints
- selected discretionary alternatives

Source text can appear as labels without determining the geometry. A viewer may
use its ordinary UI font for those labels; the diagram does not require khipu
to load a font or shape glyphs. Represent special fill capacities symbolically
rather than drawing them as enormous finite widths.

Initially, show elastic widths and break decisions. Do not imply that these are
final justified positions or visual bidi order. A standalone CLI artifact is a
smaller first step than a web application and can accompany a failing test.

### Glyph Proofs Later

Actual typography proofs require a real font and shaping/rendering backend,
retained glyph data, and resolved line placement. Loading Courier alone does
not supply these missing stages.

When those stages exist, add proofs using pinned fonts and renderer settings.
Keep numerical and structural assertions authoritative; images explain failures
and reveal visual issues but should not be the only correctness oracle.


## Knuth-Plass Performance Baseline

`linebreak/knuthplass/benchmark_test.go` provides three independent benchmarks:

- `BenchmarkEncodeLorem` measures streaming encoding without linebreaking.
- `BenchmarkBreakLorem` measures the public breaker, with encoding outside the
  timed region. Each call creates fresh breaker state.
- `BenchmarkFirstPassLorem` measures one first pass through collection and
  reports knots, retained working states, historical predecessors, line count,
  demerits and worst badness. State counts are final totals, not peak active
  counts; the terminal forced break has already cleared earlier horizon entries.

The fixed 19-word ASCII corpus repeats to 250, 500 or 1,000 words, with a trailing
space. Synthetic `Config{}` shaping uses 5 pt cells and default space elasticity;
there are `2*words+1` knots. Breaking uses `DefaultLayout` at 150 pt and
`NewKPDefaultParameters`, without a discretionary provider. This isolates the
ordinary path-building cost, not font shaping or hyphenation performance.
Benchmarks require info/debug logging disabled and do not change global logging
configuration. The default no-op tracer meets that requirement.

Run small cases first; active-state growth and candidate allocations still make
larger cases more expensive even after breakpoint indexing:

```sh
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkEncodeLorem$' -benchmem
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkBreakLorem/(250|500)$' -benchtime=1x -count=3 -benchmem -timeout=5m
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkFirstPassLorem/1000$' -benchtime=1x -benchmem -timeout=10m
```

For profiles, retain the matching test executable outside the checkout:

```sh
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkFirstPassLorem/500$' -benchtime=1x -timeout=5m -cpuprofile=/tmp/khipu-kp.cpu -memprofile=/tmp/khipu-kp.mem -o /tmp/khipu-kp.test
go tool pprof -top -cum /tmp/khipu-kp.test /tmp/khipu-kp.cpu
go tool pprof -top -alloc_space /tmp/khipu-kp.test /tmp/khipu-kp.mem
```

If the Go installation omits the `pprof` executable but includes its sources,
build it with `go build -o /tmp/khipu-pprof cmd/pprof` and substitute
`/tmp/khipu-pprof` for `go tool pprof`.

Initial before/after measurements on Apple M1, Go 1.25.6, use a single first-pass
iteration per size, with CPU/memory profiling enabled. They establish the impact
of logging guards, not a statistically stable performance guarantee:

| Words | Before | Guarded | Before B/op | Guarded B/op | Retained states |
| --- | ---: | ---: | ---: | ---: | ---: |
| 250 | 3.72 s | 0.84 s | 2.00 GB | 26.8 MB | 1,881 |
| 500 | 55.67 s | 12.18 s | 29.11 GB | 194.7 MB | 7,058 |
| 1,000 | not run | 188.87 s | not run | 1.535 GB | 27,282 |

Bytes are cumulative allocations in decimal units, not peak resident memory.
Line counts (60/119), demerits (-22,992/-47,493), worst badness (43), and
predecessor counts (1,880/7,057) were unchanged. The unguarded 1,000-word baseline
was deliberately not run. These logging-only measurements precede breakpoint
indexing (below); they do not claim linear scaling.

In the combined 250/500-word baseline CPU profile, `pathsAsString` accounted
for about 46.5% of samples cumulatively; its allocation subtree accounted for
about 95.8% of sampled allocated bytes. After guarding, the combined
250/500/1,000-word CPU profile is dominated by `updatePath` (59.9% cumulative)
and `evaluateCandidates` (31.9% cumulative), chiefly map iteration. The profiles
cover different size sets and include untimed fixture setup, so compare the
per-size benchmark numbers above for speedup, not the profiles' total times.
The 1,000-word result confirms that logging guards alone do not fix scaling.

### Breakpoint Indexing

Working states now use `map[BreakRef]map[lineNo]bookkeeping`. All three hot scan
sites visit only the requested breakpoint's line states. The benchmark sums
bucket sizes for `retained-states`; counting outer map entries would incorrectly
report breakpoints instead. No seeds, predecessor history or retired working
states were removed, and candidate eligibility is unchanged.

The same single-iteration profiled first-pass benchmark on the same machine:

| Words | Guarded flat map | Indexed map | Indexed B/op | Retained states |
| --- | ---: | ---: | ---: | ---: |
| 250 | 0.84 s | 0.028 s | 26.4 MB | 1,881 |
| 500 | 12.18 s | 0.213 s | 193.9 MB | 7,058 |
| 1,000 | 188.87 s | 1.033 s | 1.534 GB | 27,282 |

Line counts, costs, worst badness and predecessor counts match the flat-map
baseline for all three sizes. The improvement is in traversal, not allocation:
candidate slices and a growing active set still generate substantial cumulative
allocations. The short indexed CPU profile is no longer dominated by full-map
scanning; measure allocation reduction or retirement separately before changing
either. These remain synthetic, single-run measurements rather than latency
guarantees for real font-backed paragraphs.

Three unprofiled public-breaker runs (`BenchmarkBreakLorem`, `-benchtime=1x`)
measured 25-36 ms, 156-158 ms and 1.013-1.026 s for the three sizes respectively.

### Scratch-Buffer Reuse

Ordinary evaluations, discretionary evaluations and sorted horizon snapshots
now use three independent, per-breaker scratch slices. Capacity grows only as
needed; each result is borrowed until the next invocation of its producer.
Ordinary results remain live across nested discretionary processing. Snapshot
reuse preserves the boundary between evaluating existing states and adding new
ones. There is no pooling, retirement or scoring-policy change.

First-pass cumulative allocations, using the same profiled benchmark:

| Words | Indexed B/op | With reuse B/op | Allocations/op with reuse |
| --- | ---: | ---: | ---: |
| 250 | 26.4 MB | 1.55 MB | 984 |
| 500 | 193.9 MB | 5.19 MB | 2,790 |
| 1,000 | 1.534 GB | 18.03 MB | 7,445 |

All reported costs, line counts, worst badness, predecessor counts and retained
state counts match the earlier baselines. Three unprofiled public-breaker runs
measured 20-31 ms, 124-130 ms and 0.825-0.831 s respectively. The clearest gain
is allocation reduction; runtime improvements are more modest and smaller-case
timings remain noisy. The first-pass profiled times were 0.032, 0.126 and 0.824 s.

The allocation profile no longer shows evaluation or horizon slices as major
sources. It now includes a large proportion of untimed encoding/setup work;
within the breaker, growing path/seed/predecessor maps remain. Profile totals
must not be confused with B/op, which excludes fixture setup. Active-state
growth and safe retirement remain separate algorithmic work.

## Validation and Next Work

Run from the repository root:

```sh
go test ./... -count=1 -timeout=60s
go test -race ./... -count=1 -timeout=90s
go vet ./...
go build ./...
go mod tidy -diff
git diff --check
```

The helper extraction, streaming integration and metric retirement are complete.
Next testing infrastructure work is an adapter conformance suite and reusable
scripted fixtures, not another encoding backend. Internal bidi/materialization
and glue-only freezing now have synthetic coverage. Font-backed proofs and
target-width shaping tests follow when those integrations exist.
