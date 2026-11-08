# Testing Khipu Knotting

Benchmark history note: the recorded timing, cost and work-count tables below
predate neutral Unicode penalty translation. They remain evidence for the
individual optimization experiments, but selected paths, demerits and discovery
work can change under the new policy. Re-measure before using them as a current
baseline or claiming a new speedup.

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

- `linebreak/knuthplass/parameters_test.go` pins both independent profile values
  and pass counts, checks constructor isolation, and verifies that mutations to
  one profile cannot affect nil-parameter calls. Provider-error tests
  capture logging to verify that pass-2/pass-3 failures are returned unchanged,
  without retry, selection publication or duplicate error-level logging.
- `penalties_test.go` checks Unicode-to-layout translation at both sentinels and
  integer extremes. `boundary_preparation_test.go` checks neutral spaces, ZWSP
  and CJK opportunities alongside mandatory/prohibited boundaries and CRLF
  precedence. `linebreak/knuthplass/penalty_translation_test.go` checks the real
  encoder/breaker roundtrip has no ordinary-break rewards while intentional
  negative penalties still affect ranking. Corpus quality expectations use
  neutral opportunities; the leading-space materialization fixture no longer
  expects a rewarded empty first line.
- `linebreak/knuthplass/evaluation_cache_test.go` checks boundary/penalty scoping,
  scratch independence, zero warm allocations, invalidation on successful writes
  (but not losing proposals), changed seeds/new line counts, forced-boundary
  successors and error cleanup. Cached and uncached runs agree after every knot
  on working tables, seeds, horizon, retirement and predecessor history, as well
  as final paths/quality, across provider modes, variable shapes and forced or
  overlong lines. The matrix includes active emergency stretch; replacement and
  new-successor fixtures include lines accepted only through virtual capacity.
  Pass 1 allocates no evaluation cache.
- `discretionary_cache_test.go` checks successful empty automatic/explicit and
  filtered responses, paragraph/knot isolation, owner validation, manual
  overrides, release, disabled/manual requests, and retry after policy,
  validation or shaping failure. `TestEmptyDiscoveryCachePreservesPassWork`
  checks cold-pass scoring/history counts against pre-cache baselines and warm
  paragraph reuse without further policy calls or changes to the best path.
- `linebreak/knuthplass/geometry_test.go` checks shared ordinary/discretionary
  classification at exact fit, shrink and tolerance boundaries, zero capacities
  and badness saturation, including overfull lines hidden by cubic rounding.
  Lookahead and seed-replay scoring agree, ranking extras stay separate, and
  local classification does not retire working states; boundary commit does.
- `linebreak/knuthplass/emergency_geometry_test.go` checks virtual finite stretch,
  wide arithmetic, unchanged fill/saturation and shrink behavior, nonpositive-E
  equivalence, ordinary/discretionary agreement and cached scoring. It verifies
  that virtual capacity does not enter knots, fragments or seeds, and that
  only explicit pass configuration activates virtual capacity.
- `linebreak/knuthplass/emergency_pass_test.go` checks pass policy/defaults,
  validation before search, disabled/negative E, loose emergency recovery,
  unavoidable fallback, first-success ordering and internal quality context.
  It verifies new pass-3 discovery, provider-error propagation without retries,
  winner-only publication and unchanged real width planes.
- `linebreak/knuthplass/emergency_discovery_test.go` checks positive/empty owner
  cache reuse from pass 2 into pass 3 for automatic and explicit requests,
  including policy and shaping-stream counts. It covers emergency-adjusted
  discovery thresholds, explicit requests independent of automatic lookup,
  warm-run quality/path stability and final selection ownership.
- `linebreak/knuthplass/emergency_stability_test.go` checks equal-cost variant
  ordering over fresh and warm runs, unchanged knot/candidate storage, and
  physical-only post-break seeds during emergency retroactive rescans with and
  without evaluation caching.
