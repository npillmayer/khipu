# Testing Khipu Knotting

## Purpose and Status

`breakpoint_positions_test.go` covers original-source endpoint mapping for
streaming and metric paragraphs: multibyte text, nonzero paragraph offsets,
whitespace, empty/final fills, glyphless gaps, pre-marker breaks, selected
changed-spelling alternatives, and invalid or missing provenance. Unknown
legacy discretionary splits fail instead of using replacement lengths.
Both public breaker integration suites now verify mapped explicit/automatic
split positions; ordinary narrow-line contract tests check source coverage too.

`discretionary_eligibility_test.go` covers the metadata-only automatic-streaming
eligibility prerequisite: complete source segments, multibyte/combining/RTL text,
style boundaries at word ends, rejected style/direction/control-gap word fragments,
clusters spanning multiple word segments, explicit-marker exclusion, legacy or
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
streaming automatic alternatives; legacy metric-only examples may omit it.

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
cache unchanged. Metric-only candidates retain absent box references. Discovery
and active provider integration remain outside this coverage.

`stream_linebreak_contract_test.go` checks both breakers at narrow widths using
streaming-encoded paragraphs: increasing legal knot indices, terminal trailer,
forced breaks, trimmed minimum widths, no discretionary choices and unchanged
base knots. It includes repeated spaces, bidi, NBSP and fixed typographic spaces.
The tests exposed a firstfit carry-checkpoint omission, now covered independently
in the firstfit package. The proposed streaming discretionary bridge is recorded
in [HYPHENATION.md](HYPHENATION.md); it remains guarded pending implementation.

The knotting correctness audit adds `knotting_regression_test.go` for mandatory
breaks inside clusters, omitted newline glyphs, silent runs, trailing source
gaps, and style changes without new break opportunities. Public breaker tests
also cover CRLF, repeated newlines, mixed direction, NBSP, typographic spaces,
zero-width space and combining marks. Boundary preparation now explicitly
preserves UAX#14 hard-break precedence: additive upstream contributions had
weakened breaks between consecutive hard separators. The CRLF exception is
retained; this intentional streaming correction is not applied to the legacy
metric encoder. `ErrUnrepresentableMandatoryBreak` documents rejected inputs
whose required breaks need cluster splitting or glyphless blank-line support.
Real-font/backend adapter integration remains a separate validation gap.

**Integration update:** `Config.Streaming` and `EncodeParagraphStreaming` now
provide the opt-in public pipeline through retained textbox glyph storage.
`encoding_stream_test.go` exercises configuration snapshots, fresh factories,
factory errors, cancellation, both linebreakers without hyphenation, and the
explicit guard against metric-only lazy discretionary shaping. Internal tests
verify published arena associations and owned default feature storage. Earlier
draft/integration labels in this document describe the stages before this public
connection; the legacy metric encoder remains available unchanged.

Clients should be able to exercise the complete supported encoding and
linebreaking path without installing a typography backend or implementing
dependencies they do not yet understand. Adapter authors should be able to
verify that their implementations satisfy the same contracts.

This document records the testing strategy and reusable default implementations.
`SyntheticFont`, `FixedWidthShaper`, `DefaultStyleResolver`, and
`SyntheticStreamingShaper` are available APIs. Proposed packages and future
integration work are identified separately.

The implementations live in `github.com/npillmayer/khipu/helpers`; their service
interfaces and associated value types live in `github.com/npillmayer/khipu/contracts`.
Unqualified helper and service names below refer to those packages. The root
retains compatibility aliases, but examples use the canonical package paths.

Policy review: November 5, 2026. **Implemented** describes behavior available in
the current code. **Draft contract** describes an existing interface that is
not wired into the encoder. **Proposed policy** records a concrete choice for
the next implementation; it is not a claim that the behavior is available or
tested. These policies apply to the simple defaults, not to every real backend.

The current implementation already provides part of this foundation:

- `Config{}` selects a fresh `FixedWidthShaper`, default parameters, and
  disabled lazy hyphenation.
