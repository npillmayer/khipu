# Testing

How khipu is tested: which guarantees the test suites establish, the dependency
policy for tests, the deterministic default services, and planned test
infrastructure. Benchmarks and measurements are described in
[PERFORMANCE.md](PERFORMANCE.md).

## Purpose and Scope

Clients can exercise encoding, line breaking and the internal later stages
without installed fonts: `Config{}` enables synthetic streaming shaping. The
synthetic tests establish layout, ownership and lifecycle contracts, not real
typography. Hand-built knot fixtures remain useful as independent line-breaker
tests. Backend integration needs separate font-backed tests. Reusable adapter
conformance suites, visual proofs and a shared testing package are proposals
(see [Reusable Test Fixtures and Contract Tests](#reusable-test-fixtures-and-contract-tests)).

## Existing Guarantees and Evidence

- `linebreak/firstfit/cleanup_test.go` checks disabled debug/info guards,
  unchanged results across logging levels, forced-textbox exclusion from rescue,
  and paragraph-level kern flag/interior/carry semantics. Benchmark workload
  tests assert cold discovery/selection counts and warm reuse without policy calls.
- `firstfit_fitting_integration_test.go` checks ordinary checkpoints and carry
  through freezing: positive slack, exact natural/minimum widths, finite shrink,
  one-DU overflow and fixed-space input. It verifies breakpoints, unchanged text
  widths, space adjustment, fil/fill allocation, exact line totals and unchanged
  input width planes. `linebreak/firstfit/fitting_test.go` pins discretionary fit
  at the same shrink boundary, including fixed glue and one-DU excess.
- `linebreak/firstfit/discretionary_policy_test.go` checks prohibited variants at
  and above the sentinel, mixed/all-prohibited lists, exact and zero gain
  thresholds, width-first ranking, finite-penalty and candidate-order ties, and
  unchanged candidate storage. Public tests cover provider/cache lookup and both
  overflow-rescue branches, including publication of only eligible selections.
- `linebreak/firstfit/ownership_test.go` covers early/late provider errors in both
  rescue branches, unchanged nil/populated selections during callbacks and after
  failure, cache retention and retry, successful stale-selection clearing,
  validation errors, independent defaults and nil/explicit-default equivalence.
- `linebreak/knuthplass/uniform_test.go` compares automatic ordinary
  state compaction with baseline paths, quality and completion across shapes,
  passes, providers, forced boundaries, rewards, emergency recovery and fallback.
  Unknown shapes stay unoptimized. `uniform_public_test.go` compares the real
  public API against an identical-width opaque shape across all passes, cold/warm
  discovery and selection publication. `BenchmarkUniformPublic` times complete
  calls with fresh owners and candidate caches, excluding paragraph encoding.
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
  Rectangular and varying-shape corpus results pin exact breakpoints and
  quality. Final working-state/seed assertions verify that forced-boundary
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
  atomic failure.
- `paragraph_regression_test.go` preserves corpus line-width assertions and
  verifies whitespace classification after cord chunk boundaries.
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
Both public breaker integration suites verify mapped explicit/automatic
split positions; ordinary narrow-line contract tests check source coverage too.

`discretionary_eligibility_test.go` covers the metadata-only automatic-streaming
eligibility prerequisite: complete source segments, multibyte/combining/RTL text,
style boundaries at word ends, rejected style/direction/control-gap word fragments,
clusters spanning multiple word segments, explicit-marker exclusion, absent or
invalid metadata, and unchanged source/caches. Numbers and punctuation test the
distinction between structural eligibility and linguistic hyphenatability.
The provider applies this gate before automatic discovery.

`discretionary_automatic_test.go` covers lazy automatic lookup, retained run
language/script/direction, changed-spelling provenance, unique variant assignment,
malformed response rejection before shaping, batch failure/retry, cached results,
and rejected partial-word requests. Explicit and automatic requests coexist in
one paragraph without sharing fragment identity or discovery routes.
`stream_automatic_linebreak_test.go` exercises selection and cache reuse through
both breakers under their existing lookup policies, plus wide-line no-lookup
behavior and unchanged base knots. Policies must provide SourceSplit for
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
The encoding stage does not discover candidates; that happens lazily during line breaking.

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
The discretionary bridge is described in [HYPHENATION.md](HYPHENATION.md).

`knotting_regression_test.go` covers mandatory
breaks inside clusters, omitted newline glyphs, silent runs, trailing source
gaps, and style changes without new break opportunities. Public breaker tests
also cover CRLF, repeated newlines, mixed direction, NBSP, typographic spaces,
zero-width space and combining marks. Boundary preparation preserves UAX#14
hard-break precedence between consecutive hard separators, with CR LF counting
as one separator. `ErrUnrepresentableMandatoryBreak` documents rejected inputs
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

### Dependency Release Checks

The repository uses sibling `cords` and `uax` replacements. Tests exercise those
checkouts, not necessarily published dependency versions. Encoding regressions
depend on corrections in `cords`: `cordext.ByteRangeBounded` read empty spans
after the first chunk, `BoundedReader.Read` under-reported bytes, and btree
iteration ignored early stop. These fixes must be available in the released
dependency used with khipu; they are not part of a khipu-only commit.

Before release, verify compatible published dependency versions without local
replacements. Local test success does not establish that compatibility. Real-font
adapters and visual proofs also require separate integration checks; synthetic
materialization/freezing tests do not establish font-backed typography quality.

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

`StreamingShaper` cluster IDs are continuing original Unicode scalar indices,
not UTF-8 byte offsets: for `aéz` with initial base zero, the anchors are
`0, 1, 2` (byte offsets would be `0, 1, 3`). Test this explicitly, including
multiple runs, initial-cluster overrides, omitted output, and
representable-range limits. Streaming cluster
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


## Performance

Benchmarks, their workloads, reproduction commands and current measurements
are described in [PERFORMANCE.md](PERFORMANCE.md). Benchmark workloads are
also checked by ordinary tests (for example `TestEmergencyBenchmarkWorkloads`
and `TestFirstfitBenchmarkWorkloads`), which pin work counts without timing
assertions.

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

Next testing infrastructure work is an adapter conformance suite and reusable
scripted fixtures. Font-backed proofs and target-width shaping tests follow when
those integrations exist.