- `linebreak/knuthplass/emergency_benchmark_test.go` asserts that its workloads
  require pass 3, distinguish successful recovery from unavoidable fallback,
  and exercise productive or cached empty discovery as intended. Instrumented
  diagnostics match the production coordinator; timed public/isolated calls
  verify results and provider work. See the emergency workload baseline below.
- `linebreak/knuthplass/boundary_test.go` covers zero-capacity lines, rescue at
  terminal/internal forced boundaries, final-pass-only fallback, normal pass
  failure versus provider errors, per-line retirement, rejected resurrection,
  immutable fragment seeds, canonical fallback costs and negative/overflow
  opt-outs. Pruned and unpruned runs match paths/quality on rectangular and
  varying shapes with and without hyphenation, while peak live work decreases.
- `linebreak/knuthplass/hyphenation_benchmark_test.go` checks that the pass-2
  workload discovers and shapes real discretionary candidates, reuses successful
  provider lookups, and creates discretionary predecessor history. An isolated
  overlong origin demonstrates the capped-badness gate and seed-rescan case even when
  splitting its last word cannot rescue the line. No timing assertions are used.
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
  exact breakpoints and quality remain unchanged. Shared geometric screening
  later reduced predecessor counts by rejecting previously admitted overfull
  candidates. Final working-state/seed assertions now verify that forced-boundary
  retirement leaves only terminal work while preserving historical predecessors.
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

`linebreak/knuthplass/benchmark_test.go` provides three ordinary benchmarks:

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
`NewDefaultParameters`, without a discretionary provider. This isolates the
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

### Hyphenating Second Pass

`BenchmarkSecondPassHyphenatingLorem` in `hyphenation_benchmark_test.go` adds
50-, 100- and 250-word cases using the same corpus, synthetic shaping and
150 pt default layout. A deterministic test-only hyphenator supplies one
midpoint split for ASCII words of at least four characters and declines shorter
words. This is a reproducible workload, not a linguistic hyphenation policy.
The real Khipukamayuq validates and shapes its fragments and caches candidates.

This diagnostic explicitly enables pass 2 with the default thresholds. Merely
installing a provider on the public call would not suffice: the ordinary lorem
fixture can finish in pass 1. Timings include lazy discovery, replacement
shaping, graph construction and best-path collection. Paragraph encoding and
breaker setup are excluded. Each iteration gets a fresh owner and paragraph,
so discovery is cold rather than silently becoming a warm-cache benchmark.
This is not the combined latency of the public first/second-pass pipeline.

Additional metrics separate provider requests from actual hyphenator calls,
and report historical discretionary predecessors alongside retained states, line
count, cost and worst badness. At this measurement stage, only nonempty lookups
were cached: short words could cause repeated policy calls. Neither
counter measures rescanned knots; use CPU profiling to attribute that work.
Retained-state counts are final totals, distinct from the subsequently added
`peak-active-states` metric. Custom metrics describe the last iteration rather
than accumulating over `b.N`.

Start small and use explicit iteration counts; the 250-word case is much more
expensive than its ordinary counterpart. Anchor each sub-benchmark expression
so selecting `50` does not also select `250`:

```sh
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkSecondPassHyphenatingLorem$/^(50|100)$' -benchtime=1x -count=3 -benchmem -timeout=3m
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkSecondPassHyphenatingLorem$/^250$' -benchtime=1x -benchmem -timeout=3m -cpuprofile=/tmp/khipu-hyphen.cpu -memprofile=/tmp/khipu-hyphen.mem -o /tmp/khipu-hyphen.test
go tool pprof -top -cum /tmp/khipu-hyphen.test /tmp/khipu-hyphen.cpu
```

Initial single-iteration measurements on Apple M1, Go 1.25.6, with CPU/memory
profiling enabled across all three sizes:

| Words | Time | B/op | Provider requests | Hyphenator calls | Retained states |
| --- | ---: | ---: | ---: | ---: | ---: |
| 50 | 14.2 ms | 2.21 MB | 4,363 | 1,255 | 320 |
| 100 | 202 ms | 15.35 MB | 38,467 | 10,240 | 1,306 |
| 250 | 17.16 s | 239.87 MB | 660,205 | 170,379 | 8,393 |

