# Typesetting Pipeline Integration

Planning analysis and implementation status, 2026-11-14.
The stage 1 section records the agreed implementation contract, including
the resolutions of its remaining design questions. Stage 2 is implemented in
khipu, stages 3A–3C in typography, and stages 3D–3F in tytui (with shared proof
walking in typography/render). Stage 4A ordinary intrinsic measurement is implemented
in khipu and stage 4B exposes it through typography's prepared paragraph.
Stage 4C separates invocation-local preparation from accepted results in tytui;
stage 4D connects CSS intrinsic measurement and bounded auto-width layout.
Stage 4E adds bounded explicit discretionary measurement and shared preparation
reuse in khipu/typography. Later integration remains planned.
Source inspection covered sibling `css-box-layout`, `tytui`, `typography`, and
`cords` checkouts. The initial review was source-only; implementation checks are
recorded with the completed stages below.

## Objective and recommendation

Make khipu adaptable to `css-box-layout`'s `InlineLayouter` and
`IntrinsicMeasurer` through its public API. Khipu need not implement those
interfaces or import CSS layout types.

Recommended direction:

- Prepare shaped text and horizontal knot material when accurate measurement or
  layout is first requested; reuse that preparation across the two operations.
- Add explicit, indivisible atomic inline items with stable identities.
- Provide a dedicated intrinsic-width operation, separate from line breaking.
- Keep vertical assembly and atomic placement in `typography/paragraph`.
- Initially represent objects with U+FFFC anchors in the paragraph cord and a
  separate typed attachment table; leave the rope storage model unchanged.
- Build paragraph input from normalized inline groups, using styled-document
  properties and retaining DOM provenance separately.

## Evidence and baseline boundaries

This section records the source analysis before stage 2. The stage 2 and 3A–3F status
below describes the implemented changes; intrinsic integration remains planned.

Prefer current source signatures over older integration sketches. In particular,
the old `AtomicSizer` callback and line-box return contracts are superseded.

| Component | Current behavior | Integration gap |
|---|---|---|
| CSS inline layout | Supplies normalized group, owner, definite CSS-pixel content width, used values, and prepared atomic border sizes | Actual typography adapter and placements |
| CSS intrinsic measurement | Supplies group-local nested atomic min/max border widths and resolved edges | Actual paragraph measurement |
| Khipu | Shapes, encodes, breaks, reorders, and freezes paragraphs | Atomic input/identity and intrinsic queries |
| Typography paragraph engine | Orchestrates encode/break/freeze and assembles line heights and a boxed vlist | Mixed input, atomic vertical contributions, and placements |
| Cords styled text | Stores styled UTF-8 and resolves paragraph bidi | No attachment envelope; storage itself is sufficient for placeholders |

Primary local references:

- [CSS interfaces](../../css-box-layout/layout/interfaces.go),
  [intrinsic scheduling](../../css-box-layout/layout/intrinsic.go), and
  [atomic preparation](../../css-box-layout/layout/atomic.go).
- [CSS integration contract](../../css-box-layout/doc/Document-Layout-Integration.md)
  and [frozen decisions](../../css-box-layout/doc/Exec-Roadmap.md).
- [Tytui document exploration](../../tytui/doc/DOCUMENT_LAYOUT_EXPLORATION.md),
  [pipeline ownership](../../tytui/doc/type-integr.md), and
  [paragraph engine plan](../../tytui/doc/PARAGRAPH_ENGINE_PLAN.md).
- [Khipu algorithm](KHIPU_ALGORITHM.md), [frozen API](FROZEN-API.md),
  [hyphenation](HYPHENATION.md), and [whitespace policy](WHITESPACE.md).

## Ownership and dependency direction

| Owner | Responsibility |
|---|---|
| `w3c-dom` | DOM identity, cascade, inheritance, reusable CSS value services |
| `css-box-layout` | Box normalization, containing constraints, recursive intrinsic scheduling, atomic sizing, authoritative document geometry |
| Integration adapter | Collect normalized groups, translate supported CSS policy, retain source/BoxID associations, convert units, publish document results |
| Caller / upstream font preparation | Ensure supplied fonts cover the text assigned to them; arrange any coverage checking and fallback needed before typesetting |
| Typography resolver/itemizer | Style resolution and script/itemization with supplied fonts; bypass attached objects. Existing coverage checks are transitional, not the intended ownership boundary. |
| `khipu` | Horizontal preparation, break opportunities, intrinsic advances, breaking, frozen visual material and provenance |
| `typography/paragraph` | Line-height contributions, baselines, vertical stacking, paragraph-local atomic placements |
| Rendering/application | Paint, document transforms, inspection, replay and proofs |

Khipu remains independent of DOM, CSS layout, and concrete font backends.
CSS core continues to prepare nested atomics before delegating the outer group;
no adapter-initiated recursive sizing callback is needed.

**Glyph coverage is the caller's responsibility.** Khipu and paragraph layout
trust the supplied text-to-font assignments; they do not perform a coverage
preflight or search for fallback fonts. Stage 3 introduces no new coverage
validation, including for callers supplying prepared styled text directly.
Structural input validation and propagation of shaping/backend errors remain
unchanged; neither is a promise to verify glyph availability.

The typography itemizer currently performs coverage checks and coverage-driven
fallback selection. This describes existing behavior, not the intended permanent
allocation of responsibility. **Future task: walk back coverage checking in the
itemizer and relocate that functionality to a separate caller-side font-preparation
facility.** Define its destination/API and the migration of dependent fallback
behavior separately. Stage 3 does not implement that relocation or expand the
existing checks; while they remain, attached object markers must bypass them.

```text
normalized inline group
    -> adapter + typography itemization
    -> prepared text knots + atomic anchors
         -> intrinsic query -> CSS width selection and atomic preparation
         -> bind final atomic advances -> break -> freeze
    -> paragraph assembly: line heights + atomic placements
    -> adapter: extent + BoxID placements, retained typed paragraph result
```

## Measurement: shape first, defer final layout

The current intrinsic interface returns authoritative bounds with no estimate or
accuracy indicator. CSS core uses them directly for shrink-to-fit selection:

```text
usedContentWidth = min(max(minContent, availableContent), maxContent)
```

Character counts and nominal glyph metrics can disagree with layout because of
fallback, ligatures, combining marks, font features, and spacing. Do not use them
as an implicit implementation of this interface. An explicitly approximate API
could be considered later if a caller demonstrates a need.

Accurate measurement needs shaping and break analysis, but does not need final
atomic widths, selected lines, visual ordering, freezing, or vertical assembly.
Thus width-independent knotting should be reusable; a fully bound working
`Khipu` need not be the public measurement prerequisite.

The existing [performance measurements](PERFORMANCE.md) show synthetic encoding
costing substantially more than ordinary breaking. Reuse is therefore the first
optimization to pursue. These measurements were not rerun for this analysis.

### Intrinsic semantics

- Max-content uses natural advances with optional wrapping suppressed and forced
  breaks respected.
- Min-content evaluates supported wrapping opportunities, including supported
  discretionary replacements.
- Generated paragraph fill and alignment stretch do not contribute.
- Measurement and layout use the same whitespace and break policy.
- Style/shaping boundaries do not automatically introduce line breaks: an
  unbreakable sequence can span several knots.
- Atomic min/max contributions are supplied separately from final advances.

`MinContent`/`MaxContent` must not reuse `KnotCore.MinW`/`MaxW`: those fields
describe line-fitting elasticity. Summing them, finding the widest knot, or
running a very wide/zero-width paragraph is not an intrinsic measurement
algorithm. Breaker scoring, emergency behavior and mutable selections are not
part of intrinsic sizing.

The ordinary-wrapping case can scan legal boundaries and natural advances with
the appropriate edge treatment. Discretionary replacement paths require their
own evaluation; they cannot be inferred from previously selected breaks.

### Hyphenation limitation

Discovery is currently lazy. Intrinsic measurement must discover the alternatives
required by its policy rather than depend on a previous layout's cache contents.
The current model allows one split per textbox: multiple soft hyphens offer
alternative single splits, not successive splits of the remaining fragment.

First establish exact measurement under an explicitly selected ordinary-wrapping
profile. Add discretionary-aware measurement as a separate substep. Before
claiming general CSS min-content support, extend repeated splitting consistently
in measurement and layout, or explicitly reject unsupported cases. Do not silently
disable hyphenation in measurement while retaining it in final layout.

