# Khipu In The Typesetting Pipeline

Khipu is the horizontal text-layout component of the typesetting pipeline. It
turns normalized styled text and opaque inline occurrences into reusable shaped
material, intrinsic widths, and frozen visual lines. It does not interpret the
DOM, resolve CSS box sizes, assemble vertical line geometry, or paint glyphs.

The integration design separates **preparation**, **measurement**, and **final
layout**. CSS sizing can ask for intrinsic widths before it knows an object's
final advance, then reuse the same shaped text when laying out the paragraph.
This document describes the resulting design and the implemented Khipu contracts.
The [historical integration record](history/INTEGRATION_HISTORY.md) preserves
staged plans, superseded proposals and cross-repository validation evidence.
For runnable client code, use [Prepared Paragraphs](PREPARED_PARAGRAPHS.md).

## Position In The Pipeline

| Component | Responsibility at the integration boundary |
| --- | --- |
| DOM and style system | Element identity, cascade, inheritance and effective style values. |
| `css-box-layout` | Normalized formatting groups, containing constraints, recursive intrinsic scheduling, atomic sizing and authoritative document geometry. |
| Integration adapter (`tytui`) | Extract ordered text/object input, map source and box identities, translate supported CSS policy, convert units and publish document results. |
| Typography services | Resolve shaping styles and itemize text using caller-supplied fonts; skip attached atomic markers. |
| `khipu` and its breakers | Prepare horizontal material, preserve break policy, measure intrinsic advances, bind atomics, choose breaks and freeze visual material. |
| `typography/paragraph` | Assemble line heights, baselines, vertical stacking and paragraph-local atomic placements. |
| Renderer/application | Place glyphs and objects, apply document transforms, paint and retain or inspect results. |

These are ownership boundaries, not a requirement that Khipu import the other
components. Khipu depends on neutral text/service contracts and accepts resolved
inputs. `contracts` supplies the shared atomic and shaping vocabulary; concrete
font backends remain behind those interfaces. Glyph coverage and fallback are
caller responsibilities, not a Khipu preflight guarantee.

The CSS core schedules nested objects before asking for the outer group's final
layout. The adapter passes their resolved contributions into Khipu; it does not
start a second recursive sizing traversal from inside a shaping or breaking call.

## Data Flow And Public Operations