The combined CPU profile attributes 90.2% cumulatively to
`evaluateDiscretionaryCandidates`, including 85.5% to `appendActualKnots`.
These percentages overlap, not add: replay is inside discretionary evaluation.
This confirms that the workload exposes seed rescanning, unlike the ordinary
benchmark. It does not isolate the fraction specifically caused by dead states.

As above, process-wide profiles include untimed setup. Default-layout synthetic
results are not predictions for a real dictionary, font, or shaping engine.
These are the pre-retirement measurements. The benchmark initially added
coverage only; subsequent boundary-processing changes are recorded below.

### Boundary Processing and Retirement

Ordinary and discretionary candidates share geometric classification. Boundary
processing now advances existing origins before inserting replacements, gives
overfull lines a state-local discretionary rescue opportunity, scores newly
discovered successors through the current boundary, and then retires dead work.
Only the final pass uses deterministic fallback transitions. Negative minima or
unsafe minimum sums opt out of pruning; lazy fragment violations return errors.
See [RETIREMENT.md](RETIREMENT.md) for contracts and remaining limitations.

An initial unprofiled single-iteration run on Apple M1 / Go 1.25.6 measured:

| Workload | Time | B/op | Final working states | Historical predecessors |
| --- | ---: | ---: | ---: | ---: |
| First pass, 250 words | 4.25 ms | 0.82 MB | 14 | 1,679 |
| First pass, 1,000 words | 35.39 ms | 13.19 MB | 53 | 26,448 |
| Hyphenating second pass, 250 words | 53.25 ms | 15.67 MB | 39 | 3,440 discretionary |

The hyphenating run made 29,506 provider requests and 6,216 policy calls.
Three further unprofiled single-iteration runs measured 2.62-4.31 ms for the
250-word first pass, 32.24-32.50 ms for the 1,000-word first pass, and
47.45-48.73 ms for the 250-word hyphenating pass, with identical workload counts.
Costs/line counts/worst badness match the earlier 250-word hyphenating and
250-/1,000-word first-pass baselines. Historical predecessor counts and final
working counts need not match: overfull candidates are no longer accepted and
retired working states are deleted. Tombstones and predecessor history remain.
The benchmark now reports `discretionary-predecessors`, since no discretionary
working bucket survives the terminal forced boundary.

These are initial measurements, not an isolated speedup estimate: they combine
retirement, geometric/pass-policy corrections, state-local rescue evaluation,
and rescan-seed bookkeeping fixes. Earlier profile-enabled timings and current
unprofiled timings also have different instrumentation. The bounded workload
investigation below adds peak/evaluation counters. These measurements precede
the successful-empty caching change recorded later in this document.

### Work Counters and Workload Matrix

Private, optional `workMetrics` instrumentation measures one breaker pass. Public
calls leave it disabled; only nil checks remain on the hot paths. There are no
callbacks, atomics, or recurring table scans. Benchmarks attach the counters
before processing and include the initialized root in the live count.

- `peak-active-states`: maximum live `(BreakRef, line number)` working states,
  including newly inserted successors before retirement. Replacing a predecessor
  does not add a state. Historical predecessors and tombstones do not count.
- `ordinary-evals/op`: ordinary local classifications, accepted or rejected.
  Discovery screening and subsequent boundary scoring used to evaluate unchanged
  states twice; now only cache misses count as actual classifications.
- `ordinary-reused/op`: classifications avoided through boundary-local reuse.
  Added with the subsequent reuse change, not included in `state-evals/op`.
- `discretionary-evals/op`: local classifications of individual variants for an
  origin. Invalid candidates, filtered origins and missing replay seeds do not
  reach classification and do not count. Rejected geometric fits do count.
- `state-evals/op`: sum of those classifications, not unique states visited.
  Provider calls, replayed knots, updates and fallback ranking are separate work.

