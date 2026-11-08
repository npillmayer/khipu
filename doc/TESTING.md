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
their policies differ. Future freeze tests should separately check line
materialization and conservation of assigned width; these are not current
capabilities.

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
scripted fixtures, not another encoding backend. Font-backed proofs, freezing
and downstream bidi/materialization tests follow when those integrations exist.