The [public lifecycle diagram](../README.md#public-lifecycle) shows Khipu's calls.
At the pipeline boundary the data flow is:

```text
normalized inline group + resolved styles + source/box associations
    -> adapter extraction and typography itemization
    -> ParagraphInput + PrepareOptions
    -> PrepareParagraph -> PreparedParagraph
         -> MeasureIntrinsic(atomic min/max ranges) -> intrinsic widths
              -> upstream width selection and nested atomic sizing
         -> BindParagraph(definite atomic advances) -> working Khipu
              -> BreakParagraph(shared layout settings) -> indices + selections
              -> Freeze(same layout settings) -> FrozenKhipu
    -> paragraph assembly -> line geometry + atomic placements
    -> adapter publication -> accepted document geometry and retained results
```

| Operation | Inputs that become available here | Output |
| --- | --- | --- |
| `PrepareParagraph` | Styled paragraph, atomic IDs/anchors, boundary overrides and hyphenation mode; no target line width. | Owner-bound `PreparedParagraph` with shaped base text and unbound atomic slots. |
| `MeasureIntrinsic` | One `AtomicIntrinsic` min/max contribution per attached ID. | Detached `IntrinsicWidths`; no working paragraph or break decisions. |
| `BindParagraph` | One definite `AtomicAdvance` per attached ID. | Independent mutable working `Khipu`, sharing immutable prepared material. |
| Either `BreakParagraph` | Working paragraph, resolved `khipu.Params` geometry and algorithm-specific parameters. | Line-ending indices and selected discretionaries on that working paragraph. |
| `Freeze` | Those indices and selections, the same layout settings and creating owner. | Opaque `FrozenKhipu` with final widths, visual order and provenance. |

`EncodeParagraph` remains the direct text-only entry point for callers that do
not need preparation reuse or unbound atomics. Existing text-only clients do not
have to adopt the mixed-input API. The prepared workflow also accepts text alone.

### Why Preparation Is Separate From Layout

Intrinsic measurement needs shaping: character counts or nominal font metrics
cannot account for ligatures, marks, features and resolved spacing. It does not
need a target width, final atomic sizes, selected lines or vertical assembly.

Preparation therefore retains width-independent horizontal material. Measurement
and binding can be repeated without reshaping base text. Binding creates fresh
working planes, caches and selections so one layout attempt cannot overwrite
another's decisions. An earlier frozen result remains valid after remeasurement,
rebinding or rebreaking, subject to the shared-source/service lifetime contract.

This is reuse of a preparation, not a global cache or automatic invalidation
system. Changed text, styles, fonts or policy require a new preparation. The
integration adapter owns the scope of reuse; the bounded document integration
uses invocation-local preparations rather than cross-document cache keys.

## Normalized Input And Opaque Atomics

The adapter extracts the normalized formatting group in order, rather than
assuming that a DOM subtree is a paragraph. It traverses ordinary inline content
and emits one marker for an atomic boundary while leaving that object's interior
to its separately scheduled group. Whitespace normalization must consider the
whole ordered text/object sequence so spaces beside objects are not lost.

Each attached occurrence uses one U+FFFC OBJECT REPLACEMENT CHARACTER and a
`contracts.AtomicAnchor`. The anchor contains a nonzero paragraph-local ID and
the marker's first paragraph-relative UTF-8 byte offset. IDs are unique within
the paragraph and anchors are strictly ordered. Unattached U+FFFC remains text.
The adapter keeps the association between these IDs and its document/box IDs;
Khipu neither stores a CSS box nor dereferences the object's payload.

Atoms enter the horizontal stream directly, bypassing glyph production and
style/font resolution. `KTAtomic` distinguishes them from textboxes. They are
indivisible, never hyphenated, and count as content for line-edge trimming even
when their advance is zero or negative. Their identities survive logical
materialization, bidi permutation, width allocation and freezing.

The atomic contract deliberately separates four kinds of data:

| Phase | Atomic information |
| --- | --- |
| Preparation | Occurrence ID, source anchor and resolved boundary policy. |
| Measurement | Signed minimum and maximum inline contributions. |
| Binding | One fixed signed advance, including caller-resolved horizontal margins. |
| Vertical assembly | Border dimensions, margins, baseline/alignment and payload, retained upstream. |

Binding sets `MinW == W == MaxW` for an atom. Its intrinsic range does not make it
elastic or permit a breaker to resize it. Both measurement and binding require
exactly one entry per attached ID; duplicate, missing and unknown IDs are errors.
Binding does not enforce a previously queried intrinsic range.

`PrepareOptions.AtomicBoundaries` expresses resolved optional-break policy at
interior edges immediately before or after attached atoms. Overrides are sorted,
unique paragraph-byte offsets with `AllowBoundary` or `ProhibitBoundary`.
Omitted edges retain Unicode policy. Endpoints cannot be overridden and mandatory
breaks cannot be downgraded. Policy is applied before knot grouping; changing
penalties after grouping would be too late to restore a lost boundary. CSS
interpretation remains in the adapter.

## Intrinsic Sizing Contract

`MeasureIntrinsic` uses natural advances and the preparation's break policy:

- Min-content takes supported optional breaks; max-content suppresses them.
- Both respect mandatory boundaries and trim line-edge discardables using the
  same content rules as layout. An unbreakable sequence may span several knots.
- Atomic minimum contributions feed min-content; maximum contributions feed
  max-content. These are upstream size inputs, not font metrics.
- Side skips, generated paragraph fill, stretch/shrink and breaker scores do not
  contribute. `KnotCore.MinW`/`MaxW` describe elasticity, not intrinsic bounds.

Checked signed sums are normalized after complete spans. Results satisfy
`0 <= MinContent <= MaxContent < dimen.Fil`. These bounds do not promise that
any later signed-advance layout will have representable nonnegative line totals.

An upstream sizing algorithm can use these widths when choosing an available
content width, then resolve each atomic advance before binding. It must not
substitute a previous line-break result, a zero-width trial layout, or a widest-
knot estimate for the intrinsic query.

### Hyphenation And Preparation Reuse

| Preparation mode | Bound layout | Intrinsic measurement |
| --- | --- | --- |
| `NoHyphenation` (default) | Ordinary wrapping without discretionary discovery. | Supported. |
| `ExplicitHyphenation` | Authored soft-hyphen alternatives supplied by the owner. | Supported with at most one eligible split and one break-permitting material variant per textbox. |
| `AutomaticHyphenation` | Explicit and automatic alternatives through lazy discovery. | Returns `ErrUnsupportedPolicy`. |

Enabled modes require a configured `Hyphenator`; layout also needs the owner as
the breaker's discretionary provider when discovery is required. Unsupported
measurement policies fail explicitly rather than silently measuring different
semantics from layout. Successive splits of the same textbox are not supported.

Explicit measurement lazily shapes complete replacement batches and memoizes
successful results, including empty batches, on the preparation. Explicit-mode
bindings import that material when their provider is asked. This cache belongs
to the owner and is separate from a binding's selected break decisions.
See [hyphenation contracts](HYPHENATION.md) for the full policy.

## Final Horizontal Layout And Downstream Assembly

After upstream sizing, bind definite advances and resolve one layout value:
`khipu.Params` supplies a `LineShape` and physical left/right skips. Algorithm
parameters separately control first-fit or Knuth–Plass behavior. Use the same
resolved geometry for breaking and freezing; otherwise the chosen line breaks
and realized widths describe different layouts. Shape line numbers are one-based
and target widths include the skips.

Break indices alone are not a complete decision: selected discretionary variants
live on the working `Khipu`. Freeze before a later successful breaker call replaces
those selections. Freezing substitutes chosen fragments, trims edges, orders
whole knots visually, places final fill and allocates glue widths. It does not
reshape text or reverse glyph arrays within a box.

The frozen API is the downstream boundary:

- `Line` and `Knot` return value records for final widths, physical skips, source
  ranges and fit diagnostics.
- `Atomic` identifies an occurrence in visual knot order; the adapter maps it
  back to its retained box identity. `CopyText` returns false for atoms.
- `CopyText` returns independent glyph, cluster and feature slices for text,
  while immutable font/scale services remain shared.

Underfull or overfull lines are successful results with diagnostics. Finite
stretch may exceed declared capacity; shrink stops at minima. Elastic non-glue
material is unsupported, and negative completed content or skip-inclusive line
widths are rejected. A successful break does not itself guarantee successful
freezing. See [freezing](FREEZE.md) and [frozen queries](FROZEN-API.md).

Paragraph assembly consumes these horizontal results together with upstream
vertical metrics. It determines line heights, baselines and atomic border origins;
Khipu stores none of those policies. For the bounded bottom-margin-edge alignment
profile, the assembler accounts for signed margins once when translating an
atomic advance into a border-box placement. The CSS core receives extent and
placements, while typed paragraph results retain the text/glyph data for rendering.

## Coordinates And Unit Conversion

| Domain | Contract |
| --- | --- |
| Atomic anchors and boundary overrides | Original paragraph-relative UTF-8 byte offsets. |
| Working `KnotCore.Pos` | Absolute source byte position, including paragraph offset. |
| Frozen source ranges | Half-open original paragraph byte/scalar ranges; `SourceOffset` converts byte positions to document positions. |
| Original text shaping | Retained cluster anchors use original paragraph scalar coordinates; filtering markers does not erase provenance. |
| Replacement shaping | Separate replacement-local byte/scalar coordinates beginning at zero; original source provenance remains on the frozen knot. |
| Layout advances | Signed fixed-point `dimen.DU`; finite magnitudes below `dimen.Fil`. |
| Retained glyph advances/offsets | Signed font units, interpreted with the retained `FontScale`. |

The adapter owns CSS-unit conversion and rounding/range checks. `dimen.PX` equals
`dimen.BP` numerically and is not a CSS-pixel conversion policy. Preserve fractional
font-unit conversions until the completed advance is rounded; do not reconstruct
source positions from glyph counts or assume a character-by-character mapping
for changed-spelling replacements. The [coordinate examples](FROZEN-API.md#replacement-coordinates)
illustrate the distinct domains.

## Ownership, Failure And Publication

Use the creating `Khipukamayuq` for preparation, measurement, binding and freezing,
and serialize its operations. Source text, styles and retained font services must
remain stable for all derived results. Request slices are consumed without being
retained. Preparations own reusable material and memoization; bindings own mutable
working state; frozen results own their final planes and retain immutable shaping
and source references. Frozen results support concurrent reads under those
lifetime rules and do not depend on the owner remaining alive.

Owner operations take a nonnil context and return no partial result on failure;
failed measurement returns zero widths while leaving the preparation reusable.
Use `errors.Is` for wrapped argument, unsupported-policy and context errors.
Cancellation cannot interrupt every synchronous source/backend operation.
Breakers and lazy provider calls have no context argument: an earlier encoding
context does not cancel a later search.

A failed breaker preserves previous selections, although successful candidate
batches discovered before the error may remain cached. Frozen publication leaves
its input unchanged. The adapter provides the wider document transaction: build
invocation-local work, publish geometry, source associations and retained results
only after the whole document succeeds, and preserve earlier accepted results
when a later attempt fails. Khipu supplies paragraph-level guarantees, not a
cross-module transaction manager.

## Supported Integration Profile And Limits

The completed integration record describes a bounded document profile:
`NoHyphenation`, normal CSS line height, bottom-margin-edge atomic alignment,
explicit zero horizontal side skips, fixed/auto widths, resolved pixel edges and
explicit signed margins. Khipu's bounded explicit measurement is also available
to direct clients; its presence does not enable CSS manual/per-span hyphenation
mapping in the adapter.

Automatic intrinsic hyphenation, successive textbox splits, general baseline and
vertical-alignment policies, broader inline fragmentation, vertical writing,
pagination and target-width text shaping remain outside this design's supported
profile. Preparation reuse does not define cross-invocation cache invalidation.
The documented bidi L1 approximation and lack of shaping-arena compaction remain
Khipu limitations.

Khipu's tests cover mixed input, intrinsic measurement, reuse, source coordinates,
break/freeze behavior and result lifetime. The [testing guide](TESTING.md) maps
contracts to executable evidence. Cross-repository proofs and their dependency
versions belong to the [archived integration record](history/INTEGRATION_HISTORY.md);
passing this module's suite alone does not establish conformance of the whole
CSS/font/rendering stack.