Counts describe the last deterministic iteration, not totals over `b.N`.
`metrics_test.go` checks insertion/replacement/retirement/resurrection accounting,
accepted and rejected variants, repeated evaluations, per-knot agreement with
actual working tables, and identical results/history with instrumentation off/on.

`BenchmarkRetirementWorkloads` covers 50/250/1,000 words, 100/150/300 pt widths,
alternating widths, and forced internal boundaries. Each uses ordinary breaking,
a provider declining all requests, or the synthetic midpoint hyphenator. Every
case runs one final pass with admission tolerance 200; ordinary mode disables
discovery, while provider modes retain the default discovery trigger of 100.
This controlled diagnostic is not public two-pass latency. Encoding, owner and
breaker setup are untimed and fresh each iteration. Forced cases mark every
twentieth interword space mandatory without adding fill; fallback is permitted.

```sh
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkRetirementWorkloads$' -benchtime=1x -count=3 -benchmem -timeout=120s
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkRetirementWorkloads$/^1000$/^standard$/^productive$' -benchtime=3x -benchmem -cpuprofile=/tmp/khipu-retirement.cpu -memprofile=/tmp/khipu-retirement.mem -o /tmp/khipu-retirement.test
go tool pprof -top -cum /tmp/khipu-retirement.test /tmp/khipu-retirement.cpu
go tool pprof -alloc_space -top -cum /tmp/khipu-retirement.test /tmp/khipu-retirement.mem
```

CPU and allocation profiles are process-wide and include untimed fixture
encoding. B/op and ns/op exclude that setup; do not interpret profile totals as
breaker-only costs. These synthetic ASCII workloads do not measure dictionary
quality, real font shaping, or international-script behavior.

November 8 measurements on Apple M1, Go 1.25.6: all 45 cases completed three
single-iteration runs (135 observations). Custom workload counts were identical
across repeats. Selected results, with time ranges and approximate allocated
bytes per operation (decimal MB, not resident memory):

| Words | Layout | Provider | Time (ms) | MB/op | Peak states | State evaluations |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| 50 | 150 pt | productive | 1.88-1.95 | 0.80 | 74 | 4,523 |
| 250 | 150 pt | productive | 47.36-48.24 | 15.67 | 492 | 144,374 |
| 1,000 | 150 pt | none | 67.02-68.78 | 26.36 | 733 | 284,965 |
| 1,000 | 150 pt | empty | 559.54-564.35 | 275.27 | 733 | 569,930 |
| 1,000 | 150 pt | productive | 1,124.76-1,128.74 | 241.23 | 2,172 | 2,450,514 |
| 1,000 | 100 pt | productive | 2,824.23-2,888.82 | 415.52 | 4,316 | 4,374,191 |
| 1,000 | 300 pt | productive | 404.27-406.51 | 110.08 | 801 | 1,067,405 |
| 1,000 | alternating | productive | 1,374.37-1,384.09 | 258.77 | 2,600 | 2,810,668 |
| 1,000 | internal forced | productive | 347.28-350.05 | 94.97 | 1,074 | 885,322 |

The ordinary row uses tolerance 200 for this comparison. It is not a regression
against the earlier first-pass default of 100. The separate first-pass benchmark
still uses defaults: at 1,000 words its peak is 369 states and its evaluations
are 143,799; the three instrumented runs took 32.85-34.17 ms. Narrow lines and
hyphenation widen the live band considerably; neither the original estimate of
270 states nor terminal retained totals are general bounds.

Findings and bounded follow-up recommendations:

1. **Cache successful empty discovery results, with explicit lifetime rules.**
   At 1,000 words/150 pt the empty provider made 183,440 policy calls and no
   discretionary classifications. The productive provider still made 106,594
   policy calls from 503,783 provider requests; declined short words are retried.
   Source extraction and policy preparation repeat, not just a cheap map lookup.
   A cache must distinguish unqueried from successfully empty, must not hide
   errors, and must respect paragraph/provider configuration ownership.