- Public examples and integration tests exercise encoding and both linebreakers
  without loading a font.
- Tests use scripted cluster metrics to exercise boundaries and failures.
- `NewSyntheticFont()` and the zero value of `SyntheticFont` provide fixed font
  metrics and immutable scaling without loading a font or requiring a backend.
- `DefaultStyleResolver` and `SyntheticStreamingShaper` implement the default
  policies below, with focused tests and a combined external-package example.
- `StreamingShaper`, `StyleResolver`, and the font interfaces are draft
  boundaries. The current encoder does not use them.
- Freezing, final glyph placement, and rendering remain future work; see
  [Freezing](FREEZE.md) and the [documentation guide](README.md).

The strategy distinguishes three needs: convenient defaults for clients,
demanding fixtures for correctness tests, and integration with real backends.
No single implementation needs to serve all three.

## Existing Guarantees and Their Evidence

| Status | Guarantee or boundary | Source and verification |
| --- | --- | --- |
| Implemented | `Config{}` creates a fresh metric shaper per owner. Supplied shapers and hyphenators are retained; parameters and paragraph-fill values are copied. Explicit zero configuration values are honored. | [Config](../config.go), [owner tests](../khipukamayuq_test.go), [parameter snapshot test](../config_internal_test.go), [public configuration tests](../config_test.go) |
| Implemented | Metric clusters use nondecreasing run-relative byte offsets. Repeated offsets accumulate widths; zero-width metrics still identify starts. EOF carries no metric. | [Shaper](../contracts/shaping.go), [metric validation and tests](../encoding_metrics_test.go) |
| Implemented | Encoding preserves original UTF-8 source spans, rejects invalid input and metric overflow, and returns no partial paragraph on error. Every paragraph has a non-discardable final fill/forced-break knot. | [Encoder](../encoding.go), [integration tests](../encoder_integration_test.go), [migration contract](MIGRATION.md#soa-encoder-to-breaker-contract) |
| Implemented | Both breakers consume encoded paragraphs and return line-ending knot indices. Their policies may select different breaks; neither freezes or renders lines. | [Public examples](../example_test.go), [paragraph regressions](../paragraph_regression_test.go) |
| Implemented | `SyntheticFont` supplies fixed metrics and immutable scales. Its zero value works; invalid em sizes fail, and signed fractional conversions are retained without clamping to `dimen.DU`. | [Implementation](../helpers/synthetic_font.go), [tests and example](../helpers/synthetic_font_test.go) |
| Implemented helper / draft integration | `DefaultStyleResolver` validates and copies complete defaults for nil style, rejects non-nil styles, and isolates feature storage. Font scaling follows resolution; bidi analysis is separate. | [Style interface](../contracts/style_resolver.go), [resolver tests](../helpers/default_style_resolver_test.go); no active encoder integration yet |
| Implemented helper / draft integration | Synthetic streaming glyphs use scalar IDs and signed font-unit positions, with producer/consumer lifecycle rules. | [Streaming interface](../contracts/shaping_stream.go), [synthetic tests and example](../helpers/synthetic_streaming_shaper_test.go); a reusable adapter conformance suite remains step 3 |
| Implemented helper / future pipeline | Run intersection keeps byte and scalar coordinates, including gaps for omitted bidi controls. Prepared-run records describe future metadata publication. | [Intersection tests](../run_intersection_test.go), [prepared-run declarations](../prepared_run.go); neither establishes a working streaming encoder |

The table records evidence, not exhaustive coverage. The
[metric helper tests](../helpers/fixed_width_shaper_test.go) and streaming tests cover
every zero-width exception below, plus representative controls that remain cells.

The internal run-parameter preparation stage now resolves contributing style
spans once, obtains scales, and produces independently owned feature ranges for
their bidi intersections. `run_preparation_test.go` uses the synthetic font and
default resolver alongside private failure fixtures. Feature coordinates in
resolver input/output are original style-span scalar positions; preparation
clips and rebases them, preserving control gaps. This is preparation-only
coverage, not a streaming encoder integration guarantee.

`run_producer_test.go` now exercises `producePreparedRuns` in a producer
goroutine with a collecting consumer on the calling execution path. Coverage
includes bounded cord readers, original scalar cluster bases across omitted
controls, buffered and unbuffered output, cluster-ID limits, cancellation, and
failures before Shape and during Shape/Finish. The test consumer observes both
producer completion and glyph-channel closure: a pre-Shape error need not close
output, and successful completion does not mean buffered output is drained.
This stage does not yet construct knots or change the active encoder.

`boundary_preparation_test.go` covers reader-based paragraph-wide UAX#14
preparation: original byte/scalar endpoints, control gaps, style independence,
UTF-8 sequences split across reads, malformed input, I/O failures, and EOF
finalization. Migration comparisons with the existing metric boundary function
cover spaces, NBSP, mandatory breaks, CRLF, combining characters, NUL, and bidi
controls. This preserves the existing policy rather than asserting new Unicode
conformance. The new finalized source-boundary table is not yet consumed by the
active encoder or a streaming knot builder.

`candidate_metrics_test.go` covers source-based whitespace classification,
parameter defaults and snapshots, fixed versus elastic U+0020, fixed NBSP and
typographic spaces, mixed whitespace, fractional accumulation and rounding,
negative advances, invalid conversions, and reserved dimension limits. These
are independently tested primitives used by candidate accumulation; encoder
integration remains future work. See [Whitespace](WHITESPACE.md) for the policy.

Candidate accumulation is now tested separately in `candidate_knots_test.go`,
using both scripted glyphs and output from the synthetic streaming producer.
Coverage includes structural boundaries with prohibited line breaks, retained
clusters, omitted U+200B, silent-run boundary provenance, fixed/elastic space
classes, paragraph offsets, owned glyph storage, width errors and terminal fill.
Unsupported glyphless mandatory-break sequences fail without partial results.
These intermediate candidates are not yet persistent `Khipu` nodes.

`stream_candidates_test.go` exercises the connected internal pipeline from a
styled paragraph to candidate knots. Synthetic cases cover buffered/unbuffered
output, mixed direction, omitted controls, empty input and retained metadata.
Scripted backends exercise pre-Shape errors with open output, Shape/Finish
failures, terminal Err, invalid glyph anchors and cancellation. Consumer failure
tests verify that the producer exits before the operation returns. Preparation
and finalization failures return no partial candidate paragraph. This is not
yet public encoder integration; source and prepared metadata remain retained
in the internal result rather than installed in `Khipu`.

`shaping_storage_test.go` covers the separate internal candidate-publication
step: textbox-only glyph retention, empty references for glue/fill, shared run
contexts, independent glyph/feature storage, source gaps, mixed text/space
clusters, and association-plane cleanup. The legacy metric encoder still
produces no glyph storage. Discretionary shaping storage is not migrated yet.

Boundary preparation also retains independent default UAX#29 word-boundary
hints. Explicit tests cover punctuation, contractions, numeric punctuation,
combining marks, CJK text, consecutive spaces, CRLF, and omitted bidi controls.
A word hint before a space is asserted to leave the UAX#14 line break prohibited;
the existing line-penalty migration comparisons remain unchanged. This does not
claim dictionary segmentation for Thai or implement knot aggregation.

`glyph_clusters_test.go` tests the output-side cluster assembler independently
and with the streaming producer. Cases include shared anchors, multibyte source
spans, preserved glyph offsets, RTL runs, control gaps, silent runs, leading
omissions, invalid/decreasing anchors, metadata validation, and final-cluster
completion. These tests establish assembly of source-associated glyph groups,
not knot creation, width calculation, or safe-break certification.

`KnotStyler`, which currently adjusts knot width bounds, is distinct from the
draft `StyleResolver`. The existing fixed shaper ignores `SetStyle`; this does
not mean that the encoder ignores style boundaries or knot adaptation.

## Dependency Policy

### Package Ownership

The root engine imports `contracts` for service definitions and `helpers` to
construct its default metric shaper. `helpers` imports `contracts`, never the
root engine. `contracts` imports neither the root nor helper implementations.
Both may use `dimen` and the accepted dependencies below.

`contracts` owns `FontLoader`, `FontFace`, `FontScale`, `RuneReader`, `Shaper`,
`StreamingShaper`, and `StyleResolver`, together with their parameter/result
types and constants. `helpers` owns the four deterministic implementations and
their focused tests. `KnotStyler` and `Hyphenator` remain with the root paragraph
model; the linebreakers retain their own consumer interfaces.

`contracts.ErrIllegalArguments` is shared with `khipu.ErrIllegalArguments`.
`helpers.ErrShaperBusy` and `helpers.ErrShaperClosed` retain their corresponding
root error identities. Root type aliases and forwarding constructors are
collected in [compat.go](../compat.go), with
[compatibility coverage](../compat_test.go).

Helpers' external-package tests import `contracts` and `helpers`. The example
combining a helper with the engine is a root `khipu_test` example. Future testing
support that needs whole paragraphs belongs in `khiputest`, which may import
both packages without reversing the production dependency direction.

### Core Packages

Accepted dependencies for ordinary core packages are:

- `cords`, including `styled`
- `uax`, including `bidi`
- `schuko` and its subpackages
- Go standard library and extended modules

Core encoding and linebreaking should depend on khipu-owned interfaces for
external font and shaping services. Concrete backend types should not cross
those boundaries. Backend adapters may depend on concrete implementations.

### Backend Integration Tests

Concrete font implementations, including `opentype/ot` and `opentype/otquery`,
are appropriate dependencies for adapter integration tests. Such tests check
agreement about units, source mappings, cluster semantics, errors, and ownership.
They should supplement the deterministic core suite.

Avoiding backend imports in core packages and excluding backend dependencies
from the core module are different policies. If module-level isolation is
required, put these tests with the adapter or in a separate integration module.
Build tags alone do not create a separate module dependency boundary.

## Useful Default Implementations

### Public Construction

| Helper | Construction | Zero value |
| --- | --- | --- |
| `helpers.SyntheticFont` | `helpers.NewSyntheticFont()` | Ready to use |
| `helpers.FixedWidthShaper` | `helpers.NewFixedWidthShaper(cellWidth)` returns a pointer and error; width must be positive DU | Ready to use, 5 pt cells |
| `helpers.DefaultStyleResolver` | `helpers.NewDefaultStyleResolver()` | Ready to use |
| `helpers.SyntheticStreamingShaper` | `helpers.NewSyntheticStreamingShaper(outputCapacity)` returns a pointer and error; capacity must be nonnegative | Ready to use, unbuffered output |

Use pointers to the mutable shapers and do not copy them during use. A zero
explicit cell width is invalid; select the default by using the zero-value
metric shaper or `Config{}`. For streaming, zero capacity explicitly selects an
unbuffered channel. Positive capacities buffer that many output glyphs, never
input scalars. Drain concurrently with `Shape` even when using buffered output;
a run may exceed the buffer. Constructors do not start goroutines.

The [metric example](../helpers_example_test.go) demonstrates configuration of
the existing encoder. The [streaming example](../helpers/synthetic_streaming_shaper_test.go)
combines resolution, the synthetic face, scaling, concurrent production/draining,
and terminal error checking. Streaming helpers remain independent of `Config`.

### Synthetic Font and Fixed-Width Shaping

Prefer a synthetic font over loading Courier for the default environment. A
synthetic face needs no font discovery or file loading and gives tests simple,
fully specified arithmetic.

`SyntheticFont` implements `FontFace` with the metrics below. Its zero value and
`NewSyntheticFont()` are ready to use. `SyntheticStreamingShaper` is its matching
shaper with a 500-font-unit ordinary advance. `FixedWidthShaper` is the separate
metric implementation, which directly reports DU widths.

| Property | Value |
| --- | ---: |
| Units per em | 1,000 |
| Ascent | 800 |
| Descent | -200 |
| Line gap | 0 |
| Ordinary glyph advance (synthetic streaming shaper) | 500 font units |

At `EmSize = 10*dimen.PT`, an ordinary glyph advances by `5*dimen.PT`.
`FontFace` provides metrics and scaling, but no glyph-advance query; the
synthetic shaper therefore owns the advance policy. A real shaping adapter must
reject a synthetic face it cannot interpret.

The Go face has no outlines, glyphs, character map, or font file. Its scaling
can be used independently of the draft streaming path:

```go
face := helpers.NewSyntheticFont()
scale, err := face.Scale(10 * dimen.PT)
if err != nil {
    // Nonpositive sizes return an error wrapping contracts.ErrIllegalArguments.
    return err
}
advance := scale.ToDU(500) // float64 DU: exactly 5*dimen.PT
```

See the executable [synthetic font example and tests](../helpers/synthetic_font_test.go).
The current encoder's fixed-width shaper remains independent of this face.

**Implemented:** the face and scale obey the immutable-query contracts.
Nonpositive em sizes return no scale and an error wrapping
`ErrIllegalArguments`. `ToDU(u)` computes `float64(u) * float64(emSize) / 1000`;
it preserves signs and fractional values subject to floating-point precision.
It does not invert axes, round to integer DU, clamp, or reject a result outside
the `dimen.DU` range. The caller accumulates fractions, then owns rounding and
range checks. Tests cover negative descent, fractional conversion, independent
scales from one face, and results outside the integer dimension range.

The metric shaper directly produces `dimen.DU` widths and does not use a font.
Its configurable public implementation preserves the existing `Config{}`
behavior and does not depend on streaming integration.

### Fixed-Width Character Policy

**Implemented:** the metric shaper emits one metric per successfully read rune.
`SetSource` resets its byte offset to zero; `SetStyle` does nothing. The complete
zero-width set is:

| Code points | Meaning |
| --- | --- |
| U+000A, U+000D | Line feed and carriage return |
| U+000B, U+000C | Vertical tab and form feed |
| U+0085 | Next line |
| U+2028, U+2029 | Line and paragraph separators |
| U+200B, U+2060 | Zero-width space and word joiner |

Every other successfully read rune receives the selected cell width (by default
`5*dimen.PT`). This includes ordinary
space, NBSP, tab, NUL, soft hyphen, combining marks, join controls U+200C/U+200D,
variation selectors, and bidi controls not listed above. Tab is one cell, not a
tab stop. The implementation does not use a general Unicode control or
default-ignorable classification. CRLF produces two zero-width metrics; line
breaking policy determines the resulting break, not the number of metrics.

This is an arithmetic model. For example, decomposed `e\u0301` measures two
cells, while precomposed `é` measures one. No normalization, ligatures, kerning,
mark attachment, or visual reordering occurs. A zero width does not itself
authorize a break, and a nonzero width does not prohibit one. Segmentation and
penalties remain the encoder's responsibility.

**Implemented:** the exported metric `FixedWidthShaper` retains those
rules, allowing a positive caller-selected cell width in `dimen.DU`. Its
default width remains `5*dimen.PT`; zero and negative explicit widths are
errors rather than requests for defaults. Zero-width and malformed cases needed
by tests belong in scripted fixtures. `NewFixedWidthShaper` reports invalid
widths with an error wrapping `ErrIllegalArguments`.

### Synthetic Streaming Policy

**Implemented:** `SyntheticStreamingShaper` uses the following deliberately
limited model. It implements `StreamingShaper` separately from the active
`Shaper` interface; `Config{}` still uses metric shaping.

| Concern | Implemented behavior |
| --- | --- |
| Accepted face | `SyntheticFont` values and non-nil `*SyntheticFont` pointers only; reject nil or foreign faces rather than guessing from their metrics |
| Glyph generation | One record for every valid input Unicode scalar, including the zero-width set; no scalars are omitted by this shaper |
| Glyph identity | `GlyphID = uint32(rune) + 1`; reserve zero. These are synthetic identifiers, not indices into an outline table or font file |
| Advances | `XAdvance = 500` font units except for the exact zero-width set above; `YAdvance = 0` |
| Positioning | `XOffset = YOffset = 0`; combining marks remain independent 500-unit records |
| Clusters | One scalar per cluster, anchored at the initial base plus its original input scalar index; never use a byte offset or emitted-record count as a substitute |
| Direction | Accept LTR and RTL. Emit both in logical source order with positive ordinary X advances; reject mixed or neutral directions, since the caller must supply a uniform run |
| Script and language | Preserve caller metadata; they do not change the synthetic output. Do not infer scripts, select fallback fonts, or perform bidi analysis |
| Features | Accept only an empty list. Reject every nonempty list, including disabled feature entries, so feature support is never implied by a successful call |
| Normalization | None; preserve scalar identity and source accounting |

No glyph omission is needed for this baseline. Scripted fixtures will cover
omitted scalars, shared clusters, ligatures, and positioned marks. If run
preparation omits bidi controls before shaping, it must account for those source
gaps when setting cluster bases; the shaper cannot infer text it never receives.

For valid UTF-8, each successful read must describe one scalar and its correct
byte length. The streaming default rejects invalid scalar values or
inconsistent lengths, including a one-byte decoding-error U+FFFD. A literal
U+FFFD encoded in three bytes remains valid input. The active encoder already
validates UTF-8 before using its metric shaper; the standalone metric helper
delegates reading to its source and does not add these streaming checks.

An empty run emits no records and leaves the cluster base unchanged. Count
cluster positions in a wider internal type: a final scalar may use
`math.MaxUint32`, but the exclusive next base must not wrap. A subsequent
nonempty run fails unless `SetInitialCluster` supplies a representable base.
Reject overflow before emitting an unrepresentable record. Earlier records may
remain drainable on failure; paragraph publication still requires success.

At 10 pt em size, the streaming model and current metric shaper have matching
total widths for the same supplied scalars. They still have different output
types, units, coordinate systems, and lifecycles. Neither is a reference oracle
for real typography.

### Default Style Resolution

The initial idea of a `VoidStyleResolver` returning constructor-time values
needs reconciliation with the current interface contract. `ResolveStyle` says
nil style means no overrides of caller-supplied defaults; an uninterpretable
style must produce an error.

**Implemented:** the stateless `DefaultStyleResolver`:

- returns a complete copy of validated caller defaults for nil style
- rejects every non-nil style, even when it appears equal to a preceding style;
  this resolver has no knowledge of application-defined style representations
- requires a non-nil usable font, positive em size, and either `SpacingFixed`
  or `SpacingElastic`; it does not install a font or repair invalid defaults
- preserves script, language, spacing, and feature values without inference;
  script and language zero values are preserved rather than guessed
- clones the feature slice so later caller changes cannot alter the returned
  result, and does not mutate inputs or retain per-run state

The resolver does not check backend feature support or acquire a scale.
Run preparation requests `Font.Scale(EmSize)` and propagates its errors;
the shaper rejects unsupported faces or features. A nil style with a nonempty
feature list can therefore resolve successfully and subsequently fail with the
synthetic shaper. These are separate responsibilities.

Put the synthetic font and selected em size in the caller's default
`ResolvedRunStyle`. Supply inherited defaults independently for every run;
never use the preceding run's resolved result as implicit inheritance. No
constructor-time font or size overrides the supplied defaults.

`VoidStyleResolver` and `ConstantStyleResolver` are deferred. Ignoring all styles
and returning constructor-time values would need an explicit contract change.
Fixed spacing and an empty feature list remain valid explicit values, not
missing-value sentinels. Neither the resolver nor `SpacingElastic` chooses
numerical glue stretch/shrink capacities; that is a separate layout policy.

### Ownership and Construction

Convenient constructors should produce a coherent environment without mutable
global defaults. Immutable faces and scales may be shared; mutable shapers must
have the ownership required by their contracts.

| Object | Ownership and lifetime policy | Status |
| --- | --- | --- |
| `SyntheticFont`, returned `FontScale` | Immutable; safe to share. Creating another scale does not change existing scales | Implemented |
| `Config.Shaper`, `Config.Hyphenator` | Retained, not cloned. Serialize owner operations and access to mutable collaborators; a default metric shaper is private to its owner | Implemented |
| `Config.Params`, `Config.ParFillSkip` | Copy at construction; later caller changes do not configure the owner | Implemented |
| Paragraph and discretionary state | Keep source and shaping context stable while caches are used. Breaker runs replace selections; copy selections when comparing runs | Implemented |
| Default resolver result | Independent feature-slice storage per result; font may be shared under its immutable contract | Implemented |
| Prepared-run metadata | Resolve and publish before corresponding glyphs can arrive; retain an owned feature copy and immutable source mappings during consumption | Draft contract |
| Synthetic streaming instance | One producer, one draining consumer, several sequential runs, then terminal completion; fresh instance for later shaping work | Implemented helper; owner integration remains proposed |

**Proposed construction policy:** use a factory for streaming instances when
wiring the owner. Allocate a fresh instance for each paragraph shaping stream
and each later discretionary shaping operation. Several runs may share one
stream before `Finish`; a completed or failed stream is never recycled.
Do not share a mutable stream globally or between owners.

Caller-owned source and feature data must stay stable during a synchronous
`Shape` call. The default does not retain its reader or borrowed feature
storage after the call. Consumers read value records, and cancel the producer's
context if they stop draining. Backend resource disposal remains the adapter
and caller's responsibility; these interfaces have no `Close` operation.

Not every interface needs a new default type. Nil already disables lazy
hyphenation, and rectangular paragraph shapes already have a constructor.
A table-based hyphenator is more useful as a fixture for selected test words.

### Streaming Lifecycle Decisions

The existing draft requires backpressure, terminal failure reporting, and
non-disruptive rejection of `Finish` or `SetInitialCluster` during `Shape`.
The following choices are **implemented by the synthetic default**:

- Use one stable, bounded output channel per instance. Output capacity is a
  construction concern, independent of input `ShapeBufferOptions`.
- Accept zero-valued input buffering options and both declared flush modes.
  Each scalar is already a complete synthetic cluster; neither mode inserts
  completion markers. Reject unknown flush values and nonzero watermark or
  maximum-buffer requests initially rather than pretend to implement buffering.
- Once an idle instance accepts a `Shape` operation, invalid parameters, reader
  errors, or cancellation terminate it. Close output once and retain the first
  terminal error in `Err`. EOF is successful completion of that run, not an
  error and not the end of the stream.
- Reject overlapping `Shape` calls and lifecycle changes during an active call
  without disturbing it. Such rejected calls do not become the stream's `Err`.
- `Finish` on an idle healthy stream closes output successfully. Repeating it
  is harmless; after failure it returns the retained terminal error. `Err`
  remains nil after successful completion.
- Reject `Shape` and `SetInitialCluster` after successful completion or failure;
  do not reopen output or replace the terminal result. Before completion, an
  idle `SetInitialCluster` changes only the next base.

Check cancellation while sending to a blocked output channel and between reads.
It need not interrupt a source read already in progress. Failure may leave
previous records in the channel; the consumer drains or discards them, while
future streaming encoder integration must publish no partial paragraph. The
[streaming tests](../helpers/synthetic_streaming_shaper_test.go) cover lifecycle
rejections, terminal errors, cancellation under backpressure, and scalar overflow.
The current metric shaper has no channels or context-cancellation contract.

Busy operations return `ErrShaperBusy`, and operations on a completed or failed
stream return `ErrShaperClosed`. Invalid accepted arguments and scalar overflow
wrap `ErrIllegalArguments`; reader and context errors propagate. `Err` records
only an accepted shaping operation's terminal failure. `Finish` returns that
failure after the stream has failed and nil after successful completion.

### Acceptance Examples for the Defaults

These examples define the default behavior; the focused helper tests and
executable examples exercise the policies. Widths below assume the metric default or a 10 pt em size
for synthetic streaming. Byte and scalar anchors are relative to an initial
base of zero, before paragraph knot aggregation.

| Input or operation | Expected result |
| --- | --- |
| `aéz` | Metric anchors `0, 1, 3`; streaming anchors `0, 1, 2`; total width 15 pt |
| `e\u0301` versus `é` | Two records and 10 pt versus one record and 5 pt; no normalization |
| `a\u200Bb` | Three records, advances 5/0/5 pt; the zero-width record remains a cluster anchor |
| `a\tb` | Three equal-width records, total 15 pt; no tab-stop expansion |
| `a\r\nb` | Four records, advances 5/0/0/5 pt; break opportunities are a separate encoder assertion |
| Hebrew `אב` in an RTL run | Streaming emits alef then bet, scalar anchors 0 then 1, each with advance 500; no reversal or negative advances |
| Runs `aé`, then `z` | Metric offsets restart at zero per run; streaming anchors continue `0, 1`, then `2` |
| Nil style with fixed spacing and no features | Resolver preserves these explicit values and the supplied font and size |
| Nil style with enabled `liga` feature | Resolver copies the feature; synthetic streaming rejects the nonempty feature list |
| Non-nil application style | Default resolver fails; no fallback to constructor-time or previous-run values |
| Caller changes the original feature slice after resolution | Returned resolver result is unchanged |
| Cancellation while output is full | Streaming `Shape` returns cancellation, closes output, and records terminal failure without requiring further sends |

Lifecycle, invalid-input, control-exception, and overflow cases have focused
tests. Step 3 will extract reusable fixtures and adapter conformance tests.
The baseline does not replace fixtures for more realistic glyph clusters.

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

## Proposed Implementation Sequence

1. **Document existing guarantees and proposed policies — documented November
   5, 2026.** The evidence inventory and policies above specify metrics, exact
   control handling, scalar/glyph identity, feature rejection, style inheritance,
   ownership, and streaming lifecycle. Acceptance examples make the next work
   reviewable. Proposed behavior remains unimplemented unless marked otherwise.
2. **Make deterministic helpers reusable — implemented November 5, 2026.**
   `FixedWidthShaper` exposes configurable metric shaping while preserving
   `Config{}`. `SyntheticFont`, `DefaultStyleResolver`, and
   `SyntheticStreamingShaper` support standalone deterministic streaming tests.
   Public constructors, zero-value behavior, bounded output capacity, focused
   tests, and executable external-package examples are available. Encoder
   integration remains future work. Nonzero input-buffer tuning is deliberately
   rejected; richer buffering needs its own specified policy.
3. **Extract reusable fixtures and contract tests — next step.** Build on existing scripted
   shapers, add table-based hyphenation, and cover byte/scalar coordinates and
   streaming lifecycle behavior. A downstream adapter should be able to run the
   applicable suite using its own constructor.
4. **Expand invariant tests and fuzzing.** Target source mappings, arithmetic,
   cluster boundaries, and failure handling. Retain reproducible regressions
   found by fuzzing and test each linebreaker's own policies.
5. **Add structural visual diagnostics.** Produce a useful SVG and structured
   dump from an encoded paragraph and break decisions without requiring a real
   font backend or claiming final layout.
6. **Add real-backend integration and, later, glyph proofs.** Keep backend
   dependencies isolated according to the module policy, use pinned fixtures,
   and introduce rendering checks when glyph provenance and line placement are
   available.
