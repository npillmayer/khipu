# Reusable paragraph material

Preparation returns immutable SoA `Khipu` plus a synchronized `Discovery`.
`PreparedParagraph` and mutable working Khipus were replaced in reflow PR 1.
[Executable examples](../example_test.go) and the [root guide](../README.md)
show the complete public workflow. Calls are synchronous and take no context.

## Ownership and reuse

| Handle | Contents and lifetime |
| --- | --- |
| `Khipu` | Immutable base knots, glyphs, source/bidi context, atomic IDs, fixed discovery mode; retained while reflow remains possible |
| `Discovery` | One incarnation's fixed recipe and synchronized complete alternative cache; shared across trials, errors retryable |
| `AtomicBinding` | Shared base plus canonical copied signed advances, no frame width or selections |
| `Tikray` | Immutable complete decisions, selected alternatives, captured geometry, policy and diagnostics; independent of later trials |
| `BreakCheckpoint` | Copied accepted prefix, consumed-atomic values and resume position; no chain of prior plans |
| `FrozenKhipu` | Final horizontal geometry and stable glyph/source records; retains none of the working handles above |

Preparation already builds knots. Binding does no shaping and does not clone
full base knot planes. A new preparation gets a distinct process-local
`IncarnationID` even for identical input. Stable borrowed source, styles and
services are required. Discovery serializes misses; services shared across
incarnations still require appropriate serialization. No runtime or replay
serialization of these handles is defined.

## Anchors and atomic boundaries

Attach unique nonzero `contracts.AtomicID` values to ordered U+FFFC occurrences
using original paragraph-relative UTF-8 byte offsets in `ParagraphInput.Atomics`.
Unattached U+FFFC stays text. `PrepareOptions.AtomicBoundaries` may allow or
prohibit optional breaks at interior atomic edges, but cannot erase mandatory
breaks. Anchors retain original source/scalar identity and bypass shaping.

`Khipu.Knot` returns a `KnotTemplate`. Atomic templates have `MetricsKnown=false`
and zero placeholder widths. `BindParagraph(k, advances)` requires exactly one
finite signed advance per ID in any order. Zero and negative advances are valid;
unknown, duplicate, missing and out-of-domain entries fail. Getter slices are
copies. Clients retain object payload, border/margin geometry and nested output;
equal horizontal advances do not prove equal vertical/interior realizations.

## Measurement and binding contracts

`MeasureIntrinsic(k, discovery, request)` uses base material without line geometry,
vertical metrics, binding or breaking. It requires matching nonnil handles even
with `NoHyphenation`. Supply exactly one ordered finite signed min/max range per
atomic ID, including resolved horizontal margins. Request order is arbitrary;
storage is consumed, not retained. Complete signed spans are normalized into
finite nonnegative DU results. Fill, skips and stretch/shrink are excluded;
ordinary/mandatory breaks and edge trimming retain their existing semantics.

## Hyphenation policies

`PrepareOptions.Hyphenation` fixes `NoHyphenation`, `ExplicitHyphenation` or
`AutomaticHyphenation` for the incarnation. Enabled modes require a configured
`Hyphenator`. `EncodeParagraph` chooses automatic (including explicit) when a
hyphenator is supplied, otherwise none. Trial scoring changes do not change the
discovery universe. Changed discovery rules require a new preparation.

`Discovery.Alternatives(index)` publishes complete immutable batches, including
successful emptiness. Errors never publish a partial pair or batch and can retry.
Automatic variants retain valid nonzero IDs; zero IDs receive unused positive IDs
in response order after reserving retained nonzero IDs. Explicit variants ignore
provider IDs and use source-position/material order. Candidates remain in tie
order. `IDs` and `Candidates` return detached values; clients cannot inject or
select candidates by mutating a cache.

Intrinsic measurement supports NoHyphenation and bounded explicit hyphenation:
at most one eligible position and one permitting variant per textbox. On a cold
cache, multiple positions fail before provider calls; multiple permitting variants
fail after complete validation/feature admission but before new streams or
CheckFace. A supported request shapes the complete batch, including prohibited
variants. Warm measurement applies admission to the complete existing set.
Automatic intrinsic measurement, multiple positions/variants and repeated splitting
remain `ErrUnsupportedPolicy`; rejection never poisons layout discovery.

## Independent trials

Create a `BreakRequest` from a binding, matching discovery and explicit geometry.
First-fit and KP return a complete immutable `Tikray`, including the final trailer.
A failed search returns nil. `NewTikray` is the validated publication seam for
custom breakers: it copies decisions, validates legal progress and mandatory
boundaries, and resolves only already-published alternative IDs. It never
queries shape/discovery and does not certify optimality.

`Lines` and `Policies` return copies. `Diagnostics` returns a value; availability
flags distinguish absent scores/passes from zero. `PolicyForLine` uses zero-based
complete-paragraph indexes; nil/zero plans and invalid indexes return zero/false.
Continuation diagnostics describe its suffix search. `Freeze(plan)` takes no
substitute binding or geometry and cannot be changed by later trials.

## Checkpoint continuation

1. Produce a complete first-fit or KP Tikray.
2. Call `Checkpoint(plan, acceptedLines)`, from zero through the whole line count.
3. Bind any new suffix atomic advances to the same incarnation.
4. Put the checkpoint in `BreakRequest.Prefix` and call
   `firstfit.ContinueParagraph(request, params)`.

The initial gate is NoHyphenation. KP continuation and general replacement carry
remain deferred. A KP prefix can nevertheless precede a first-fit suffix.
`SeedFor(binding)` validates incarnation and consumed atomic advances without
prefix scans, shape queries, discovery or services. Its copied prefix/resume
values are scratch, not a certificate; `NewTikray` compares decisions with the
original checkpoint and rejects edits to the locked prefix.

Accepted widths/skips are imported automatically. The shape is queried only for
suffix line numbers, starting at acceptedLines+1. All accepted lines returns a
copied complete plan without suffix search. Full freezing/assembly may still
visit prefix content. Whole-paragraph bidi context and original coordinates remain
intact; a frame boundary does not create a new bidi paragraph. The upstream bidi
limitation is unchanged.

Process paragraphs in turn. Retain preparation/discovery/checkpoints only while
that paragraph has remaining flow or retries; then release them and retain frozen
fragments. Frame scheduling, page optimization and vertical/interior compatibility
are application concerns. No register/page solver is implemented by this API.