2. **Consider reusing unchanged ordinary evaluations within a boundary.** Empty
   discovery exactly doubles ordinary classifications here (284,965 to 569,930)
   without changing peak states, predecessor count or selected quality. Rescue
   discovery and final boundary admission currently score separately. Reuse must
   not skip new successors or replacements introduced by productive discovery.
3. **Continue measuring state growth, not just retirement.** Productive narrow
   lines retain 343,701 historical predecessors versus 4,316 peak live states.
   Multiple line counts and variants remain legitimate work; no new dominance
   rule or history deletion is justified by these measurements alone.

Separate CPU/allocation profiles covered the 1,000-word/150 pt cases: ten timed
ordinary iterations and three each for empty/productive, plus Go's calibration
iterations. In the empty profile, `streamingDiscretionaryCandidates` accounts for
25.1% cumulative sampled CPU and 56.0% of allocated bytes; `io.ReadAll` accounts
for 38.4% of bytes, with bounded cord reader/iterator work also visible. These
overlapping percentages support investigating repeated extraction, not charging
all cost to the hyphenator itself. In the productive allocation profile the
provider subtree accounts for 39.7% and `evalNewSegmentSeed` for 24.1%.
Productive CPU samples include substantial runtime scheduling/wakeup activity;
`evaluateDiscretionaryLine` is 10.9% cumulative and `appendActualKnots` 2.4%,
so replay no longer dominates as it did before retirement. The ordinary profile
includes much untimed encoding; within breaking, state insertion/history maps
remain allocation sources. Process-wide percentages are attribution clues, not
isolated speedup estimates or sums to add together.

The bounded synthetic investigation is complete. The next subsections record
negative-result caching and evaluation reuse; real fonts/dictionaries and wider
script coverage remain separate follow-ups.

### Successful Empty Discovery Caching