CSS references: [intrinsic sizes](https://www.w3.org/TR/css-sizing-3/#intrinsic-sizes)
and [hyphenation](https://www.w3.org/TR/css-text-3/#hyphenation).

## Atomic input and source coordinates

U+FFFC is Unicode's **OBJECT REPLACEMENT CHARACTER**, intended to mark the
position of an embedded object in a text stream, with its object/formatting data
held separately. This standard meaning directly matches our proposed atomic
anchors; the character itself supplies neither dimensions nor object identity.
It is distinct from U+FFFD REPLACEMENT CHARACTER (`�`), used for unknown or
unrepresentable characters. See the [Unicode Standard, Chapter 23](https://www.unicode.org/versions/Unicode17.0.0/UnicodeStandard-17.0.pdf).

Recommended initial representation:

```text
Logical items: Text("Hello ") -- Atomic(id=7) -- Text(" world")
Cord:          "Hello \uFFFC world"
Attachment:    paragraph anchor -> item 7
Adapter:       item 7 -> CSS BoxID
```

The attachment, not the character alone, establishes object semantics. A literal
U+FFFC without an attachment must have an explicit documented interpretation.
Each attached object occupies one scalar/three UTF-8 bytes in the prepared
paragraph, even when its dimensions are zero. Adjacent objects have distinct
anchors and identities.

The marker participates in paragraph bidi analysis but is not text requiring a
font glyph: it bypasses shaping and any existing upstream coverage checks.
Object boundaries split text shaping/itemization; no glyph cluster
may absorb an attached marker. Preserve paragraph byte/scalar coordinates and
feature-domain mappings when omitting markers from shaper input.

The paragraph source is the prepared stream, including markers. It is not the
original HTML. Extend adapter provenance with an object association rather than
fabricating text-node ranges containing the object's descendants. Preserve the
existing meaning of frozen source ranges as ranges in the supplied paragraph.

Inline-block interiors are bidi-independent from surrounding text; the outer
item participates neutrally. See [atomic-inline bidi treatment](https://www.w3.org/TR/css-writing-modes-3/#bidi-atomic-inlines).

Zero-byte attachments are less suitable initially: they require ordering multiple
items at one offset, explicit bidi participation, and new line/source mapping.
Current [generated-material placement](../generated_placement.go) only accepts
the validated source-less paragraph trailer.

### Separate intrinsic, horizontal, and vertical data

| Operation | Atomic data |
|---|---|
| Preparation | Stable ID and anchor; supported resolved boundary policy |
| Intrinsic measurement | Minimum/maximum inline contribution |
| Final horizontal layout | One definite advance |
| Paragraph assembly | Border dimensions, margins, alignment/baseline data |

Once prepared by the caller for final layout, an atom is fixed-width:
`MinW == W == MaxW`. Its intrinsic range is not permission for the line breaker
to resize it.

Keep attachments outside font/style payloads. Put the minimal shared input
vocabulary in a dependency-light contract usable by both itemization and encoding;
stage 1 selects the existing `khipu/contracts` package. A convenience builder may later belong in
`cords/styled`, but changing `CordEx`, chunks, or tree summaries is not required.

## DOM, styled document, and normalized group extraction

The DOM supplies provenance; the styled document supplies effective properties;
the normalized layout tree determines formatting participation and paragraph
groups. One DOM element may generate several boxes/groups.

The adapter walks the supplied normalized group in order. It traverses ordinary
inline descendants, reads referenced text and effective typography, and emits one
anchor at each atomic boundary while skipping that boundary's entire subtree.
The interior is handled by independently scheduled groups.

[Tytui extraction](../../tytui/htmlinput/extraction.go) provides useful inheritance,
whitespace, bidi and provenance behavior, but its selected-subtree eligibility
rules cannot define document paragraph boundaries.

Normalize whitespace across the complete ordered text/object sequence. Separate
normalization of each text fragment would erase meaningful spaces beside objects.
Preserve inline structure associations needed for edges, empty inline elements,
and later fragments. Initially restrict ordinary inline margins/padding/borders
to an explicit supported subset; reject unsupported contributions instead of
silently omitting them.

## Required changes inside khipu

1. Add a direct non-glyph candidate path. The current
   [candidate accumulator](../candidate_knots.go) is driven by shaped clusters;
   objects must enter the ordered sequence directly, including object-only input.
2. Add an explicit atomic knot kind and private identity references. Do not
   disguise atoms as textboxes, whose validation requires shaping records and
   whose classification participates in discretionary discovery.
3. Treat atoms as content for line-edge trimming. Both breakers and
   [logical materialization](../logical_lines.go) currently recognize only
   `KTTextBox` as content protecting adjacent spaces. A custom retained-neutral
   knot would not suffice. Share the content classification across these paths.
4. Carry atomic identity through materialization, visual permutation, width
   allocation, and freezing. Expose typed frozen atomic access; `CopyText` remains
   false for atoms. Zero-sized atoms must survive exactly once.
5. Apply resolved object boundary policy before knot grouping. Both sides may
   introduce opportunities; do not mutate penalties after text has already been
   grouped across a required boundary.
6. Keep atoms indivisible and ineligible for hyphenation. Preserve source coverage,
   cancellation, validation, and atomic publication behavior.

Raw UAX classification is not the complete CSS boundary policy. CSS specifies
particular behavior around atomic items and NBSP, word joiners, and related
classes. The adapter resolves supported CSS behavior into neutral input policy;
khipu need not parse CSS strings. See [line-breaking details](https://www.w3.org/TR/css-text-3/#line-break-details).

## Paragraph assembly and CSS adaptation

`typography/paragraph` must include atomic vertical contributions and retain
placements. Merely exposing horizontal object knots cannot satisfy the layout
interface. Existing assembly either ignores non-text material for font metrics
or treats it as source-styled space; atoms need a separate branch.

For the initial horizontal, bottom-margin-edge alignment policy:

```text
advance = borderWidth + marginLeft + marginRight
borderX = visualPenX + marginLeft
borderY = lineBaseline - marginBottom - borderHeight
```

Include the object's margin-box contribution in line extents. Preserve signed
margins and test overflow and aggregate geometry limits. Core already supplies
border dimensions including padding/borders: do not add these again. Return
border origins after margin/alignment effects; core must not add margins again.

`PreparedAtomic` currently has W/H only. This supports the bounded policy already
proposed by the CSS integration notes, but full inline-block baseline behavior
needs an internal baseline contribution. Define that later through core preparation
or explicit retained integration data, rather than guessing from height.
See [CSS baseline rules](https://www.w3.org/TR/CSS22/visudet.html#propdef-vertical-align).

Additional mismatches:

- CSS permits zero content width; `paragraph.Run` currently requires positive
  width. Define zero-width behavior for empty input and overflowing indivisibles.
- `paragraph.Run` currently chooses breaker-specific layout defaults internally.
  Expose resolved spacing/alignment control so changing breakers does not
  implicitly change the intended paragraph policy.
- Unit conversion belongs at the adapter: CSS px -> DU, and fractional big points
  -> CSS px. `khipu/dimen.PX` is not the CSS pixel conversion. Check range and
  rounding without narrowing retained glyph geometry to float32.

Retain concrete paragraph results by group BoxID with OwnerID. Translate stable
atomic IDs to exactly one placement per prepared BoxID, including zero-sized
items. Return extent plus placements; lines and glyphs remain in typed retained
results. Publish those results, source/paint associations and geometry only after
the entire document succeeds. Discard invocation-local work on failure.

## Proposed public API capabilities

These are the capability overview; concrete proposed signatures and their
recommended contracts appear in the stage 1 section below.

| Capability | Contract direction |
|---|---|
| Mixed paragraph input | Styled paragraph plus validated object anchors and resolved boundary policy |
| Prepared paragraph | Opaque reusable shaping, knot templates, provenance, and unresolved atomic slots |
| Intrinsic query | Prepared material plus atomic bounds -> content min/max widths |
| Bind layout material | Prepared material plus definite atomic advances -> owned working Khipu |
| Frozen atomic access | Frozen knot -> stable object identity |
| Paragraph assembly | Frozen material plus atomic geometry -> vlist and placements |

Keep `EncodeParagraph` as a text-only convenience. Reuse existing run preparation,
candidate construction and shaping storage rather than build another shaper path.

Preparation reuse must not imply that today's `Khipu` is immutable: breakers
replace selected discretionaries. Layout attempts need independent mutable work
and selections; immutable shaping records may be shared. Specify ownership of
lazy candidate caches explicitly. Start with invocation-local reuse; defer global
caching, invalidation and speculative-layout acceptance protocols.

Measurement may retain private preparation, but must not publish a final paragraph
result, placement, or geometry. Final layout binds the selected atomic sizes and
freezes its own decisions before publication.

## Staged implementation plan

### 1. Freeze mixed-input and ownership contracts

Specify anchor/ID validation, byte/scalar domains, separate measurement and final
dimensions, supported boundary policy, errors, and bounded vertical alignment.
Resolve the API/package questions below and reconcile stale sibling documentation.

Acceptance: examples cover text-only, object-only, adjacent objects, nested
atomics, and source mapping without requiring DOM types in the paragraph engine.

### 2. Encode fixed-size atomics through freezing

**Implemented.** `contracts.AtomicID`/`AtomicAnchor`, `ParagraphInput`,
`PrepareOptions`, `PrepareParagraph`, `AtomicAdvance`, `BindParagraph`,
`KTAtomic = 3`, and `FrozenKhipu.Atomic` provide the core mixed-input path.
Preparation retains private unbound knot templates and immutable shaping records;
binding creates independent working planes, source associations, discretionary
caches and selected decisions. Shared shaping slices are capacity-limited so
appending replacements cannot overwrite another binding's records.

Attached markers stay in Unicode/bidi analysis and source coordinates, but are
removed from shaping runs before style resolution and feature intersection.
Atoms enter candidate accumulation directly. Both breakers and freezing use
the same content classification, preserving zero-width atoms and spaces between
content items. Frozen identity follows physical bidi order; CopyText is absent
for atoms. Unattached U+FFFC and the legacy EncodeParagraph path remain text.

Boundary overrides and preparation-time hyphenation modes are enforced. Signed
advances retain widened intermediate sums in breaker bookkeeping; negative
completed line geometry returns ErrUnsupportedPolicy during freezing. Positive
overfull output within the finite domain retains existing diagnostics.

Regression coverage: `paragraph_input_test.go`, `atomic_integration_test.go`,
and `linebreak/wss_test.go`, alongside the existing text-only suite. Coverage
includes attachment validation, marker/style bypass, feature domains, original
byte/scalar coordinates, binding isolation, bidi, zero width, multiline boundaries,
overfull atoms, signed-prefix recovery, and frozen snapshot stability.

The implementation scope is limited to the core. Typography itemization,
CSS geometry/placement adapters, and intrinsic measurement remain stages 3–5;
no changes to cords or sibling modules are required for this stage.

Implement object-aware preparation, direct candidate insertion, content
classification, and frozen identity. Audit both breakers and every intermediate
representation; preserve text-only behavior.

Acceptance: both breakers retain each object exactly once, never shape or
hyphenate it, preserve appropriate surrounding spaces, and handle mixed bidi,
zero-sized and overfull objects. Invalid attachments fail without partial output.

### 3. Complete fixed-size paragraph/CSS integration

**Stages 3A–3F implemented.** The source review finds that khipu's
stage 2 API and CSS core's current callback contracts cover the required horizontal
boundary. Most implementation belongs in typography and the document integration
layer. Deliver six reviewable increments, in the order below. Source inspection
is not evidence that the combined pipeline already builds or passes these tests.

#### Module boundaries

| Module/package | Stage 3 responsibility |
|---|---|
| `khipu` | Keep the stage 2 mixed-input, binding and freezing APIs; add integration regressions and fix demonstrated core defects. No API redesign is planned. |
| `typography` | Atomic-aware projection/itemization and preservation of original feature domains. Exclude markers from existing checks; add no coverage validation. Relocating coverage checking out of the itemizer is future work. No DOM, CSS box-tree or concrete backend dependency in the generic package. |
| `typography/paragraph` | Prepared orchestration, atomic geometry validation, vertical assembly and detached placements. No CSS parsing, source locators, paint or box-tree traversal. |
| `tytui/documentlayout` | Styled-document-to-render-input adaptation, normalized-group traversal, CSS/paragraph bridge, unit conversion, invocation-local retention and document publication. |
| `tytui/htmlinput` | Reuse or factor computed-style, normalization and provenance-emission helpers; retain the existing single-paragraph extraction behavior. |
| `css-box-layout` | Continue owning normalization, used dimensions, nested atomic preparation and block positioning. Verify the real adapter against existing interfaces; reconcile stale documentation. |
| `cords` | No planned changes: styled text, U+FFFC storage and zero-byte bidi markup suffice. |
| `w3c-dom` | Reuse existing CSS services. Any demonstrated general cascade/computation defect belongs here, without typography dependencies. |

Use the concrete signatures in the stage 1 proposal below. The new tytui package
is application integration initially, not a new general-purpose layout library.
Do not recreate the removed tytui paragraph engine or move CSS orchestration into
khipu. Keep resource loading, discovery and concrete font adaptation at their
existing boundaries.

#### Initial supported policies

This subsection records the stage 3 boundary. Stage 4D below now supplies an
intrinsic measurer and permits bounded auto-width inline-blocks; the other
restrictions recorded here remain in force.

The document bridge initially supports nested inline blocks with definite,
nonnegative fixed-pixel widths. CSS core determines their completed border sizes
and lays out their interiors before the enclosing group's callback. Pass no
intrinsic measurer to fixed-width flow; reject auto-width atomics rather than
invent estimates. Intrinsic measurement and auto sizing remain stages 4 and 5.

Use `CSSLineHeight` with `normal`, `NoHyphenation`, supported horizontal text/bidi
processing, and `AlignBottomMarginEdge`. This alignment is the agreed bounded
policy, not a claim of full CSS inline-block baseline behavior. Resolve the
paragraph strut from the owning paragraph's typography, including for object-only
groups; do not choose it from the first glyph or a fallback run.

Resolve horizontal skips explicitly and pass the same values to either breaker
and freezing. Use zero natural alignment skips initially; do not let changing
breakers silently choose a different paragraph alignment policy. General
`text-align` mapping and indentation are not implied by this bridge. The generic
paragraph API still preserves existing profiles when `Horizontal == nil`.

Zero available width and zero-sized objects are valid. Positive overfull lines
remain successful results with diagnostics. Signed margins/advances retain the
stage 1 finite-domain and negative-completed-line restrictions. Atomic border
sizes already contain padding and borders; add only margins to their contribution.

The extraction subset uses normal whitespace collapsing. Reject unsupported
authored policies explicitly, including auto-width atomics, nonzero ordinary-inline
edges, metric-bearing empty inline elements, preserved whitespace/explicit breaks,
and richer vertical alignment. Authored line-height values beyond `normal` require
the separate DOM computation work before acceptance. This bridge restriction does
not remove the paragraph engine's existing typed factor/length support. Preserve
legacy text-only proof and replay behavior; no document replay format is added.

#### 3A. Make typography itemization atomic-aware

Implement the agreed `Resolver.ItemizeInline(ctx, text, spans, atomics, policy)`
entry point while retaining existing text-only itemization APIs. **Implemented
in typography:** [inline.go](../../typography/inline.go) owns anchor validation,
atomic-aware projection and private feature windows;
[itemize.go](../../typography/itemize.go) shares the existing text algorithm;
[resolver.go](../../typography/resolver.go) resolves those windows for shaping.

Coverage assurance belongs to the caller, as specified under ownership above.
This increment adds no coverage preflight, missing-glyph validation or fallback
requirement. The itemizer's existing coverage functionality will be moved elsewhere
in a future task; making markers bypass that current code is not an endorsement
of keeping coverage validation in itemization.

Begin in projection/coalescing: [ProjectStyles](../../typography/bypass.go) resolves
styles before [itemization](../../typography/itemize.go). Skipping only the existing
coverage checks would still resolve object-only spans. Attached markers must bypass
style/font resolution and those checks, separate text grapheme/script/fallback
processing, and produce no text `Binding`. Preserve marker bytes and original
paragraph scalar positions. Unattached U+FFFC remains ordinary text.

Do not independently itemize text substrings on either side of an atom and reset
their feature coordinates. Retain private original-domain/window metadata and
clip/rebase features deliberately for emitted text fragments. Keep marker source,
paint and bidi metadata outside text font bindings.

Acceptance: object-only styles need no font mapping or placeholder glyph; literal
U+FFFC remains text under the caller's font-coverage responsibility; no new coverage
pass is introduced in preparation or layout. Combining characters, existing
fallback behavior, paint boundaries and range-limited features remain correct
before and after atoms. Existing text-only itemization and legacy paint-policy
regressions remain unchanged pending the separate coverage-relocation task.

Implementation details and boundaries:

- Anchor IDs must be unique and nonzero, offsets strictly increasing, and each
  offset must select one complete U+FFFC. Invalid UTF-8 is rejected before cord
  construction. Original bytes remain intact; attached markers are unstyled gaps
  without text bindings. Return no partial itemization on error.
- Apply the existing paint policy to text-bearing source spans before slicing
  around atoms. A separate object-only span is a domain barrier: it is skipped
  without resolving its style, not used to coalesce neighboring text. Structural
  span validation still applies. With no anchors, use the existing text-only path.
- Record each projected domain's original byte identity and each emitted text
  window's scalar coordinates, counting marker scalars. `ResolveStyle` intersects
  late-supplied features with that window and rebases them locally. Empty overlaps
  disappear; feature order, values and disabled entries survive. Script/fallback
  splits retain the same original domain. Khipu can then apply its existing bidi
  intersection without resetting the origin. Retain the returned style values.
- Request slices are neither modified nor retained. Source/paint/bidi association
  stays with the caller, which passes the same anchors to khipu preparation.
  No new public metadata record or dependency on a concrete font backend is needed.

Regression evidence: [generic itemizer tests](../../typography/inline_test.go)
cover object-only style bypass, literal markers, adjacent objects, combining
characters, soft hyphens, fallback/script barriers, both paint policies, nonzero
feature origins, invalid anchors/UTF-8, cancellation and caller ownership. The
horizontal integration tests preserve multibyte/LTR/RTL source coordinates through
both breakers and freezing. [Real-font tests](../../typography/adapter/opentype/inline_test.go)
apply a range-limited ligature after an atom and check exact glyphs and original
cluster positions with both breakers and paint policies. Typography and tytui
ordinary/race suites, builds and vet checks verify compatibility, including the
generic dependency audit and existing replay fixtures.

Stage 3A adds no coverage validation and does not relocate existing checks. It
requires no production changes in khipu, cords, CSS core or tytui. Paragraph
preparation/vertical assembly are implemented in stages 3B/3C below. Extraction is
implemented in stage 3D; the CSS callback and publication are implemented in stage 3E.

#### 3B. Add prepared paragraph orchestration

**Implemented together with stage 3C**, so mixed layouts return complete frozen
content and vertical geometry. [prepared.go](../../typography/paragraph/prepared.go)
owns preparation and reusable layout; [layout.go](../../typography/paragraph/layout.go)
shares breaking/freezing and result assembly with the existing `Run` path.

Extend [paragraph.Input and orchestration](../../typography/paragraph/layout.go)
with the agreed `Input.Atomics`, `Prepare`, `Prepared.Layout`, `AtomicBox`,
`AtomicMargins`, `HorizontalLayout` and `LayoutRequest` records and operations.

Preparation constructs bidi information once, consumes markup callbacks, and
retains khipu preparation plus stable source/configuration for assembly. Each
layout validates atomic geometry, derives advances, binds fresh working material,
breaks, freezes and assembles. Do not accept independent advances that could
contradict the supplied border geometry. Clone request/configuration slices as
needed; retain external services only under their stable-lifetime contracts.

Allow width zero. Explicit horizontal skips replace the profile completely,
including zero values, and remain identical through breaking and freezing.
Preserve `Run` as the text-only convenience path and retain its historical error
stages. Reject nonempty anchors at `Run`'s input boundary because it supplies no
atomic geometry. Add `StagePrepare` and `StageBind` with the stage 1 cause-preserving
error mapping; measurement and `StageMeasure` implementation remain stage 4.

Acceptance: repeated layouts with different widths/object dimensions reuse
preparation without sharing mutable decisions; no re-itemization or loss of
private styles; cancellation/failure returns no partial result, leaves preparation
reusable, and cannot change an earlier frozen result or vlist. Owner operations
remain serialized rather than gaining an implicit concurrent-use promise.

Implementation retains persistent styled text and resolved bidi segments, never
the markup callback. It copies anchor/boundary slices, configuration envelopes
and feature defaults; line-layout ranges and atomic geometry are detached per
layout. New atomic advances are always derived from border dimensions/margins.
All layout attempts use fresh bound material. `Run` still uses its original
encoding/hyphenation path and `StageEncode`, but shares subsequent orchestration;
it now accepts width zero and rejects atomic input at `StageInput`. New
`StagePrepare`/`StageBind` errors preserve causes, including argument, backend and
cancellation errors. `MeasureIntrinsic`/`StageMeasure` remain unimplemented.

Acceptance evidence: [prepared tests](../../typography/paragraph/prepared_test.go)
exercise repeated widths and atomic dimensions, default and explicit skips,
consumed markup, stable private styles, copied requests/configuration, detached
diagnostics/vlists and recovery after binding, breaking, freezing and assembly
failures. [OpenType integration](../../typography/adapter/opentype/prepared_test.go)
checks range-limited ligatures and original scalar anchors through both breakers
and repeated layouts without reshaping base text. Existing text-only, hyphenation
and replay cases remain compatible. Typography and tytui ordinary/race suites,
builds and vet checks pass; no tytui production adaptation is part of this stage.

#### 3C. Assemble atomic vertical geometry and placements

**Implemented.** [atomic.go](../../typography/paragraph/atomic.go) owns checked
border/margin geometry, advance derivation and physical placement traversal.
Both assembly policies now handle atomics explicitly, and `VList.AtomicPlacements`
returns detached border origins.

Geometry-aware assembly behind `Prepared.Layout` provides the agreed detached
`VList.AtomicPlacements()` accessor. Before this stage,
[compatibility assembly](../../typography/paragraph/assemble.go) ignored objects,
while [CSS line-height assembly](../../typography/paragraph/lineheight.go) treated
non-glyph material as source-styled space. Both now have distinct atomic branches
and exclude marker-only ranges from eager source-metric resolution.

For bottom-margin-edge alignment, use:

```text
advance       = borderWidth + marginLeft + marginRight
aboveBaseline = marginTop + borderHeight + marginBottom
belowBaseline = 0

borderX = visualPenX + marginLeft
borderY = lineBaseline - marginBottom - borderHeight
```

Combine signed atomic contributions with text extents and the paragraph strut
before validating final geometry; do not apply text half-leading to markers or
clamp each object's contribution separately. Walk all frozen knots, including
glue, to advance the visual pen from the line origin plus its left skip. Do not
reorder physical knots a second time.

Placements are ordered by line and frozen-knot traversal, not numerical X. Zero
advances can share X and negative advances can make X nonmonotonic. Preserve
existing text-only compatibility calculations. The old frozen-only `Assemble`
must reject atom-containing input without supplied geometry rather than silently
ignore it; a frozen advance alone cannot recover border height or margins.

Acceptance: independent numeric line-height, baseline and border-origin oracles;
object-only and mixed lines; negative margins, zero sizes, RTL and coincident
positions; margins/padding/borders counted once; detached placement slices and
stable old vlists after subsequent layouts.

The CSS policy retains its explicit paragraph strut and combines signed atomic
extents before validating the completed line. Marker-only source ranges bypass
eager font-metric resolution; per-range text line-height values do not add leading
to atomics. The compatibility policy preserves existing text metrics and fallback
gap, then unites atomic extents with those values. Neither policy infers an
internal inline-block baseline. Frozen-only `Assemble` now returns
`ErrUnsupportedPolicy` for atomic content without border geometry.

Owning tests provide independent baseline/height/border-origin expectations for
object-only and mixed lines, multiple lines, signed extents, margins, zero-sized
objects, zero target width, coincident positions, RTL paragraph fill and
nonmonotonic X. They check finite bounds after signed accumulation, overflow of
published border origins or paragraph height, absent marker font resolution,
detached placement slices, cancellation and preservation of prior results.
The CSS callback, CSS unit conversion and intrinsic queries remain deferred.

#### 3D. Extract normalized groups with source correspondence

**Implemented in [tytui/documentlayout](../../tytui/documentlayout/README.md).**
`Build(ctx, document, selectedNodeID)` snapshots source/computed typography,
constructs typed CSS input and delegates normalization to `BuildLayoutTree`.
`Document.Groups()` returns detached group/owner identities with inherited
paragraph contexts, and `Document.Extract(ctx, groupID)` returns a detached
`InlineGroup`. `Document.Root()` exposes borrowed immutable structural input for
the future flow callback. This stage does not invoke `FlowLayout`.

Build typed `RenderNode` input from the styled document, with retained source and
computed-style lookup, then let CSS core normalize it. Extract each resulting
`BoxAnonymousInline` group. The current [subtree extractor](../../tytui/htmlinput/extraction.go)
rejects nested blocks and inline blocks, so calling it once per group is not a
valid shortcut. Reuse its emission/provenance logic where applicable, not its
single-selection eligibility rules.

Walk group children in source order. Read `BoxText.TextRef` through the retained
source table. At `BoxInlineBlock`, emit one marker and skip its entire subtree;
its interior belongs to independent groups. Assign occurrence-local IDs and
retain an explicit `AtomicID -> BoxID` association. Anchor ordering follows emitted
byte positions, never BoxID order or dimension-table iteration.

Collapse whitespace across text nodes while treating atoms as content. Derive
anchors, styles, paint, line-height ranges and bidi ranges from the same emitted
byte stream. Store atomic provenance separately as element/box identity, not as
invented DOM text-node bytes. Anonymous owners need an explicit inherited
paragraph-style context. Original source locators remain distinct from normalized
box identities, including when one source element produces split boxes.

Acceptance: split/hoisted inline structure, anonymous groups, nested atomics,
whitespace on either side of objects, entities, multibyte text and isolates;
no duplicate extraction of atomic interiors; exact transformed text/source/paint
correspondence; the existing single-paragraph extraction path remains compatible.

The concrete extraction envelope contains `Text`, document `Styles`, separate
`Paint`, normal `LineLayout` ranges, `Isolates`, ordered `Anchors`, `TextSources`
and `AtomicSources`. All output ranges refer to the same processed UTF-8 stream.
Text source records retain decoded DOM node byte ranges, original locators and
normalized text BoxIDs. Atomic records separately retain occurrence ID, BoxID,
element locator and marker range. A literal U+FFFC text character is not attached.
IDs are group-local and assigned by emission order, not BoxID order.

Typography resolution is shared through the new detached
`htmlinput.Document.ResolvedStyles` helper, without changing the existing
single-paragraph eligibility rules. Whitespace transformation reuses
`paragraphtext.Normalize`, and isolate markup reuses htmlinput's implementation.
The extractor retains first-source-unit ownership of collapsed space for style,
paint and bidi scopes. Source/style tables are detached from the original DOM;
published CSS tree nodes must remain read-only. Failure/cancellation publishes no
partial document or group; synchronous collaborators retain their existing limits.

Initial box-style adaptation supports definite px atomic widths, px box edges
and auto block widths/margins; other relative box lengths remain unsupported.
Font sizing retains htmlinput's existing absolute/relative-unit support. Reject
non-normal line-height, preserved whitespace, explicit breaks, hidden/replaced
content, unsupported display modes, positioned/floating flow, height/min/max
constraints, nonzero ordinary-inline edges and metric-bearing empty inlines.
Empty inline wrappers with the strut's font request may disappear; differing
font requests require the deferred empty-inline metric model. Coincident nested
isolate boundaries retain the existing explicit rejection. Border longhands and
supported four-side shorthands work; combined `border` parsing remains outside
the current DOM subset. No new glyph coverage validation is introduced.

Acceptance evidence: [extraction tests](../../tytui/documentlayout/extract_test.go)
cover split/hoisted structure, source-vs-box identities, anonymous owner contexts,
nested/adjacent/empty atomics, whitespace, entities, multibyte and literal markers,
isolates, exact transformed provenance, style/paint/layout correspondence and
detached ownership after source removal. Text-only results are compared with the
legacy extractor. [Paragraph composition](../../tytui/documentlayout/paragraph_test.go)
feeds groups through `ItemizeInline` and prepared layout under both breakers,
checking original coordinates and owner-derived struts for object-only groups.
Tytui's ordinary/race/build/vet checks include existing TUI, pipeline and replay
regressions. CSS callback geometry/conversion and publication are covered by stage 3E below.

#### 3E. Implement the CSS callback and transactional publication

**Implemented in `tytui/documentlayout`.** Implements the existing [InlineLayouter contract](../../css-box-layout/layout/interfaces.go)
in an invocation-local adapter. Obtain completed border dimensions from
`PreparedAtomicsView` and margins from `UsedValuesView`, convert to `AtomicBox`,
then prepare/layout the group. Return overall extent and ID-mapped border placements.
CSS core retains responsibility for nested preparation and scheduling; the
paragraph callback must not recurse into atomic interiors or size them again.

Apply the stage 1 conversions and finite checks:

```text
CSS px -> DU:       round(px * 49152)
big points -> px:   points * (4/3)
```

Convert primitive dimensions once and derive advances afterward. Retain paragraph
geometry as float64 big points; narrow only CSS-facing summaries to float32 after
validation. Do not overwrite CSS core's prepared border dimensions with quantized
round-trip values. Conversion tests use the stated per-primitive error budget.

Callback placements are relative to the owner's content origin. Do not add owner
padding/borders or ancestor offsets there. Document rendering composes those
transforms once; it does not stack paragraph lines again.

Retain concrete paragraph results, source maps and paint by group BoxID with an
explicit OwnerID. Publish the table and CSS geometry together only after the whole
`FlowLayout` succeeds. Capture context in the invocation adapter and check it at
callback/publication boundaries; existing synchronous collaborator cancellation
limits remain. Reject duplicate group layout callbacks initially rather than
silently overwrite results; speculative acceptance is deferred.

Acceptance: every prepared BoxID is placed exactly once, including zero-sized
objects; correct anonymous-owner and ancestor transforms; no double edges/margins;
conversion limits and rounding; late failure/cancellation preserves the previous
complete document snapshot and exposes no partially retained new document.

The public entry point is:

```go
type LayoutRequest struct {
    Width   float32 // containing-block width, CSS pixels; zero is valid
    Breaker paragraph.Breaker
}
type Config struct {
    Resolver      *typography.Resolver
    NewShaper     func() (contracts.StreamingShaper, error)
    BufferOptions contracts.ShapeBufferOptions
}
func (d *Document) Layout(context.Context, LayoutRequest, Config) (*Result, error)

type GroupResult struct {
    Input     InlineGroup
    Paragraph paragraph.Result
}
func (r *Result) Document() *Document
func (r *Result) Geometry() box.LayoutGeometryTable
func (r *Result) Groups() []Group
func (r *Result) Group(box.BoxID) (GroupResult, bool)
```

The caller supplies an initialized resolver and fresh exclusive shaper factory;
there is no backend/discovery or new coverage policy. Calls sharing a resolver
are serialized; font/scale services remain stable for retained results. Each
invocation resolves used values, calls CSS `FlowLayout` with no intrinsic measurer,
and keeps callback results private until success. `Document` has no mutable
current-result slot: callers replace their snapshot only on nil error. Failure
returns nil and leaves every earlier result intact, including after resize.

`Geometry`, `Groups` and `Group` copy mutable maps/envelopes/slices, including
paragraph diagnostics. The normalized document, frozen paragraphs and vlists are
shared read-only. `GroupResult.Input` retains explicit `OwnerID`, source/paint
and AtomicID-to-BoxID correspondence. Callback order is nested-first; public group
order remains source order. Repeated callbacks fail with `ErrUnsupportedPolicy`.

The callback supplies normal CSS line height, the inherited owner strut, no
hyphenation, bottom-margin-edge atomic alignment and explicit zero side skips.
It converts prepared border sizes and used margins independently, without sizing
interiors or modifying CSS border dimensions. Empty atomic interiors currently
reserve a normal paragraph strut; zero-sized prepared boxes are still valid at
the callback boundary.

Acceptance evidence: [flow tests](../../tytui/documentlayout/layout_test.go)
exercise both breakers, nested scheduling, owner/ancestor transforms, exact edge
accounting, signed margins, zero available width, overfull objects, source/paint
retention, detached results and resize. [Boundary tests](../../tytui/documentlayout/geometry_test.go)
cover zero-sized prepared items, duplicate callbacks, malformed views, independent
primitive rounding, float32 summary budgets, finite-domain limits and cancellation
at publication. Late backend failure/cancellation returns no partial document and
preserves the previous complete snapshot. Real-font proofs are covered by stage 3F below.
Tytui's full ordinary/race suites, build and vet pass with the new callback and
ownership tests alongside the existing TUI, pipeline and replay regressions.

#### 3F. Verify nested fixed-size integration and align documentation

**Implemented.** The original acceptance criteria follow, then the concrete API
and evidence. Intrinsic measurement, auto-sized atomics, TUI document features and
document replay remain outside this completed fixed-width increment.

Start with typed synthetic fixtures, then an HTML/CSS document fixture, then
real-font integration tests with caller-supplied suitable fonts. Exercise both
breakers with text-object-text, adjacent and object-only groups, nested fixed-width
inline blocks, anonymous groups around
nested blocks, zero available width, overfull objects, signed margins and bidi.
Include resize, repeated successful invocations and failures after earlier groups
have already completed. Verify old snapshots after source-input mutation/removal
where their retained-source contract permits it.

Produce a diagnostic document proof from retained geometry and glyphs without
remeasurement. Test coordinates and source associations independently of the
renderer rather than relying on screenshots alone. TUI features and document
replay are outside this stage. Preserve static paragraph replay fixtures and
legacy text-only rendering evidence.

Run focused owning tests followed by ordinary/race suites, build and vet in
typography and tytui, plus relevant khipu and CSS core regressions. Keep generic
package dependency audits and backend-service tests. Update sibling documentation
that still describes `AtomicSizer` or line-box callback results, following CSS
core's existing roadmap rather than creating a separate decision log.

Stage 3's completed nested fixed-size document path demonstrates
independently checked line heights and border origins, exact-once edge accounting,
owner-relative placement, retained source/paint and transactional result lifetime.
No intrinsic-measurement stub, auto-sizing estimate or broader CSS support claim
is part of that acceptance.

The reusable walker now lives in `typography/render`:

```go
type ParagraphInput struct {
    Frozen      *khipu.FrozenKhipu
    VList       *paragraph.VList
    Text        string
    Paint       []typography.PaintSpan
    PaintPolicy typography.PaintPolicy
}
type Services struct {
    Backend  Backend
    Identity typography.IdentityProvider
}
func BuildParagraphProof(context.Context, ParagraphInput, Services) (*ParagraphProof, error)
```

It consumes retained placements and glyphs, applies visual order once per detached
directional fragment and preserves original processed UTF-8 source/paint anchors.
It never loads fonts, queries line metrics, reshapes, rebreaks or restacks lines.
The existing `pipeline.BuildGlyphProof` delegates to it while preserving its public
proof records, SVG emission and static replay evidence. The generic package has no
CSS/DOM/backend imports; font identity/outline services are explicitly injected.

The document-specific proof API in `tytui/documentlayout` is:

```go
func BuildProof(context.Context, *Result, render.Services) (*Proof, error)
func WriteSVG(context.Context, io.Writer, *Result, render.Services) error
```

`Proof` contains detached absolute float64 big-point boxes, lines, glyphs and atomic
origins. Each paragraph contributes its already placed geometry at its OwnerID's
absolute content origin. CSS RelativeTo chains are composed once; no owner edges
or line offsets are added twice. Core border dimensions remain unchanged. Glyphs
retain group/owner, line/knot, cluster, original SourceByte, font hash, paint and
covering TextSpan records. Atomic records retain their separate ID/BoxID/element
source correspondence, including precise origins before CSS float32 narrowing.

Proof errors/cancellation expose no partial object; SVG generation completes in
memory before writing. Writer errors can still leave partial destination bytes.
The SVG is a diagnostic of boxes, baselines and outlines, not a full CSS painting
or stacking-context model. It adds neither font embedding nor a replay format.

Acceptance evidence:

- [Typed integration](../../tytui/documentlayout/typed_integration_test.go) starts
  with explicit normalized-input styles/source identities, independent of DOM/CSS
  parsing, and checks nested struts, dimensions, markers and content parents.
- [Document proof tests](../../tytui/documentlayout/proof_test.go) check exact
  synthetic ancestor and multiline/bidi coordinates under both breakers, real
  HTML/CSS nested/adjacent/object-only layout, resize/repeated output, source DOM
  removal, retained paint and font associations, and proof/writer failures.
  Latin ligature and pointed Hebrew glyph IDs, clusters and offsets are compared
  against existing independent HarfBuzz captures, with their font hashes.
- Stage 3E tests continue to cover zero available/prepared sizes, positive overfull
  objects, signed margins, unit-conversion budgets, duplicate callbacks and late
  layout failure/cancellation without changing older complete snapshots.
- [Shared walker tests](../../typography/render/paragraph_test.go) deny metric
  access after layout and verify byte anchors, physical glyph positions, paint,
  detached ownership and late outline failure/cancellation.
- [Fixture and reproduction commands](../../tytui/documentlayout/testdata/README.md)
  accompany a [real-font SVG](../../tytui/documentlayout/testdata/nested-firstfit.svg).
  The proof was rasterized and visually inspected; numeric expectations come
  from independent tests, not from the generated SVG.

Documentation in tytui's document exploration, paragraph-engine plan and integration
overview now describes `InlineRequest`/`InlineResult{Extent, Atomics}`, core-owned
nested preparation and typed transactional retention, following CSS core's existing
roadmap. Historical proposals are explicitly labeled. Glyph coverage remains the
caller's responsibility; existing itemizer checks are unchanged and their relocation
is still deferred. Stage 4 intrinsic measurement is not implemented by this work.

Validation: full ordinary/race suites, build and vet pass in typography and tytui,
including generic dependency audits, backend services and unchanged legacy replay
tests. Khipu and CSS core full ordinary suites also pass. The documented installed
proof command reproduces the sample SVG byte-for-byte.

### 4. Add intrinsic measurement and preparation reuse

**Stages 4A–4E implemented.** Ordinary and bounded explicit intrinsic measurement
are available in khipu and typography. Tytui has an invocation-local preparation
table and CSS measurement callback for bounded auto-width layout, retaining its
`NoHyphenation` profile. The stage 1
API and geometry contracts remain authoritative. Measurement is based on shaped
material, not estimates, breaker trials or rendering measurements.

Stages 2–3 already supply the preparation abstraction needed here.
[PreparedParagraph and BindParagraph](../paragraph_input.go) retain immutable
horizontal material and create fresh working bindings;
[paragraph.Prepared](../../typography/paragraph/prepared.go) owns the reusable
paragraph preparation. Do not introduce another encoding abstraction. Base shaping
is reusable. Stage 4E also shares complete explicit replacement batches between
measurement and bindings of an `ExplicitHyphenation` preparation. Automatic-mode
and legacy discovery retain their binding-local caches.

| Module/package | Existing foundation | Stage 4 change |
| --- | --- | --- |
| `khipu` | Prepared knot templates, shaping/source metadata, atomic slots and independent bindings | Intrinsic-width evaluation; later, reusable discretionary preparation |
| `typography/paragraph` | Owner-bound `Prepared` with reusable horizontal preparation and final layout | `MeasureIntrinsic` wrapper and `StageMeasure` |
| `tytui/documentlayout` | Normalized extraction, final CSS callback and transactional publication | Invocation-local prepared-group table, intrinsic callback and bounded auto-width integration |
| `css-box-layout` | Recursive intrinsic scheduling, shrink-to-fit selection and descendant resolution | Integration verification; no interface change is currently required |
| `cords` | Persistent text, style domains and bidi preparation | No anticipated changes |

#### 4A. Implement ordinary intrinsic measurement in khipu

**Implemented.** [Intrinsic measurement](../intrinsic.go) provides the stage 1
signatures, without target-width or breaker parameters:

```go
// Package khipu.
type IntrinsicWidths struct {
    MinContent dimen.DU
    MaxContent dimen.DU
}

type AtomicIntrinsic struct {
    ID                     contracts.AtomicID
    MinAdvance, MaxAdvance dimen.DU
}

type MeasureRequest struct {
    Atomics []AtomicIntrinsic
}

func (kq *Khipukamayuq) MeasureIntrinsic(
    ctx context.Context,
    prepared *PreparedParagraph,
    request MeasureRequest,
) (IntrinsicWidths, error)
```

Read prepared material directly. Do not bind fabricated atomic widths, invoke a
breaker, freeze material, reorder it visually or assemble vertical lines. Validate
context and owner identity. Each request must provide exactly one entry for every
attached ID, in any order; reject missing, duplicate, zero or foreign IDs, reversed
endpoints and out-of-domain dimensions. Consume/copy request data without retaining
caller-owned mutable slices. Atomic endpoints are signed **advances**, including
caller-resolved horizontal margins, not border dimensions or glue capacities.

The first supported profile is `NoHyphenation`:

- Max-content accumulates natural advances with optional wrapping suppressed.
- Min-content measures complete spans between supported wrapping opportunities.
  A span may cross multiple shaping/style knots; a knot boundary is not itself a
  wrapping opportunity. Use the prepared boundary policy, including atomic overrides.
- Use atomic `MinAdvance` for min-content and `MaxAdvance` for max-content.
  Ordinary text/glue contributes natural `W`, not `MinW` or `MaxW` elasticity.
- Apply layout's leading/trailing discardability semantics. Zero-width atomics
  remain content and must affect where edge trimming stops.
- Exclude generated paragraph fill entirely, including a configured nonzero
  natural fill width. Identify it through the validated preparation/trailer
  contract or explicit metadata, never a zero-width/zero-source-length heuristic.
- Alignment/side skips, stretch/shrink capacity, breaker scoring, emergency
  behavior and previously selected breaks do not contribute.
- Respect mandatory boundaries represented by supported prepared input. This
  does not add support for source mandatory breaks, blank lines or whitespace
  policies that preparation currently rejects.

Use checked `int64` accumulation, permitting signed intermediate contributions
and avoiding assumptions that prefix widths grow monotonically. Apply the agreed
normalization only after complete candidate-span evaluation:

```text
MinContent = max(0, rawMin)
MaxContent = max(MinContent, rawMax)
```

Require both published widths to be below `dimen.Fil`; reject overflow rather than
saturating. Do not clamp each negative atom or knot independently. The effective
max-content floor is consistent with the CSS sizing definitions, including negative
margins: [CSS Sizing terminology](https://www.w3.org/TR/css-sizing-3/#auto-box-sizes).
Successful measurement does not promise that every later chosen line can be frozen:
negative completed layout widths retain the existing `ErrUnsupportedPolicy` behavior.

Initial implementation should live with prepared material and source metadata in
khipu. Reuse or factor narrowly scoped edge-treatment logic where useful, with
cross-checks against both breakers and freezing; do not import a breaker into the
measurement layer or change breaker scoring to implement intrinsic sizing.

Acceptance:

- Hand-calculated text-only and mixed widths, including words split across
  multiple style/shaping knots, adjacent atoms and prohibited atomic boundaries.
- Leading/trailing versus internal whitespace, retained neutral material,
  zero-width atoms, empty/all-discardable input and object-only groups.
- Ordered and permuted atomic requests; all ID, range, ownership and context errors.
- Signed contributions, completed-span normalization, checked accumulation and
  values near the finite bound; no per-atom clamping.
- Nonzero natural paragraph fill and changes in glue elasticity do not change
  intrinsic widths. No target width, alignment or breaker dependency is introduced.
- Measurements before and after layouts with both breakers are identical for the
  same request. 4A initially rejected `ExplicitHyphenation`; 4E now supports
  its bounded explicit profile;
  `AutomaticHyphenation` measurement remains deferred beyond 4E. Neither silently
  returns ordinary-wrapping widths under an enabled hyphenation policy.

Implementation and verification:

- The evaluator scans immutable prepared knots and their source/atomic metadata
  directly. Candidate spans use the same first/last-content trimming rule as
  logical line materialization; accumulation remains signed and checked in
  `int64`. Only the validated generated trailer is excluded. Queries have linear
  cost in knots plus atomic occurrences and do not retain caller request storage.
- [Unit tests](../intrinsic_test.go) cover hand-calculated widths, style-spanning
  words, mandatory boundaries, signed ranges and normalization, finite-domain and
  accumulator overflow, retained neutral material, fill/elasticity exclusion,
  request/owner validation, cancellation and recovery, and unsupported modes.
  Service counters and preparation snapshots check that queries neither shape nor
  hyphenate nor mutate preparation. Neutral trimming is cross-checked against
  logical line materialization.
- [Integration tests](../intrinsic_integration_test.go) bind, break and freeze the
  same preparation with both breakers at wide and narrow widths. Measurements
  remain identical across layouts and working-plane mutations; changed atomic
  ranges are consumed afresh. Frozen geometry and copied glyphs remain unchanged,
  and natural frozen width agrees with max-content for the wide fixture.
- No typography or tytui API is added by 4A. Their measurement wrapper, preparation
  table and CSS callback remain stages 4B–4D. Discretionary measurement remains 4E.
  Glyph coverage remains caller-owned; no new validation or relocation of existing
  checks is included.
- Verified with khipu `go test ./... -count=1`, `go test -race ./... -count=1`,
  `go build ./...` and `go vet ./...`; downstream regressions passed with
  typography `go test ./paragraph/... -count=1` and tytui
  `go test ./documentlayout/... -count=1`. Broader cross-module stage 4 validation
  remains part of the later integration increments below.

#### 4B. Expose measurement through typography's prepared paragraph

**Implemented.** The [prepared-paragraph wrapper](../../typography/paragraph/measure.go)
delegates to khipu's ordinary intrinsic measurement. The API and error stage are:

```go
// Package paragraph.
func (p *Prepared) MeasureIntrinsic(
    ctx context.Context,
    request khipu.MeasureRequest,
) (khipu.IntrinsicWidths, error)

const StageMeasure Stage = "measure"
```

The wrapper delegates to the retained khipu owner/preparation. It requires no
atomic heights, alignment, line-height values, paragraph strut measurement or
rendering services. These remain final-layout concerns. Preserve `StageInput`
for invalid handles/nil contexts and cancellation already present at entry;
measurement-request validation/evaluation and unsupported modes use `StageMeasure`.
Preserve `errors.Is` for argument, policy, cancellation and backend causes.

Acceptance uses instrumented resolver, shaper and hyphenator services over:

```text
Prepare -> Measure -> Measure -> Layout(A) -> Layout(B) -> Measure
```

Base shaping occurs only during preparation. Changing atomic ranges changes the
answer without reshaping text; measuring ordinary material does not discover
discretionaries or query vertical metrics. Test both breakers, detached request
storage, failure/cancellation followed by retry, and earlier frozen/vlist results
remaining unchanged. Keep source coordinates and private feature domains intact.

Implementation and verification:

- `Prepared.MeasureIntrinsic` validates the handle and entry context, then calls
  the retained khipu owner/preparation directly. It introduces no duplicate width
  evaluator, geometry conversion, result cache or assembly dependency. Failure
  returns zero widths with the original cause wrapped at the agreed stage.
- [Owning tests](../../typography/paragraph/measure_test.go) instrument resolver,
  shaper, hyphenator and font metrics across repeated measurement and both breakers.
  Ordinary measurements consult none of these services; changing atomic ranges takes
  effect without reshaping, and earlier frozen geometry/glyphs and vlist material
  remain unchanged. Tests cover invalid handles/requests, entry and evaluation
  cancellation, signed normalization, retry, and rejection of automatic intrinsic
  hyphenation without damaging later layout. Stage 4E adds the explicit cases.
- The [OpenType preparation regression](../../typography/adapter/opentype/prepared_test.go)
  measures before and after layouts with both breakers and both paint policies,
  retaining its glyph-ID/cluster assertions for private feature domains and source
  coordinates. Base shaping still occurs once per preparation.
- No tytui callback or preparation table is introduced; those remain 4C–4D.
  Coverage validation is unchanged and its relocation remains deferred.
- Typography and tytui both pass `go test ./... -count=1`,
  `go test -race ./... -count=1`, `go build ./...` and `go vet ./...`.
  These include the generic-package dependency audit, OpenType service tests and
  tytui's retained proof/replay compatibility checks. The 4A khipu implementation
  is unchanged by this increment.

#### 4C. Separate preparation from accepted layout results in tytui

**Implemented.** The invocation-local adapter retains a private entry:

```go
type preparedGroup struct {
    input     InlineGroup
    paragraph *paragraph.Prepared
}
```

A private helper validates group/owner identity, extracts the group, itemizes its
text and prepares it once. Both measurement and final layout use this helper.
The prepared entry retains the same extraction, anchors, source associations,
paint and inherited owner context for both operations. Markup is consumed during
preparation, as it is today. No partial preparation is inserted on failure.

Keep separate invocation-local tables for completed preparations, accepted final
paragraph results and final-layout callback tracking. Measurement followed by
layout is valid; measurement must not trip the repeated-final-layout guard or
populate the accepted-result table. Repeated final callbacks remain unsupported.

Preparation reuse contracts:

- Cache preparations, not measurement answers, initially. A group may be measured
  with different atomic intrinsic ranges; group BoxID alone cannot key an answer.
- Group/owner identity is scoped to one immutable normalized document and one
  invocation's stable services and policies. Reject foreign tree/owner requests.
- Serialize operations on each prepared handle/owner. Retain no invocation-scoped
  CSS view; copy the needed dimensions during each callback.
- Reuse is invocation-local. Independent `Document.Layout` calls still construct
  fresh preparations. Cross-invocation resize caching needs a separate lifetime
  and invalidation contract; do not add it implicitly to `Document`.
- Completed private preparations may survive another callback's failure until
  the invocation is discarded. No preparation handle escapes in published results.
- Publish CSS geometry, accepted paragraphs, source maps and paint together only
  after the whole flow succeeds. Cancellation/failure leaves older snapshots intact.

Acceptance: measurement then layout prepares a group once; repeated measurements
with changed atomic ranges reuse shaping but recompute widths; layout-only groups
still prepare on demand. Exercise failure after earlier groups were prepared,
retry behavior, owner/identity mismatches and unchanged final-callback rejection.

Implementation and verification:

- [Group preparation](../../tytui/documentlayout/preparation.go) extracts,
  itemizes and prepares each requested group once, retaining its source/paint
  envelope and inherited owner context. Both misses and hits validate the exact
  normalized node and OwnerID. A final cancellation check precedes insertion;
  failed attempts leave earlier complete entries intact and insert no partial
  entry. The helper neither accepts a paragraph result nor marks a final callback.
- [The invocation](../../tytui/documentlayout/layout.go) owns separate `prepared`,
  `groups` and `started` tables. Final layout consumes the cached preparation and
  copies primitive dimensions from its current callback views. All final attempts
  retain the no-repeat rule, including failures after successful preparation.
  Only accepted paragraph results and completed CSS geometry are published.
  Preparations, tracking state and CSS request views do not escape in `Result`.
- [Preparation tests](../../tytui/documentlayout/preparation_test.go) measure
  through the private helper and `paragraph.Prepared.MeasureIntrinsic`, then run
  final CSS layout under both breakers. They check exact intrinsic answers for
  changed atomic ranges, unchanged service counts on cache hits/measurement,
  layout-only preparation on demand, multibyte marker/source correspondence,
  retained source/paint/results, and fresh shaping on independent resize calls.
  Other cases exercise identity mismatches before and after caching, late backend
  failure and retry, cancellation, preserved older snapshots, and the distinction
  between preparation retry and forbidden repeated final callbacks.
- This increment does not implement CSS `MeasureInline` or enable auto-width
  inline-blocks. Stage 4D will use the same helper for its callback. No measurement
  answer cache, cross-invocation preparation cache, new coverage validation or
  changes to khipu/typography are introduced.
- Tytui passes `go test ./... -count=1`, `go test -race ./... -count=1`,
  `go build ./...` and `go vet ./...`, including existing nested fixed-size,
  retained-proof, publication/cancellation and replay compatibility controls.

#### 4D. Connect the CSS intrinsic callback and bounded auto-width path

**Implemented.** The invocation adapter implements the existing CSS interface:

```go
func (i *invocation) MeasureInline(
    request box.InlineMeasureRequest,
) (box.IntrinsicWidths, error)
```

For each extracted atomic occurrence, look up intrinsic **border** endpoints in
`IntrinsicAtomicsView` and horizontal margins in `ResolvedEdgesView`. Convert each
primitive separately with the established checked px/DU conversion, then form
minimum and maximum advances using checked sums. Do not round a pre-summed CSS
margin box. Call the prepared paragraph's measurement operation and convert only
the final summary back to CSS pixels, checking float32 narrowing and endpoint order.

Do not add padding/borders again, add the group's owner edges, or recurse into
atomic interiors. [CSS intrinsic scheduling](../../css-box-layout/layout/intrinsic.go)
already owns nested bounds, shrink-to-fit selection and descendant used-value
resolution. No CSS interface change, recursive callback or new cords representation
is anticipated. Owning-library fixes, if exposed by tests, remain with that library.

Pass the invocation adapter as both layouter and intrinsic measurer to `FlowLayout`.
Once callback tests pass, remove tytui's specific rejection of `width:auto` on
inline-blocks. Keep other unsupported policies unchanged, including relative box
lengths, richer whitespace/wrapping and automatic CSS hyphenation. Fixed-width flow
must retain its existing behavior and must not acquire unnecessary measurement work.

Acceptance:

- View/identity validation, arbitrary BoxID ordering, exact-once margin addition
  and primitive rounding budgets, including negative margins and finite limits.
- A simple auto-width atomic fixture exercises measure -> shrink-to-fit -> final
  binding/layout -> publication, checking widths below, inside and above the
  intrinsic interval. A measured group is prepared once and laid out finally once.
- Different atomic measurement inputs reuse preparation without stale answers.
- Measurement failure/cancellation after earlier successful preparations publishes
  no document or accepted paragraph table and preserves older complete results.
- Existing fixed-width, retained-proof and replay tests remain unchanged controls.

Implementation and verification:

- [The measurement callback](../../tytui/documentlayout/measure.go) uses the 4C
  helper, validates required views and atomic identity, and copies border endpoints
  and horizontal margins per occurrence. It rejects reversed border ranges before
  quantization, converts primitives separately, and checks completed signed
  `int64` sums before narrowing. Only border bounds plus horizontal margins are
  passed to typography; padding/borders and owner edges are not added again.
- Final summaries use the existing checked big-point/CSS conversion, including
  float32 narrowing and ordered endpoints. No request view or measurement answer
  is retained, no final callback is marked, and no accepted paragraph is inserted
  during measurement. Context is checked at entry, during evaluation and before
  summary return; failures return zero bounds and preserve completed preparations.
- [Document flow](../../tytui/documentlayout/layout.go) supplies the same invocation
  as both CSS callbacks. The specific auto-width inline-block rejection is removed;
  relative box lengths, broader whitespace/wrapping and automatic hyphenation
  remain unsupported. CSS core owns recursive bounds, shrink-to-fit selection and
  descendant resolution. No sibling engine API change was needed.
- [Callback tests](../../tytui/documentlayout/measure_test.go) cover arbitrary
  BoxID/AtomicID ordering, independent half-DU rounding, exact-once margins,
  signed contributions and wide intermediate sums, missing/invalid views,
  mismatched identities, finite limits and float32 narrowing, changed requests
  without reshaping, cancellation and retained output ownership.
- [Auto-width tests](../../tytui/documentlayout/auto_test.go) observe the real CSS
  callback schedule and hand-calculated intrinsic/shrink-to-fit widths below,
  inside and above the interval under both breakers. A measured group prepares
  once and receives one final callback; every frozen line receives the selected
  CSS target. Public publication agrees with observed flow. Empty auto boxes,
  late measurement failure/cancellation, fresh-invocation retry and preservation
  of older snapshots are covered. A fixed-width control makes zero intrinsic
  callbacks. Existing fixed-size/proof/replay controls remain intact.
- Natural intrinsic bounds do not prescribe breaker scoring or guarantee a fitting
  result under every layout policy. The existing zero-side-skip Knuth–Plass policy
  can select overfull output at min-content width; this increment preserves that
  behavior and checks the selected target independently of line-count choices.
- Typography and tytui pass `go test ./... -count=1`,
  `go test -race ./... -count=1`, `go build ./...` and `go vet ./...`.
  Khipu and CSS core pass full ordinary test suites. These checks include dependency
  boundaries, backend services and the retained fixed-width proof/replay controls.
  No new glyph coverage validation is introduced.

Stage 5 then broadens nested-auto-sizing, resize, conversion and lifetime evidence;
it should not need another callback architecture or preparation abstraction.

#### 4E. Explicit discretionary measurement and preparation reuse

**Remark:**
I am not a fan of testing hyphenation opportunities to find out about minimum width.
Hyphenation is a fairly expensive operation and the chances that all (or even most)
of these will ever be used in the final layout are slim. Super-narrow columns may
be needed in a browser environment with tables and narrow screen real-estate, but
not really needed in other typesetting environments. I can, however, imagine circumstances
where very long "words" will have to be broken up, e.g., a web URL. Therefore I can
imagine that we would want soft-hyphens to be respected for minimum width calculation.
This should be no expensive operation and suffice for most but the very unusual cases.
Users in typesetting environments may be inclined to fix these through manual intervention,
i.e., applying soft-hyphens. (End of new remark)

**Implemented.** Support authored
U+00AD opportunities without running automatic word hyphenation to obtain intrinsic
widths. Automatic intrinsic hyphenation is deferred indefinitely and is not a
stage 4 completion requirement. Existing lazy automatic hyphenation during layout
remains available and unchanged.

The remark above records the motivation. Finding explicit marker positions avoids
linguistic word segmentation, but accurate measurement may still require shaping
both replacement halves: ligatures, joining and break material can change widths.
Reuse completed explicit preparation instead of paying that cost again for each
measurement or binding. Do not assume that slicing existing glyphs and adding a
hyphen width is sufficient, or claim a performance gain without service-call evidence.

The implemented measurement profiles are:

| Prepared policy | Intrinsic measurement |
| --- | --- |
| `NoHyphenation` | Existing ordinary-wrapping evaluation; unchanged. |
| `ExplicitHyphenation` | Include supported authored soft-hyphen replacements under the single-split restrictions below. |
| `AutomaticHyphenation` | Continue returning `ErrUnsupportedPolicy`; no automatic discovery or fallback to explicit-only/ordinary widths. |

Layout-only automatic discovery stays lazy; the shared explicit cache does not
cause eager discovery of unmarked words.

This is a deliberately bounded typesetting policy, not full CSS hyphenation
conformance. CSS includes enabled hyphenation opportunities in min-content sizing
([CSS Text hyphenation](https://www.w3.org/TR/css-text-3/#hyphenation)). The retained
single-split limitation can prevent supporting those semantics for multiply marked
text. Accept that limitation explicitly; do not silently claim unrestricted CSS
min-content results. A sizing policy that ignores automatic opportunities while
layout uses them would need a separately named/documented contract, outside 4E.

For URL-like text, distinguish a hyphenation hint from an optional break without
inserted hyphen material. U+00AD's rendering depends on the break policy and
language/script; U+200B supplies a break opportunity without intrinsic width
([Unicode layout controls](https://www.unicode.org/versions/Unicode17.0.0/core-spec/chapter-23/)).
URL analysis and any additional caller-supplied break representation are separate
work; 4E does not introduce them or assume an ASCII hyphen for every explicit break.

Required work:

1. Enumerate eligible authored soft-hyphen positions from prepared source metadata,
   independently of breaker rescue heuristics or earlier selected/discovered breaks.
   Use the existing explicit provider request for break material (`"\u00AD"`),
   not automatic requests containing whole words. Unmarked text needs no discovery.
   Preserve existing eligibility and feature/provenance restrictions; do not join
   unrelated styled boxes to invent complete hyphenation words.
2. Retain completed immutable explicit replacement material with the preparation,
   under owner control. Current candidates contain binding-local arena references;
   they cannot simply be copied into another binding's candidate map. Import
   replacement records when a binding's provider is asked for that textbox,
   without mutating cached data or eagerly exposing candidates during binding.
   Selected decisions and working arrays remain local to each binding.
3. Evaluate replacement paths with their surrounding unbreakable material.
   Pre-break material contributes together with preceding content; post-break carry
   contributes together with following content. The maximum of isolated fragment
   widths is not a sufficient paragraph measurement algorithm. Max-content retains
   the no-break material; scoring/tolerance does not determine intrinsic answers.
4. Add an internal context-aware explicit preparation/shaping path. Preserve the existing
   public breaker-provider API. The synchronous `Hyphenator` call keeps its existing
   cancellation limit; check context around it and pass context to fragment shaping.
5. Publish only complete immutable cache entries, including successful empty
   explicit discovery. Errors remain retryable. Completed entries may survive a later failed
   request, but partial batches and selected layout decisions may not.

**Retained split limitation:** no successive splits of one textbox are added in
4E. Existing layout may choose one of several alternative explicit split positions,
but each candidate replaces the whole original textbox with a pre-break/post-break
pair; remaining markers are removed from those replacements. These alternatives
cannot be chained into several successive breaks of the same original word.

Keep the planned measurement restriction: reject a textbox with multiple distinct
eligible explicit split positions with `ErrUnsupportedPolicy`, rather than compute
a width assuming successive splits that layout cannot realize. Several independent
textboxes may each have one supported split. Multiple material variants at the same
position are restricted as specified below. Do not change existing layout-only handling
of multiply marked text. Successive splitting is a possible future extension,
requiring a shared representation and changes to both measurement and layout;
it is neither part of 4E nor a prerequisite for completing the bounded stage 4 scope.

Settled design decisions for implementation:

| Decision | Resolution |
| --- | --- |
| Multiple variants at one split | Measurement supports at most one break-permitting material variant at the sole eligible position. Reject two or more with `ErrUnsupportedPolicy`, even if their dimensions or spelling happen to match. Do not choose by provider order, penalty, local fragment width or breaker result. Existing layout-only variant selection is unchanged. |
| Cache representation | Each `ExplicitHyphenation` preparation owns a private, lazily populated map keyed by original template knot index. Entries contain complete immutable shaped replacement pairs, variant/penalty metadata and original source splits, without binding-local arena IDs. Reuse existing `shapedDiscretionaryInput`-style records and reconstruct local storage on import. |
| Cache consumers | Both measurement and lazy explicit discovery from bindings of that preparation use the same owner-controlled cache. Bindings keep a private association with their preparation but import a batch only at the normal provider request point. `BindParagraph` does no discovery, shaping or eager candidate-map population. |
| Explicit eligibility | Use retained textbox source/context and the current explicit-marker rules. Ignore markers lacking non-marker text on either side within that textbox. Count distinct structurally eligible source split positions before consulting the provider; more than one is unsupported for measurement regardless of what the provider might return. Never concatenate neighboring textboxes to recover a split. |
| Context propagation | Add an internal context-bearing cache/preparation operation. Measurement passes its context through fragment shaping and checks around synchronous policy calls. Keep the public provider signatures unchanged; their existing contextless layout path retains its cancellation limits. Do not install a mutable current-context field on the owner. |
| CSS policy mapping | No CSS hyphenation mapping change in 4E. Tytui stays on `NoHyphenation`; mapping `hyphens:none`/`manual`, handling per-span differences and choosing document break-material services are a separate integration increment. Automatic CSS hyphenation remains unsupported. |

The existing public `MeasureIntrinsic`, `PrepareOptions`, `Hyphenator` and
breaker-provider signatures suffice. Typography's existing prepared wrapper
continues to delegate and wrap evaluation failures at `StageMeasure`. No new
public cache, measurement-mode override, provider interface or renderer API is
introduced. Automatic-mode preparations and legacy encoding retain their current
binding-local discovery/cache behavior; 4E's shared explicit cache is scoped to
`ExplicitHyphenation` preparations only.

**Eligibility, material and error rules.** Apply the source-position restriction
even if layout has already cached alternatives: prior layout must not change
whether measurement succeeds. Leading/trailing markers, marker-only text and
unmarked text contribute their ordinary no-break material and require no material
provider call. Consecutive interior markers with distinct source splits count as
multiple positions, even if stripping them produces identical replacement strings.

For a sole eligible position, use the existing marker-only material request and
validate the entire response using existing UTF-8, provenance and feature rules.
The owner still assigns variants and the split after U+00AD; provider-supplied split
coordinates and variant IDs do not override these explicit-source identities.
An empty response declines the break and is cached as a successful empty result.
Empty inserted material is valid: `PreBreak:""` and `PostBreak:""` mean a silent
break with the original left/right text still present, not an empty response.
No default ASCII hyphen or fallback material is invented.

Resolve zero/unspecified penalties through the existing owner default. A variant
with effective `PreBreak.Penalty >= MaxPenalty` does not authorize a measurement
break. Zero permitting variants therefore means ordinary measurement; one means
the explicit split is included; two or more mean `ErrUnsupportedPolicy`. Finite
penalty preferences, tolerance and gain filters do not affect intrinsic widths.
This filtering must not delete variants from a complete layout batch or change
the layout-only provider's response. Malformed material, backend failures and
existing feature/provenance errors preserve their original causes, rather than
being treated as an empty response. Range-limited replacement features remain
unsupported under the existing rules. No new glyph coverage checks are added.

**Deterministic width evaluation.** For min-content, take all supported ordinary
opportunities and the sole permitting explicit split in each supported textbox.
At that split, replace the original textbox contribution with its complete
pre-break fragment, close the current span, and begin the next span with the
complete post-break fragment. Include neighboring unbreakable knots and atomic
minimum advances on the appropriate side. Preserve the original after-textbox
boundary policy after the post-break fragment; its storage defaults must not
invent an extra break. Apply the ordinary first/last-content trimming rule to
each completed span, retaining zero-width content. Do not sum original and
replacement advances together or use the maximum of isolated fragment widths.

For example, a nonbreaking prefix of width 3 followed by a pre-break fragment of
width 7 contributes 10 to one candidate span; a post-break fragment of width 4
followed by a nonbreaking suffix of width 6 contributes 10 to the next. Measuring
only the replacement halves would incorrectly report 7 instead of 10 for this
two-span fixture. Separate supported textboxes may each contribute a split;
successive splits within one original textbox remain excluded.

Max-content uses original no-break text/glue and atomic maximum advances, with
mandatory boundaries retained and no explicit replacement material inserted.
Both evaluations retain stage 4A's paragraph-fill exclusion, checked signed
`int64` accumulation, edge treatment and final normalization
`MinContent = max(0, rawMin)`, `MaxContent = max(MinContent, rawMax)`.
No target width, fake binding, breaker trial or selected-discretionary state is
an input to measurement. Signed intermediate sums may exceed the finite DU range;
published endpoints must remain below `dimen.Fil`, without saturation.

**Cache ownership and publication.** Keep base prepared knots and shaping records
immutable. The new memoization index may grow only under serialized owner
operations; each inserted entry and its glyph/feature/source storage is immutable.
The key is preparation-local, so it needs no text hash, BoxID or atomic width.
Provider and font/scale services remain borrowed under the stable-lifetime
contract. Copy provider-owned material slices before retaining any data.

Cache a complete per-textbox batch, not a paragraph-wide measurement answer.
Absent, successful-empty and complete-shaped entries are distinct states. Perform
all fallible validation/shaping and a final context check before inserting a
complete batch. A failed or cancelled batch leaves no partial entry; earlier
complete batches survive. Unsupported multiplicity is an evaluation error, not
an empty-result cache entry. Reject it before expensive shaping when detectable;
already complete batches from layout remain valid and untouched. Errors are
retryable, although unchanged unsupported input will still be rejected.

Bindings request/import cached pairs lazily, even when measurement populated the
cache before `BindParagraph`. Reconstruct independent local candidate records,
arena indexes and any mutable glyph/cluster/feature storage using existing
publication validation; do not pass a shared cached arena into an importer that
relocates it in place. Imports validate the binding's owner and original
source/context association and commit a whole local batch only after validation.
They consult no shaper or material provider on a cache hit. Bindings created
before the cache was populated can reuse it on a later normal provider request.
Binding selections, public/manual candidates and working-plane edits never feed
back into prepared cache entries. Manual-candidate precedence remains local.
Releasing/unwiring a binding must clear its preparation association.

This lazy visibility rule preserves the breaker's existing discovery schedule.
Measurement must not make additional candidates visible earlier in a breaker
pass, affect selection through a prefilled working map, or import unrelated
textboxes. A layout-first request may populate the shared cache with a full batch,
including variants or multiple positions that measurement rejects; measurement's
restriction must not limit existing layout-only behavior. Cache imports preserve
stable variant order, source splits, glyph clusters and private feature domains.

For measurement, check context during source scanning/evaluation, before and after
the synchronous material provider, during fragment shaping and before publication
and final return. A provider call itself remains non-interruptible. Do not retain
request contexts in cache entries. Layout calls through the existing contextless
provider retain their current cancellation behavior; extending that public
boundary is deferred rather than being hidden in this implementation.

Acceptance for the bounded explicit profile must cover pre/post replacement
contributions across neighboring knots, repeated calls and prior layout independence,
explicit discovery/shaping counts, cache imports into multiple bindings, successful
empty material responses, late backend failure/cancellation, and explicit rejection
of multiple distinct eligible split positions. Assert that unmarked words trigger
no discovery and `AutomaticHyphenation` measurement fails without consulting the
provider. Verify that existing layout-only automatic and multiple-marker behavior
is unchanged. Frozen output from earlier bindings must remain unchanged. The
eventual accepted algorithm needs hand-calculated or independently enumerated
replacement-path oracles, not widths inferred from the breaker's chosen result.

Concrete acceptance cases also include:

- Leading/trailing, marker-only and consecutive markers; an empty provider
  response versus empty inserted material; all-prohibited variants, one permitted
  plus prohibited variants, and two permitted variants at one position. Reject
  unsupported multiplicity consistently before and after layout, without changing
  the variants available to layout-only callers.
- The adjacent-content oracle above, several separately marked textboxes in an
  otherwise unbreakable span, mandatory boundaries, zero-width content and signed
  atomic contributions. Max-content must retain no-break glyph advances.
- Both `Prepare -> Measure -> Bind/Layout -> Measure` and
  `Prepare -> Bind/Layout -> Measure -> Bind/Layout`. Successful explicit batches
  invoke the material provider and fragment shaping only once per preparation
  and textbox; successful empty responses are likewise reused. Repeated queries
  with different atomic ranges recompute widths without reshaping replacements.
- A binding made before measurement, fresh bindings made afterward, and manual
  local candidate edits. Imports do not alias cache records, repeated requests do
  not duplicate local variants, and binding release does not invalidate another
  binding, the preparation or an earlier frozen result.
- With identical final parameters, measurement inserted before layout must leave
  break positions, selected variants, frozen glyph/source geometry and typography
  vlist placements identical under both breakers. In particular, cache hits must
  not change when a breaker first sees candidates.
- Provider failure, pre-/post-fragment shaping failure and cancellation before
  commit publish no partial batch or widths. Retrying preserves prior complete
  entries. Errors remain detectable with `errors.Is`, including through
  typography's `StageMeasure` wrapper; document layout remains on its existing
  `NoHyphenation` profile.

These choices settle the stage 4E scope and behavior. Internal helper names and
record layout may follow repository conventions without reopening these contracts.
Successive splits, ambiguous variant-path optimization, automatic intrinsic
hyphenation, CSS manual/per-span policy mapping and a context-bearing public
provider API remain explicit future work, not unresolved 4E prerequisites.

**Implementation record (4E).** Public signatures are unchanged.

- `intrinsic.go` evaluates explicit pre/post fragments in logical min-content spans,
  preserving surrounding unbreakable material and the original after-box boundary.
  Max-content still reads base material. Both use the ordinary checked signed
  span evaluator and final finite-domain normalization.
- `discretionary_prepared.go` owns the preparation-local complete-batch cache and
  lazy binding import. `PreparedParagraph` retains immutable shaped pairs; bindings
  retain a private association, cleared by unwiring. Import uses existing staged
  publication to allocate detached local arenas. Manual candidates retain priority.
- `discretionary_explicit.go` shares structural marker eligibility between layout
  and measurement and adds internal context propagation. No public provider or
  automatic discovery boundary changed. Whole-input feature mapping still uses
  the existing restrictions, with original byte/scalar provenance preserved.
- Typography's existing delegation needs no algorithm change. Its measurement
  comments, README and tests now describe/verify explicit preparation reuse and
  `StageMeasure` error propagation. No tytui or CSS core code changes are required;
  the document callback deliberately remains on `NoHyphenation`.
- Owning tests cover the hand-calculated neighboring-content and signed-atomic
  oracles; style-split textboxes without invented breaks; markers/material/penalty
  restrictions; empty and full cache batches; failure/cancellation/retry; detached
  glyph/cluster/feature imports, source coordinates, local overrides and release.
  Both-breaker regressions compare layout-first and measure-first break choices,
  frozen glyphs/source geometry and typography vlist placements, with service-call
  counts proving one successful material request and two fragment shapes per
  single-variant textbox/preparation.

Validation completed: `go test ./... -count=1`, `go test -race ./... -count=1`,
`go build ./...` and `go vet ./...` in khipu, typography and tytui; ordinary
`go test ./... -count=1` in css-box-layout. These include generic dependency audits,
OpenType adapter regressions and existing retained document proof/replay controls.
After the final feature-error precedence refinement, khipu's full ordinary suite,
focused explicit measurement/cache race tests and typography's paragraph suite
were rerun successfully. No proof/replay fixtures were regenerated.

#### Stage 4 completion and validation boundaries

4A–4D provide ordinary wrapping; 4E completes the bounded stage 4 objective with
explicit discretionary measurement and preparation reuse. Automatic intrinsic
hyphenation and successive textbox splitting are deferred extensions,
not completion requirements. No claim of general CSS min-content/hyphenation
conformance follows from supporting this bounded subset.

Run focused owning measurement/cache tests, then typography and tytui ordinary/race
suites, build and vet, plus khipu and CSS core regressions. Preserve generic package
dependency audits, backend-service tests and the stage 3 retained proof/replay
controls. With identical final constraints and atomic dimensions, inserting an
intrinsic query must not change the resulting final layout or source correspondence.
Use service-call counts to establish reuse before making performance claims.

Glyph coverage remains the caller's responsibility. Add no new validation and
leave relocation of the existing itemizer checks to its separately deferred task.
Global caches, concurrent operations on one prepared owner, speculative final
callbacks, broader whitespace/inline policies and document replay are not added.

### 5. Verify nested auto sizing and result lifetime

Exercise measure -> width selection -> descendant resolution -> final layout.
Keep CSS core responsible for scheduling and width selection.

Acceptance: nested intrinsic items, conversion/rounding, signed margins, resize,
cancellation and failure recovery; no measurement publishes accepted layout data;
old retained results remain usable after new successful or failed invocations.

## Initial design questions

The following table records the initial questions. The stage 1 proposal below
recommends concrete answers; its final subsection identifies what remains open.

| Decision | Recommended starting point / unresolved detail |
|---|---|
| Attachment representation | U+FFFC plus typed side table; finalize builder and validation API |
| Shared package | Dependency-light contract usable by itemizer and encoder; avoid engine dependency in basic input vocabulary |
| ID scope | Stable within prepared paragraph/invocation, mapped to BoxID by adapter; finalize zero/duplicate rules |
| Unattached U+FFFC | Explicit literal-text or rejection behavior; never infer attachment |
| Prepared-state ownership | Immutable shared shaping with separate working state; decide cache ownership and context-aware discovery |
| Intrinsic policy | Exact supported semantics; define whitespace, discretionary and boundary profiles explicitly |
| Repeated discretionary splits | Extend both paths or reject unsupported CSS cases before claiming general min-content support |
| Boundary policy API | Neutral resolved opportunities/constraints applied before grouping; settle interaction with mandatory breaks and Unicode rules |
| Vertical alignment | Bottom margin edge initially; define future baseline source and optional richer alignment |
| Inline edges and empty elements | Preserve structural associations; specify supported subset and later fragmentation behavior |
| Zero width and signed geometry | Specify outcomes and validation limits consistently through both breakers, freeze, and assembly |
| Paragraph spacing/alignment | Expose resolved geometry without duplicating authorities; preserve existing compatibility behavior |
| Replay | Keep current text-only fixtures stable; define a versioned object/document format only when persistence is requested |

Preserved whitespace modes, glyphless mandatory breaks/blank lines, tabs, full
CSS inline fragmentation, vertical writing, and speculative layouts are separate
extensions. Existing unsupported cases should continue to fail explicitly.

## Stage 1 API proposal

Proposed and refined on 2026-11-14. The remaining stage 1 questions are resolved
below as concrete recommendations for implementation; later-stage extensions stay
explicitly deferred. These are documentation decisions, not implemented APIs.
Existing text-only entry points retain their behavior. Code snippets omit imports
and implementations.

### Shared object vocabulary and ordering

Place the minimal vocabulary in `khipu/contracts`, which the typography itemizer
already consumes. No additional package is needed initially.

```go
// Package contracts.
type AtomicID uint64

type AtomicAnchor struct {
    ID         AtomicID
    ByteOffset uint64 // Paragraph-relative start of one U+FFFC.
}
```

IDs identify occurrences, not reusable resources: two uses of one image have
different IDs. IDs are nonzero and unique within one prepared paragraph. Anchors
are strictly ordered by byte offset, and each addresses exactly one U+FFFC at
`[ByteOffset, ByteOffset+3)`. Duplicate IDs/offsets, invalid byte boundaries,
out-of-range anchors, and anchors on another character are errors. Zero-sized
objects still require anchors and identities. An unattached U+FFFC remains
ordinary text processed by the existing text path, with no inferred attachment.

**Compatibility with css-box-layout:** anchor ordering is a requirement on the
adapter-produced paragraph slice, not on CSS BoxIDs or the prepared-item views.
Both `InlineRequest` and `InlineMeasureRequest` provide a normalized `Group`.
The adapter walks its children in logical order, appends text or a marker, and
records the marker's resulting byte offset. At each atomic boundary it retrieves
dimensions by `Atomics.Lookup(node.BoxID)` and skips the interior subtree.
Neither `PreparedAtomicsView` nor `IntrinsicAtomicsView` specifies an iteration
order; both are ID lookups. Thus no CSS interface change is needed.

For example, logical order may produce anchors `(offset=6, ID=42)` and
`(offset=10, ID=9)`: offsets are increasing, IDs need not be. Bidi may later change
visual order without changing anchor order. Final placement and dimension tables
are matched by identity, not by slice position. CSS requires exactly one placement
per prepared item, including zero-sized ones; it does not require placements in
anchor order. Measurement and layout must use the same extraction semantics and
ID association. IDs alone are not stable across unrelated document rebuilds.

The adapter may use a checked BoxID conversion directly or maintain its own
`AtomicID -> BoxID` table. No payload, dimensions, DOM pointer, font style, or
renderer callback belongs in `AtomicAnchor`.

### Khipu preparation and boundary policy

```go
// Package khipu.
type ParagraphInput struct {
    Paragraph *styled.Paragraph
    Atomics   []contracts.AtomicAnchor
}

type PreparedParagraph struct { /* unexported */ }

func (kq *Khipukamayuq) PrepareParagraph(
    ctx context.Context,
    input ParagraphInput,
    options PrepareOptions,
) (*PreparedParagraph, error)

type BoundaryAction uint8

const (
    AllowBoundary BoundaryAction = iota + 1
    ProhibitBoundary
)

type AtomicBoundary struct {
    ByteOffset uint64
    Action     BoundaryAction
}

type HyphenationMode uint8

const (
    NoHyphenation HyphenationMode = iota
    ExplicitHyphenation
    AutomaticHyphenation // Includes explicit opportunities.
)

type PrepareOptions struct {
    AtomicBoundaries []AtomicBoundary
    Hyphenation      HyphenationMode
}
```

Preparation validates input, analyzes boundaries, resolves styles, shapes text,
and constructs reusable horizontal material with unbound atomic slots. It does
not fabricate a working Khipu containing zero-width stand-ins. The supplied
paragraph already carries bidi resolution; the higher paragraph engine constructs
it from styled text, direction and markup. Markers participate in analysis but
bypass shaping; they require no font glyph. Preparation introduces no glyph
coverage validation: the caller supplies suitable text-to-font assignments.

Use one boundary table rather than independent before/after settings on each
object: adjacent objects share a boundary. Entries are sorted and unique, and may
only address interior paragraph boundaries immediately before or after attached
objects. Omission retains Unicode-derived behavior. Allow means an ordinary
zero-penalty opportunity; prohibit forbids an optional break. Neither splits an
object. An override that would downgrade a mandatory break is an error. Apply
the table before grouping knots.

Forced breaks, arbitrary penalty overrides and per-span wrapping controls are
outside this initial proposal. Adapters reject unsupported policies. CSS-specific
boundary rules are translated by the adapter into this neutral table.

Hyphenation mode is fixed for the prepared paragraph and used consistently by
measurement and layout. NoHyphenation disables discovery even if a provider is
configured; the other modes require the appropriate provider. Missing services
are errors. Intrinsic measurement supports `NoHyphenation` and stage 4E's bounded
`ExplicitHyphenation` profile. Unsupported multiplicity and `AutomaticHyphenation`
return `ErrUnsupportedPolicy`, never silently downgrade. Existing EncodeParagraph
retains its compatibility behavior.

### Intrinsic query and final binding

```go
type IntrinsicWidths struct {
    MinContent dimen.DU
    MaxContent dimen.DU
}

type AtomicIntrinsic struct {
    ID                     contracts.AtomicID
    MinAdvance, MaxAdvance dimen.DU
}

type MeasureRequest struct {
    Atomics []AtomicIntrinsic
}

func (kq *Khipukamayuq) MeasureIntrinsic(
    ctx context.Context,
    prepared *PreparedParagraph,
    request MeasureRequest,
) (IntrinsicWidths, error)

type AtomicAdvance struct {
    ID      contracts.AtomicID
    Advance dimen.DU
}

func (kq *Khipukamayuq) BindParagraph(
    ctx context.Context,
    prepared *PreparedParagraph,
    atomics []AtomicAdvance,
) (*Khipu, error)
```

These inputs are advances, not border dimensions. The CSS integration adds
horizontal margins to supplied intrinsic border bounds or final border width.
Khipu does not interpret margin/border/padding fields.

Each request supplies exactly one entry per attached ID. Missing, duplicate or
foreign IDs are errors; dimension slice order is immaterial. Inputs are copied,
with no retained caller-owned map or callback. Require MinAdvance <= MaxAdvance.
Signed contributions are permitted for negative margins, within finite supported
DU limits. Use checked wider accumulation, not overflow or fill saturation.
Do not clamp each contribution individually; final paragraph bounds satisfy
`0 <= MinContent <= MaxContent`.

BindParagraph creates independent working arrays and empty selected decisions,
sharing only immutable shaping records. Each bound atom has
`W == MinW == MaxW == Advance`. Existing BreakParagraph and Freeze operations
then consume that working Khipu and the same resolved geometry.

Measurement takes no target width, line shape, breaker or alignment options and
does not modify working selections. Stage 4E supplies bounded explicit
discretionary measurement; prior layout discoveries do not determine the answer.

### Frozen object identity

```go
func (f *FrozenKhipu) Atomic(knotIndex int) (contracts.AtomicID, bool)
```

Invalid indexes and non-atomic knots return false. FrozenKnot supplies width and
source range; Atomic supplies identity, and CopyText returns false for atoms.
Every attached occurrence survives exactly once, including zero-width objects.
Allocate `KTAtomic KnotType = 3`, using the currently reserved value. Keep
KTKern=0, KTGlue=1, KTTextBox=2 and KTUserDefined=5 unchanged; leave 4 reserved.
Document that custom kinds remain above KTUserDefined. This does not collide with
the documented custom range, and no supported penalty/discretionary kind is
revived. Uses of reserved value 3 were not supported extension points. No existing
wire format gains atomic support merely through this numeric allocation.

### Typography itemization

```go
// Package typography.
func (r *Resolver) ItemizeInline(
    ctx context.Context,
    text string,
    spans []typestyle.Span,
    atomics []contracts.AtomicAnchor,
    policy PaintPolicy,
) (styled.Text, []Binding, error)
```

This preserves marker bytes and source coordinates and prevents object markers
from being absorbed into text graphemes/shaping runs. It does not size objects or
introduce coverage validation. Coverage is the caller's responsibility; attached
objects require no font glyph and bypass the itemizer's existing coverage checks.
Moving those checks out of the itemizer is a separate future task. Existing
text-only itemization entry points remain available. Preserve original feature
domains when splitting around objects; marker removal must not renumber paragraph
anchors.

### Typography paragraph preparation and layout

```go
// Package paragraph. Input extends the existing record with Atomics.
type Input struct {
    Text      styled.Text
    Direction bidi.Direction
    Markup    bidi.OutOfLineBidiMarkup
    Atomics   []contracts.AtomicAnchor
}

type Prepared struct { /* private owner and prepared paragraph */ }

func Prepare(
    ctx context.Context,
    input Input,
    options khipu.PrepareOptions,
    config khipu.Config,
) (*Prepared, error)

func (p *Prepared) MeasureIntrinsic(
    ctx context.Context,
    request khipu.MeasureRequest,
) (khipu.IntrinsicWidths, error)

type AtomicMargins struct {
    Top, Right, Bottom, Left dimen.DU
}

type AtomicAlignment uint8

const (
    AlignBottomMarginEdge AtomicAlignment = iota + 1
)

type AtomicBox struct {
    ID                        contracts.AtomicID
    BorderWidth, BorderHeight dimen.DU
    Margin                    AtomicMargins
    Alignment                 AtomicAlignment
}

type HorizontalLayout struct {
    LeftSkip, RightSkip khipu.KnotCore
}

type LayoutRequest struct {
    Params     Params
    Horizontal *HorizontalLayout
    Atomics    []AtomicBox
}

func (p *Prepared) Layout(
    ctx context.Context,
    request LayoutRequest,
) (*Result, error)

type AtomicPlacement struct {
    ID   contracts.AtomicID
    Line int
    X, Y float64 // Border origin; fractional big points, Y downward.
}

func (v *VList) AtomicPlacements() []AtomicPlacement
```

Layout derives advances from AtomicBox, binds fresh material, breaks, freezes and
assembles. Callers do not independently supply potentially inconsistent advances
and border geometry. Border sizes are nonnegative, margins signed, and all sums
are checked. Inputs use DU; placements preserve the existing fractional-big-point
output convention. Line is a zero-based paragraph line index. The placement
accessor returns a detached slice in ascending paragraph line order, then frozen
visual-knot order within each line (physical left-to-right traversal). This is
sequence order, not a numeric X sort: negative advances can make coordinates
nonmonotonic, and zero-width items can share X. Use frozen knot order to break
all ties. Consumers associate entries by ID, never by dimension-table index.
An empty placement list may be nil; callers use len rather than nil identity.

Bottom-margin-edge alignment uses the equations given above. Include atomic
margin-box contributions in line extents without assigning text half-leading to
the marker. Zero/unknown alignment is an error. Do not expose an internal-baseline
field until its upstream source and semantics are defined.

Horizontal == nil preserves existing breaker profiles. Otherwise both skips are
complete authoritative values, including explicit zeros, used for breaking and
freezing. Only the geometry fields W/MinW/MaxW of the skip cores matter, following
the existing Params contract. Params.Constraints.Width remains the sole target
width authority. Measured CSS integration initially uses zero natural alignment
skips; indentation needs a separate intrinsic policy.

Zero available width is valid, with overfull results for non-fitting indivisibles.
Preserve existing empty-paragraph/strut behavior; object-only input is not empty.
Existing Run remains the text-only convenience operation and rejects nonempty
anchors, since its arguments contain no atomic geometry. Mixed callers use
Prepare/Layout. New stage errors preserve causes through the existing paragraph
error envelope, using the stage mapping specified below.

### Ownership, concurrency and failure contracts

| Value | Proposed contract |
|---|---|
| Anchor/boundary/dimension slices | Copied by receiving operation; caller may reuse after return |
| PreparedParagraph | Bound to creating Khipukamayuq; foreign owner use is an error |
| Source paragraph, arbitrary styles, font services | Retained under stable-source/service lifetime contracts; no promised deep clone |
| Prepared shaping records | Immutable after publication |
| Lazy discretionary cache | Private and owner-controlled; publish only complete entries |
| Bound Khipu | Independent mutable arrays and selections per binding |
| Frozen result/vlist | Independent of later bindings/selections; concurrent reads under existing immutable-service contracts |

Serialize all operations on one owner, including preparation, measurement,
binding and discovery. paragraph.Prepared has the same restriction. This is an
opaque reusable handle, not a promise of concurrent layout through one instance.
Do not introduce locking, cross-owner transfer, Close, or global caches initially.

Callers must not destructively wrap/edit a retained source paragraph or mutate
retained style/service objects. Copying slices is not deep-copying styled.Style.
At the higher boundary, markup callbacks are consumed during preparation; retain
resolved bidi data rather than requiring those callbacks to survive afterward.

Failure/cancellation publishes no partial result and leaves preparation reusable.
Completed immutable cache entries may survive a failed attempt; partial candidates
and selected layout decisions may not. Preserve current cancellation limits of
synchronous collaborators. Reuse ErrIllegalArguments, propose ErrUnsupportedPolicy
for supported-shape-but-unimplemented policies, and wrap backend/context causes.
Nil contexts are invalid. Current frozen-result lifetime guarantees remain intact.

Document publication stays transactional: measure may retain private preparation,
but only final layout fills accepted-result tables. Publish geometry, paragraph
results, source maps and paint together after overall success. No replay format
change or service serialization is implied.

### Stage 1 resolutions: geometry and unit conversion

Use `F = int64(dimen.Fil)` as the exclusive finite-magnitude bound. Dimension
records never accept Fil/Fill/Filll/Infinity as lengths. Existing symbolic fill
semantics remain valid only for glue capacity fields such as skip MaxW.

- Border dimensions and available width: `0 <= value < F`.
- Individual margins and atomic advances: `-F < value < F`.
- Atomic intrinsic endpoints: finite signed values with MinAdvance <= MaxAdvance.
- Derived advances, margin-box extents and coordinates: calculate in checked
  int64 before narrowing; do not wrap or silently saturate into a fill sentinel.
- Intermediate accumulation may cross the DU range and return through signed
  contributions, provided int64 arithmetic stays valid. Validate completed
  quantities at their consumption/publication boundary, not every prefix.
- Final materialized line content width, natural width including side skips,
  and allocated line width remain nonnegative and below F, matching the current
  freeze geometry contract. A chosen line with a negative total is unsupported;
  return ErrUnsupportedPolicy, do not clamp its width or move its contents.
  There is no automatic retry with a different breaker. Positive overfull lines
  within the domain remain successful output with existing diagnostics.
- Margins may exceed border dimensions in magnitude, giving negative atomic
  advances or negative above/below-baseline contributions. Preserve these signed
  contributions. Combine vertical contributions with the paragraph strut and
  other items before validating the final nonnegative line height. Do not clamp
  an individual object's contribution or rewrite its border dimensions.
- Final paragraph width/height must be nonnegative and below F in DU-equivalent
  units. Baselines and border origins may be signed; require finite magnitude
  below F. Reject arithmetic overflow or out-of-domain published geometry with
  ErrIllegalArguments. Retained fractional-big-point geometry uses the same
  bounds without intermediate integer narrowing.

For intrinsic measurement, first evaluate complete candidate spans under the
selected policy using signed natural contributions and correct edge trimming.
Let rawMin/rawMax be the resulting group bounds before nonnegative normalization:

```text
MinContent = max(0, rawMin)
MaxContent = max(MinContent, rawMax)
```

Normalize at the completed group boundary, not independently for each atom or
knot. Preserve signed candidate arithmetic and reject overflow. Both returned
bounds must be below F; overflowing positive bounds are errors, not capped sizes.
This normalization does not promise that every signed input can be frozen at
every width: negative completed line totals retain the explicit limitation above.
The future intrinsic algorithm must honor these rules without assuming monotonic
prefix widths. CSS core's separate block-contribution combination remains its own
contract; the adapter must not reproduce that recursion inside group measurement.

Adopt the pipeline convention `96 CSS px = 72 big points`. Conversion uses
float64 intermediates, with a finite/range check before conversion to integer DU:

```text
CSS px -> DU:       round(px * (3/4) * 65536)
DU -> CSS px:       float64(du) / 49152
big points -> px:   points * (4/3)
```

Round to nearest with halfway cases away from zero, as existing candidate-width
rounding does. Validate the rounded value before casting to DU. Convert each
primitive border size/margin once, then form advances from those converted
values; do not separately round an already-summed margin-box width. Measurement
uses the same conversion for intrinsic border endpoints and margins. Do not use
outward rounding in only one path, or infer 1 CSS px from dimen.PX.

Retain paragraph output at float64 big-point precision. Narrow only CSS-facing
extent/placement summaries to float32, after finite/range checks. The core retains
its original prepared border dimensions; the adapter must not overwrite them
with quantized round-trip values. Quantization is explicit: each converted
primitive differs by at most half a DU, with additional float32 rounding at the
CSS boundary. Tests budget these errors per conversion rather than use an
unexplained global epsilon. Geometry near the finite limit must be rejected if
conversion makes it unrepresentable.

### Stage 1 resolutions: errors and compatibility

Add these values to paragraph.Stage:

```go
const (
    StagePrepare Stage = "prepare"
    StageMeasure Stage = "measure"
    StageBind    Stage = "bind"
)
```

| Failure boundary | Stage |
|---|---|
| Public-envelope validation, nil handle/context, or already-cancelled context at entry | StageInput |
| Styled paragraph/bidi construction | Existing StageParagraph |
| Lower-level PrepareParagraph, including anchor/boundary validation, resolution and shaping | StagePrepare |
| Intrinsic request validation/evaluation, including unsupported measurement mode | StageMeasure |
| Final atomic geometry validation, advance derivation and BindParagraph | StageBind |
| Breaking, freezing, vertical assembly | Existing StageBreak, StageFreeze, StageAssemble |

Cancellation after entry carries the stage executing when it is detected.
Preserve errors.Is for context cancellation, ErrIllegalArguments,
ErrUnsupportedPolicy, and backend causes through paragraph.Error.Unwrap.
Malformed data, unknown enum values, foreign ownership and numeric overflow are
ErrIllegalArguments; a recognized policy or geometric case intentionally outside
the supported subset is ErrUnsupportedPolicy. Do not convert all backend errors
to argument errors. Existing paragraph ErrInvalidInput behavior remains for
public-envelope validation; wrapping must not discard an available underlying
cause. No new error sentinel per individual bad field is needed.

Existing Run keeps its historical stage labels, including StageEncode, even if
it later reuses preparation internally. Its nonempty-anchors rejection occurs at
StageInput. New Prepare/Layout methods use the mapping above. Tytui may map the
new stages in future integration, but existing text-only replay diagnostics must
not change just because internal factoring changes.

Extend paragraph.Input in place with Atomics. Source inspection of current
typography and tytui callers (including the parlayout/layout import aliases) found
keyed or empty literals, so no positional-literal migration was identified.
Adding a field can still break external positional literals; document this source
compatibility change and migrate any discovered callers to keyed literals during
implementation. Do not add a second input type or constructor solely to preserve
positional literals. Nil Atomics preserves existing input meaning. This review
does not claim compilation of the future API or coverage of external consumers.

### Recommended scope and deferred decisions

Recommendations now made concrete: contracts package for IDs/anchors; U+FFFC
side-table representation; occurrence-local nonzero IDs; literal treatment of
unattached markers; owner-bound serialized preparation; copied request slices;
separate signed intrinsic contributions and fixed layout advances; explicit
boundary table; fixed preparation hyphenation mode; bottom-margin-edge alignment;
zero-width support; unchanged cords storage and text-only convenience behavior.

The five remaining stage 1 questions now have recommended resolutions: reserved
kind 3, line/frozen-knot placement order, checked signed geometry and explicit
conversion/normalization, prepare/measure/bind error stages, and in-place Input
extension with keyed-literal compatibility guidance. No unresolved stage 1 API
question is intentionally left as an implementation prerequisite. Implementation
must verify these contracts rather than treating this document as test evidence.

Later work remains explicitly deferred: repeated discretionary splitting and its
intrinsic algorithm; per-span wrapping; full inline-block baseline contributions;
ordinary inline edge fragmentation and metric-bearing empty inline elements;
indentation contributions; preserved whitespace/blank lines; global caches,
speculative acceptance, and object/document replay formats. Initial adapters
support zero ordinary inline edges and reject unsupported nonzero contributions
or metric-bearing empty elements. They must not silently ignore these inputs.

Separately deferred: remove coverage checking from the typography itemizer and
relocate it to caller-side font preparation, with an explicit migration plan for
coverage-driven fallback. Its destination/API is not settled by this integration
plan. This future task does not change the caller's responsibility for coverage
or authorize new coverage validation in stage 3.

### Stage 1 contract examples and acceptance

- Text-only input has empty attachment and dimension slices.
- Object-only input has one marker and one identity, even for zero dimensions.
- Adjacent objects have distinct ordered anchors and one shared boundary entry.
- Repeated resources have distinct occurrence IDs.
- Nested inline-block interiors are separate prepared paragraphs; the outer
  paragraph carries only the enclosing atomic occurrence.
- Logical offsets may increase while BoxIDs decrease; both CSS lookup interfaces
  remain usable, and bidi may produce a different final visual order.
- A failed operation permits retry with the same preparation.
- Two layouts can share preparation while retaining independent frozen results.

## Verification strategy

Use deterministic synthetic contracts first, then real-font integration proofs.
Include independent numeric expectations, not only screenshots or expectations
generated from the implementation.

Fixtures should cover:

- A word spanning style/shaping knots without a permitted break.
- Ligatures, combining marks, multibyte text, and feature domains beside anchors.
- Mixed LTR/RTL text, isolates, and differently directed atomic interiors.
- Object-only input, adjacent objects, object-space-object, leading/trailing spaces.
- Empty groups, zero available width, zero-size objects, and oversized objects.
- Nested fixed and auto inline-blocks, margins, borders, padding and conversions.
- Soft hyphens, automatic alternatives, multiple potential splits, and unsupported
  policies with explicit errors.
- Cancellation, failed preparation/layout, independent selections, retained
  results, and concurrent reads of published snapshots.

Run focused owning tests, affected integration suites, build and vet at each
implementation stage. Run race coverage when introducing shared preparation or
retained results. Preserve existing paragraph/replay controls. This planning pass
performed no runtime verification.