`Khipukamayuq` now remembers successful empty discovery on each owned encoded
paragraph, without changing the public candidate map or breaker loops. Both
explicit and automatic declines are cached, including filtered alternatives.
Errors remain retryable; disabled discovery and ownerless/manual Khipus are not
negative-cached. See [HYPHENATION.md](HYPHENATION.md#ownership-and-deferred-work)
for the stable-policy lifetime contract and manual candidate precedence.

Before/after measurements used the same 1,000-word/150 pt matrix cases, three
single-iteration runs per case on Apple M1 / Go 1.25.6. Encoding and fixture
setup remain untimed; each run starts with a cold paragraph. Times are medians
with observed ranges, not latency guarantees. MB/op is cumulative allocation.

| Provider | Before ms (range) | After ms (range) | Before/after MB/op | Before/after policy calls |
| --- | ---: | ---: | ---: | ---: |
| Empty | 573.0 (555.9-639.6) | 117.3 (101.5-246.7) | 275.3 / 27.8 | 183,440 / 999 |
| Productive | 1,135.1 (1,118.5-1,203.7) | 869.7 (868.5-875.7) | 241.2 / 98.4 | 106,594 / 999 |

One empty-provider timing was notably higher than the other two; the table
retains that observation. Allocation and policy-call reductions are clearer
evidence than a speedup ratio based on this small sample. Allocations fell from
about 4.26 million to 32,550 per empty-provider run and from 2.51 million to
77,710 per productive run.

Provider requests remain 183,440/503,783, and classifications remain
569,930/2,450,514 (empty/productive). Peak active states, predecessor counts,
line counts, cost and worst badness also remain unchanged. The missing thousandth
policy call simply means that one textbox is never requested by these workloads;
there is no eager pass over all words. This PR removes repeated source/policy
preparation, not repeated scoring. The separate second optimization is recorded
below.

```sh
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkRetirementWorkloads$/^1000$/^standard$/^(empty|productive)$' -benchtime=1x -count=3 -benchmem -timeout=60s
```

### Boundary-Local Ordinary Evaluation Reuse

`boundaryEvaluations` shares local ordinary results between pass-2 discovery and
commit at a single permitted boundary. A separate reusable value buffer avoids
aliasing ordinary/discretionary scratch. Successful state insertion/replacement,
width updates, and retirement invalidate the entire affected breakpoint bucket.
Losing proposals do not. New line counts and changed discretionary seeds are
therefore rescored; predecessor totals and terminal extras are still read/applied
at commit. Normal and error exits end the scope. Pass 1 and forbidden boundaries
do not allocate cache storage. Conservative invalidation can still cause repeat
scoring; this is not a promise of one evaluation per state under all workloads.

The same command above measured three before/after runs at 1,000 words/150 pt,
with successful-empty caching enabled on both sides. Medians and ranges:

| Provider | Before ms (range) | After ms (range) | Ordinary classifications before/after | Reused/op |
| --- | ---: | ---: | ---: | ---: |
| Empty | 94.8 (93.9-95.5) | 88.1 (85.4-88.9) | 569,930 / 284,965 | 284,965 |
| Productive | 872.2 (871.1-877.4) | 844.1 (843.6-849.5) | 1,520,388 / 760,194 | 760,194 |

Ordinary classifications halve in these workloads, but median runtime improves
only about 7%/3%. Discovery, seed replay, updates, insertion/history and map
traversal still cost time. Allocation remains approximately 27.9 MB/op for the
empty case and grows from 98.4 to 98.8 MB/op for productive discovery because of
the boundary cache's reusable storage. These are small-sample synthetic timings,
not a general speedup guarantee.

Discretionary classifications, provider/policy requests, peak working states,
historical predecessors, selected line counts, costs and worst badness remain
unchanged. `state-evals/op` reports actual work only; adding `ordinary-reused/op`
recovers the former evaluation demand in the equivalence fixtures. The private
disable switch exists only for diagnostic uncached comparisons, not as a public
configuration option. Focused tests also check that warm cache storage incurs
no allocations and that newly discovered or replaced successors can still reach
the current forced boundary without stale classification or fallback.

### Emergency Pass Workloads

`BenchmarkEmergencyWorkloads` measures the complete public breaker call and an
isolated emergency pass, with 16/128/512 repeated words. Synthetic shaping uses
5 pt glyph cells. The line target is 17 pt, with zero side skips, E=6 pt and the
default PreTolerance/Tolerance (100/200). Ordinary spaces retain default elasticity.
All fixtures have `2*words+1` knots, including the final retained fill.

| Workload | Input/policy | Required outcome |
| --- | --- | --- |
| `loose-none` | Repeated `ab `, no provider | 10 pt words cannot share a line; E admits loose lines without fallback |
| `loose-declining` | Same text, provider returns no alternatives | Same recovery, with successful-empty caching and new pass-3 policy calls |
| `split-productive` | Repeated `abcd `, midpoint alternatives | 20 pt words split into 15/10 pt shaped fragments; one selected variant per word, no fallback |
| `overfull-declining` | Repeated `abcd `, provider declines | Each 20 pt word exceeds the hard 17 pt minimum-width bound; every word needs fallback |

The normal `TestEmergencyBenchmarkWorkloads` requires passes 1/2 to fail and
pass 3 to complete, and checks the expected fallback/discovery behavior. It
also compares the instrumented diagnostic with the uninstrumented production
coordinator. No elapsed-time assertions are used.

Timing and counter scopes:

- **Public:** `BreakParagraph`, including validation, all three fresh breakers,
  lazy fragment shaping, collection and selected-discretionary publication.
- **Isolated:** `breakParagraphPassRefs` for pass 3, including breaker setup and
  collection but excluding publication. The actual first two passes execute
  outside timing on that iteration's paragraph, retaining their discovery caches.
- **Both:** owner construction and paragraph encoding are fresh and untimed in
  every iteration. Encoding is not included in `ns/op`, `B/op` or `allocs/op`;
  lazy shaping performed during the timed scope is included. Neither scope freezes.
- **Counters:** collected in a separate untimed diagnostic, not by instrumenting
  the timed calls. Every timed result and provider/policy call delta is checked
  against it. Public counters sum work across passes, with maximum (not summed)
  `peak-active-states`; isolated counters exclude prelude work. `passes/op` is
  3 versus 1, while `selected-pass` is 3 in both. Public output also reports
  `p1-state-evals/op`, `p2-state-evals/op` and `p3-state-evals/op`. Verbose fixture
  tests show all per-pass counters. These counts are per operation, not totals
  over `b.N`. The lightweight provider-request counter is active in timed calls.

```sh
go test ./linebreak/knuthplass -run '^TestEmergencyBenchmarkWorkloads$' -v -count=1
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkEmergencyWorkloads$' -benchtime=3x -count=3 -benchmem -timeout=120s
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkEmergencyWorkloads$/^512$/^split-productive$' -benchtime=20x -count=5 -benchmem -timeout=120s
```

Initial baseline: November 8, 2026, Go 1.25.6, darwin/arm64, Apple M1. Medians
of three unprofiled runs of three timed iterations each, info/debug logging
disabled. Small samples, especially sub-millisecond cases, are noisy; these are
diagnostic baselines, not a throughput guarantee or a before/after speedup claim.

| Words | Workload | Public ms | Isolated pass-3 ms |
| --- | --- | ---: | ---: |
| 16 | loose-none | 0.064 | 0.046 |
| 16 | loose-declining | 0.082 | 0.065 |
| 16 | split-productive | 0.256 | 0.215 |
| 16 | overfull-declining | 0.051 | 0.048 |
| 128 | loose-none | 0.220 | 0.200 |
| 128 | loose-declining | 0.401 | 0.389 |
| 128 | split-productive | 1.844 | 1.760 |
| 128 | overfull-declining | 0.399 | 0.365 |
| 512 | loose-none | 0.856 | 0.810 |
| 512 | loose-declining | 1.664 | 1.654 |
| 512 | split-productive | 7.731 | 7.361 |
| 512 | overfull-declining | 1.686 | 1.600 |

For the 512-word public cases, ranges were 0.833-1.444, 1.647-1.712,
7.328-7.947 and 1.683-1.702 ms respectively. Differences between independently
timed public and isolated cases should not be treated as precise prelude costs.

512-word work and allocation baseline (P = public; E = isolated emergency pass):

| Workload | State evaluations P/E | Peak P/E | Provider requests P/E | Policy calls P/E | Bytes/op P/E | Allocs/op P/E |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| loose-none | 1027/1023 | 3/3 | 0/0 | 0/0 | 637472/626896 | 1134/1094 |
| loose-declining | 1027/1023 | 3/3 | 1024/1022 | 511/510 | 1364992/1352888 | 12771/12708 |
| split-productive | 4095/4092 | 4/4 | 1535/1534 | 512/511 | 5906810/5618122 | 36491/36364 |
| overfull-declining | 514/512 | 2/2 | 513/512 | 512/511 | 1367288/1355189 | 12880/12817 |

The loose cases perform 1027 ordinary classifications and reuse 1025; the
productive case performs 1537 ordinary plus 2558 discretionary classifications
and reuses 1536 ordinary results. The overfull case performs 514 ordinary
classifications and reuses 513. Actual state-evaluation totals exclude reuse.
Productive recovery yields 1024 lines and 512 chosen variants; other cases yield
512 lines. Only the overfull workload uses fallback (512 selected transitions).
Recovered paths have worst badness 158 under virtual stretch, versus 10000 for
the overfull path. These are optimization scores, not physical freezing quality.

Passes 1/2 exhaust after two ordinary evaluations each for loose inputs, and
one each for overfull inputs (plus one rejected discretionary in the productive
pass 2). Pass 3 therefore dominates this baseline. The peak remains 2-4 and
evaluation work scales linearly for these fixtures; neither observation establishes
general K&P complexity. Late pass-2 exhaustion, broad active sets, large variant
fan-out and real font shaping require separate workloads. Freezing insufficient
real capacities remains outside this measurement and implementation.

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
