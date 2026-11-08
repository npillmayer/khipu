# Freezing A Khipu

This note records the design and implementation plan for the post-linebreaking step which
assigns final widths to the knots of each line.

For now, the working term will be **freezing the khipu**.

Status updated November 8, 2026: internal logical-line materialization and
line-local bidi ordering creation are implemented. An internal partial knot
permutation now maps source-backed knots, including selected discretionary
fragments. Final ParFillSkip placement is implemented as a separate internal
step. Atomic internal visual-line construction now connects these stages.
Elastic line geometry now snapshots target widths and physical side skips.
The pure allocator now produces numeric width plans, without changing knots or
glyphs. Internal frozen-output construction now applies these plans for fixed
textboxes and adjustable glue. The separate `frozenParagraph` has final-width
SOA planes, retained shaping/provenance and no elastic knot cores. No public
`FrozenKhipu` type or freeze entry point exists yet. Both modern linebreakers
provide the breakpoints and retained selected variants consumed by this pipeline.

The upstream rewrite is complete for the supported knotting and linebreaking
contracts. `EncodeParagraph(ctx, para)` is the sole encoder. Textboxes retain
glyphs, positioning, source provenance and shaping contexts. Lazy explicit and
automatic discretionaries retain shaped replacement fragments and their original
source split. `BreakpointPositions` maps chosen breaks to paragraph-relative
UTF-8 byte endpoints. The metric-only shaping path has been removed.

The next decisions concern a public freeze/result contract and future
script-sensitive realization, not another encoder migration. See
[the public result contract draft](FROZEN-API.md),
[the overall algorithm](khipu-algorithm.md#line-local-bidi-ordering) and
[discretionary ownership](HYPHENATION.md).

LineShape and both side skips now live in the existing Params, shared by
breaking and internal freezing rather than duplicated in FreezeOptions.
`firstfit.DefaultLayout(shape)` selects zero-width right skip with fil stretch
for ragged-right layout; `knuthplass.DefaultLayout(shape)` retains its 50 BP
finite-stretch profile. Defaults are resolved once before breaking; explicit
zero skips remain valid overrides. Both breaker APIs now take the resolved
Params as their second argument; their algorithm parameter types have no skips.

## Implemented Logical-Line Stage

`Khipukamayuq.materializeLogicalLines` in [logical_lines.go](../logical_lines.go)
is internal. It accepts an owned encoded paragraph and a complete breakpoint
list from either breaker, ending at the final trailer. It validates aligned
planes, legal break endpoints, mandatory-break coverage and selected candidates.
Off-path choices, ambiguous/missing variants, inconsistent source spans and
missing shaping associations fail with no partial result or input mutation.
It does not check whether the path fits a target or is optimal.

Each `logicalLine` retains its ending knot index, original paragraph-relative
byte interval, a trailing-glue L1 interval, materialized knots and natural width. Width excludes external
left/right skips. Knot cores retain their elasticity; this is not frozen output.
Each knot records its original index, source span, arena box reference and,
for replacements, the selected variant and pre-/post-break role. Materialization
explicitly marks the validated final trailer as paragraph fill; dimensions or
source position alone do not establish that identity.

Selected pre-break material ends one line and post-break material seeds the
next. No hyphenator or shaper is called. Empty/silent stored replacements remain
represented. Untrimmed source intervals come from `BreakpointPositions`, not
visible knot lengths, so whitespace, omitted controls and changed spellings do
not corrupt later bidi coordinates.

Trimming matches both breakers: flagged knots before the first or after the
last non-discardable textbox are removed. Retained neutral glue/kern survive
but do not stop trimming. A line without content keeps neutrals and removes all
discardables. The final fill survives. An empty line selected by a breaker is
preserved if supplied by the breaker. Neutral Unicode penalties no longer reward
an empty leading-space line in the standard integration fixture; this is a
ranking change, not a prohibition of that breakpoint or a change to the deferred
first-line whitespace policy.

The result owns line slices and core snapshots and retains arena slice headers
for immutable published records. Appending candidates, replacing selections or
unwiring the original Khipu does not invalidate those records. Original source,
styles, font services and published arena contents must remain immutable.
The entire arena remains retained, including unused candidates; compaction is
deferred. Materialization itself does not split or mutate bidi resolution.

Call before rerunning the breaker: there is no generation token to detect a
stale but structurally compatible breakpoint/selection combination. Result
construction is single-owner, not concurrency-safe. A future public API must
settle how a complete decision is handed off; the internal implementation does
not introduce a public freeze or renderer API prematurely.

Tests in [logical_lines_test.go](../logical_lines_test.go) and
[logical_lines_integration_test.go](../logical_lines_integration_test.go) cover
ordinary/empty paragraphs, forced breaks, flag-based trimming, logical RTL
content, glyphless gaps, changed-spelling and consecutive replacements, silent
fragments, lifetime retention, invalid input and width overflow. Integration
tests exercise ordinary, explicit and automatic results from both breakers.

The intention is:

- linebreaking decides *where* lines break
- freezing decides *how* each line reaches its target width
- later box creation consumes the frozen result

This differs from TeX terminology:

- we do **not** call this "boxing"
- we do **not** plan an "unfreeze"

So freezing should be understood as an irreversible transition from an elastic
paragraph representation to a final line-local width assignment.

## Problem Statement

The current `Khipu` carries elastic widths:

- `W`
- `MinW`
- `MaxW`
- `Penalty`

After linebreaking, that is still only a *potential* paragraph.

To typeset lines, we still need to:

1. determine the actual knot sequence of each line
2. arrange each materialized line in visual segment order
3. determine the target line width
4. compute the line delta between natural width and target width
5. distribute that delta across the adjustable knots of the line
6. write final knot widths

The freeze step therefore sits between:

- linebreaking
- later box creation / rendering

## Inputs To Freeze

Freezing will need at least these inputs:

- an elastic `Khipu`
- the breaker's sequence of line-ending knot indices
- selected discretionaries (`SelectedDiscretionaries`)
- the same resolved Params used during linebreaking, including LineShape and skips
- retained glyph/context/source records and paragraph bidi resolution

The public entry point belongs to `Khipukamayuq`; its internal collaboration
with `contracts.StreamingShaper` for width-directed reshaping remains to be designed.

Breakpoints and selections must come from the same breaker run; a later run
replaces `SelectedDiscretionaries`. A freeze request must validate or snapshot
this combination rather than read mutable selections throughout construction.

The important point is that linebreak results are not enough by themselves.
The freeze step must also know:

- which discretionaries were chosen
- the target width of each output line
- the width range of each surviving knot

The document no longer assumes a separate freeze-time policy for last-line
justification. That decision is already encoded upstream through paragraph skip
settings such as `LeftSkip`, `RightSkip`, and especially `ParFillSkip`.

Likewise, freeze should not decide from scratch which knots are adjustable.
That information already arrives through `W`, `MinW`, and `MaxW`.

## Output Of Freeze

The intended output is a **frozen khipu**:

- each surviving knot has a final `W`
- `MinW` and `MaxW` are gone
- `Penalty` is gone

That means a frozen khipu is no longer a linebreaking input.

This strongly suggests that freezing should produce a representation distinct
from the current elastic `Khipu`, even if the name still contains "khipu".

## Important Finding: Freeze Is Also A Materialization Step

Freezing is not just "write final widths into existing knots".

It also has to materialize the final line-local knot stream.

Examples:

- leading and trailing discardable material, trimmed during linebreaking,
  should not become visible line content
- a selected discretionary replaces one textbox with its pre-break fragment on
  the current line and its post-break fragment on the next line

Therefore the frozen result may differ from the original elastic `Khipu` in:

- knot count
- knot order at line boundaries
- knot identity of split textboxes

This is the strongest argument against freezing the current elastic `Khipu`
in-place.

## Recommended Representation Direction

The current elastic `Khipu` is optimized for:

- linebreaking
- lazy discretionary lookup
- width ranges

A frozen representation should instead optimize for:

- final line-local knot order
- final widths only
- no linebreaking metadata

So the clean direction appears to be:

- keep the current `Khipu` as the elastic paragraph model
- introduce a separate frozen representation derived from it

Conceptually:

```go
type FrozenKhipu struct {
    W     []dimen.DU
    Pos   []uint64
    Len   []uint16
    Kind  []KnotType
    Flags []KnotFlags
    Lines []FrozenLine
}
```

where `FrozenLine` maps line boundaries into the frozen knot stream.

This sketch is not a settled API. In particular, source `Pos/Len` alone
cannot necessarily identify altered discretionary fragments or reshaped
content. Ordinary textboxes already reference an owner-local shaping arena;
owner-produced discretionary candidates also reference retained glyphs,
replacement input and shaping contexts. `SourceSplit` describes original input,
not replacement-string length. The sketch above omits the glyph/context and
provenance associations that a usable frozen representation must preserve.

The internal implementation owns its final planes and provenance records and
retains immutable shaping storage with the existing lifetime guarantee.
Bare owner-local IDs without their arena are
not sufficient. Freezing must not rediscover alternatives or reshape an already
selected fragment merely to recover its stored glyphs. Manual metric-only
fixtures remain useful for arithmetic tests, but cannot supply missing glyph
or source provenance for general line materialization.

The public shape remains open, but the main point is stable:

- freezing should produce a new final structure
- not destroy the elastic one

## Line Construction Before Width Distribution

For each output line, freeze will first need to build the exact knot sequence.

That line-construction phase must:

1. validate the complete breakpoint path, including the final paragraph trailer
2. resolve selected candidates by source knot and variant, retaining their stored fragments
3. build each logical line, carrying a selected post-break fragment onto the next line
4. remove line-edge discardables using the same flag-based semantics as the breakers
5. retain exact original-source line coverage independently of trimmed content

Do not treat every line as a simple slice between knot indices: a selected
discretionary contributes material on both sides of one source knot. The final
ParFillSkip remains non-discardable. Preserve the current first-line trimming
semantics rather than introducing a new whitespace policy here.

Only after this materialization step does it make sense to compute the final
width adjustment of the line.

## Line-Local Bidi Ordering

Linebreaking and initial materialization use logical order. Before final width
assignment, use `BreakpointPositions` from the chosen path to partition the
paragraph's bidi resolution at the original source endpoints. Split
`ResolvedLevels`, apply the trailing-glue L1 approximation, then compute an
`Ordering` for each line; do not split an
already computed paragraph ordering or store numeric levels on knots.

Traverse the ordering's source segments to arrange compatible knot/box groups.
Do not reverse glyph arrays inside boxes. Preserve their direction metadata for
later placement. Replacement fragments attach through their original-source
provenance, not their replacement lengths. Generated skips attach to physical
line edges; source-less material cannot be placed by byte intervals alone.

Source coverage must be partitioned before trimming: discarded spaces still
matter for line-boundary bidi rules. UAX#9 L1 handling is separate from width
trimming. Verify that directional segment boundaries can be represented without
splitting inseparable shaped boxes, and reject unsupported cases explicitly.

### Implemented Ordering Preparation

`logicalParagraph.lineOrderings` in [line_ordering.go](../line_ordering.go)
returns one `bidi.Ordering` per materialized line. It validates complete source
coverage, clones the paragraph's `ResolvedLevels` once, and performs
`Split(lineEnd, false) -> ApplyL1(from, to) -> Reorder()` for each line. The final
line consumes the remainder without another split. Coordinates stay relative
to the original paragraph, never absolute `KnotCore.Pos` or replacement text.

The sibling bidi package now supplies `Clone` and `ApplyL1`; its split fixes
preserve paragraph direction and partition isolates. All mutating operations
run on the private clone. Errors publish no partial ordering list and leave
the original resolution and materialized knots unchanged; retrying is safe
under the existing immutable-source ownership contract.

This internal stage creates orderings only. The separate partial mapping stage
below consumes them without changing the logical snapshot. No public freeze
entry point calls either stage yet.

### Source-Backed Knot Mapping

`logicalLine.mapOrdering` in [knot_ordering.go](../knot_ordering.go) now maps
an Ordering to indices into the materialized line's knot slice. The returned
`knotOrdering` is deliberately not a complete visual line:

- `visual` lists nonempty-source knots, including selected discretionary
  fragments, in physical left-to-right order.
- `unresolved` lists original zero-source items in logical order. These include
  final fill and are passed to the generated-material placement step; they are
  not dropped.

The mapping follows these independently testable steps:

1. Flatten `Ordering.Runs` using `Segments(run.IsOpposite(LeftToRight))`.
2. Validate nonempty, directional, line-local segments. Build a separate
   source-sorted index and reject overlapping/duplicate segment coverage.
3. Validate monotone, nonoverlapping knot source spans within the line. Separate
   original zero-source material, then locate every remaining
   knot in exactly one segment using binary search.
4. Require whole-knot containment. Reject boundaries or source gaps through
   a knot, including same-direction segment boundaries. Glue and kern remain
   indivisible too; no width or cluster splitting is introduced here.
5. Emit segment buckets in visual order, reversing the sequence of knot indices
   within RTL buckets. Never reverse glyph arrays inside boxes.

Segments without surviving knots are valid: they can represent trimmed spaces
or source without glyphs. Every knot occurs exactly once across the two result
lists. Failure returns no partial mapping and changes neither input. Indices
refer to the logical line, not original Khipu indices or byte positions. Cores,
widths, glyphs and arena records stay untouched. Complexity is
`O(S log S + K log S)` time and `O(S + K)` temporary space.

The strict whole-segment rule is an intentional limitation of this first PR.
It does not attempt to prove that a knot crossing adjacent same-direction
segments is safe to keep intact. Such support should be added explicitly with
tests, rather than choosing one source anchor and hiding the boundary.

[knot_ordering_test.go](../knot_ordering_test.go) covers LTR/RTL/mixed traversal,
nested isolates, noncontiguous source intervals, empty/trimmed segments,
source-backed glue/kern, local indices and paragraph-relative offsets, actual
style-split RTL boxes, deferred fill, ambiguity rejection and unchanged
glyph storage. These are mapping tests, not a complete visual-line integration.

### Selected Discretionary Placement

Selected fragments now use the same whole-segment containment rule as ordinary
textboxes. Pre-break provenance must end at the logical line end; post-break
provenance must start at the logical line start. Those roles do not dictate
physical placement: an RTL pre-break fragment can precede other boxes visually.
Only knot indices are permuted. Box references, fragment roles, variant IDs,
shaping contexts, glyph sequences and widths remain unchanged.

The lookup uses original source bytes, never replacement length or cluster
coordinates. Changed spelling and inserted glyphs therefore need no special
mapping. Empty replacement input or input producing no glyphs still participates
if its original provenance is nonempty. Silent does not mean source-less.

Genuinely zero-source fragments are explicitly **unsupported** in this stage:
they return an error, not an unresolved or silently omitted fragment. A position
alone does not identify a segment across a line or direction boundary. We do
not attach to a neighboring box, infer direction from replacement characters,
or place the fragment at a guessed physical edge. Storage accepts endpoint
splits, but current breakpoint validation requires a strictly interior split,
so valid materialized fragments already have nonempty provenance. Supporting
endpoint splits in a future pipeline would also require an explicit attachment
contract here. Cross-segment fragments and fragments covering source gaps are
also rejected atomically, including boundaries between same-direction segments.

`knot_ordering_fragments_test.go` covers mixed-direction placement, actual shaped
LTR/RTL consecutive alternatives, silent replacements, zero-source and ambiguous
failures, repeatability and unchanged retained data. Consecutive alternatives
are published directly so placement tests do not depend on language-specific
hyphenation discovery; the changed-spelling test also uses the provider path.

### Generated-Material Placement

`logicalLine.placeGenerated` in `generated_placement.go` completes the partial
mapping into a new slice of visual-order knot indices. It recognizes only the
explicitly marked, validated final ParFillSkip. It does not classify arbitrary
zero-source glue as fill or infer identity from a stretch sentinel.

Fill goes at the paragraph's trailing physical edge: after content for LTR,
before content for RTL. The final run's direction does not change this policy.
An empty or all-space paragraph still retains its fill; intermediate lines get
no new fill. The fill's natural width and elasticity remain unchanged, including
custom finite or zero capacities. Numeric distribution is a later stage.

The helper takes the **paragraph embedding direction explicitly**. The integrated
constructor now supplies it from `paragraph.BidiLevels().ParagraphDirection()`.
This read-only upstream accessor preserves paragraph direction independently of
text runs and across clone/split/L1/reordering operations. No inference from
`DirectionAt` or independently configured `Params.BidiDir` is needed. Clients do
not need to supply a second direction parameter.

The partial mapping must account for every knot exactly once. Missing,
out-of-range or duplicate indices, already-mapped source-less material, an
invalid marked trailer and unknown unresolved items fail with no partial result
or input mutation. A successful result owns its index slice. It does not yet
construct or publish a visual-line object.

LeftSkip and RightSkip are shared Params fields, not knots in the
materialized snapshot. They are neither synthesized nor placed here. The later
elastic geometry stage carries those exact inputs in physical edge slots,
counting their widths once. Unknown source-less kern/glue and zero-source
fragments still need separate contracts; fill placement is not their fallback.

`generated_placement_test.go` covers LTR/RTL and opposite-direction text,
isolates, empty/all-space paragraphs, wrapped lines, selected replacements,
custom finite fill, invalid mappings, retry safety and unchanged glyphs/widths.

### Integrated Visual-Line Construction

`Khipukamayuq.materializeVisualLines` in `visual_lines.go` accepts the same complete
breaker decision as the logical materializer. It first constructs the logical
snapshot, then calls `logicalParagraph.visualLines` to:

1. Create line-local orderings on a private bidi clone.
2. Read authoritative paragraph direction through `ParagraphDirection()`.
3. Map each line's source-backed knots and selected fragments.
4. Complete each permutation with generated final-fill placement.
5. Copy whole knot records into new physical-left-to-right line slices.
6. Return a `visualParagraph` only after all lines succeed.

`visualLine` is a distinct internal type so its reordered knots cannot be
accidentally passed back to logical trimming/mapping. It preserves the original
line source interval, ending breakpoint and natural width. All knot cores,
elastic bounds, box references, fragment roles and variant identities survive
unchanged. No glyph arrays are reversed or reshaped. The visual paragraph retains
the immutable arena and source under the same lifetime contract as the logical
snapshot; it owns its line and knot slices independently.

Even an error on a later line returns no partial result, leaves logical input
and retained bidi resolution untouched, and permits a retry. Unsupported
source/segment boundaries, the accepted approximate L1 policy and rejection of
unknown source-less material remain unchanged; integration does not widen those
contracts. Output is elastic visual content, not `FrozenKhipu` or renderer boxes.

`visual_lines_test.go` checks direction ownership, nested isolates, empty/all-space
paragraphs, slice independence, retained arena lifetime and later-line failures.
`visual_lines_integration_test.go` verifies actual visual sequences through both
breakers, including mixed directions and forced RTL line breaks. Existing breaker
integration cases also exercise the visual constructor for trimming and selected
automatic/explicit alternatives; fragment tests cover silent and consecutive
replacements through the combined pipeline. Only `export_test.go` exposes value
snapshots to external tests; the production API remains internal.

The next, implemented geometry stage carries target widths and side skips.
Visual snapshots themselves remain content-only; this prevents repeated
preparation from adding skips twice.

### Elastic Line Geometry

`Khipukamayuq.materializeLineGeometry` in `line_geometry.go` connects the complete
breaker decision to visual construction and `visualParagraph.prepareGeometry`.
It accepts the same resolved Params used by the breaker, including shape and
exact left/right skip values. There are no implicit defaults and no dependency on
package `linebreak`: the public `khipu.LineShape` interface declares `LineLength(int32)`, which
existing `linebreak.ParShape` implementations satisfy structurally.

The result is an internal `geometryParagraph` with one `geometryLine` per line.
Its physical sequence is represented explicitly as:

```text
leftSkip | content.knots in visual order | rightSkip
```

The two edge slots hold the supplied `KnotCore` values unchanged, including their
individual elasticity. They do not invent source positions or arena references.
Their role defines edge glue; incidental Kind/Flags/Penalty values do not make
them textboxes, discardable content, or break opportunities. They are not trimmed
or bidi-reordered. Left means physical left even for RTL, while the already
placed final ParFillSkip stays within content and is counted only once.

The shape is sampled once per line, using one-based logical line numbers. Target
width includes both side skips, matching both breakers' bookkeeping:

```text
naturalWidth = leftSkip.W + content.naturalWidth + rightSkip.W
delta = targetWidth - naturalWidth
```

Targets, skips and copied visual content are snapshotted; the result retains no
shape object or mutable line/knot slices from its input. Source/arena records
remain shared under the immutable ownership contract. Shape values must be
stable between breaking and preparation. The breakpoint list cannot detect a
different but structurally valid shape or parameter set. Shape queries should
be side-effect free; queries already made cannot be undone if a later line fails.

Preparation validates ordered width bounds, finite natural widths and targets,
cached content totals, and checked sums. Finite signed skip widths are allowed
provided total natural width stays nonnegative. Positive `Fil`, `Fill` and
`Filll` maxima remain per-item symbols; they are not summed into numeric
capacities. `Infinity`, infinite natural widths and infinite minimum bounds
have no supported geometry contract. Targets and total natural widths must be
below `Fil`. Invalid input yields no partial geometry and changes no input.

This stage does not adjust any W, require fixed textboxes, judge available
stretch/shrink, or declare the line feasible. Both positive and negative delta
are retained, even if the requested adjustment will later prove impossible.
Frozen construction is a subsequent internal stage; no public entry point exists.

`line_geometry_test.go` covers one-based snapshots, signed delta, finite negative
skips, immutable inputs, empty/all-space and RTL lines, final-fill accounting,
invalid widths/targets, overflow and later-line failure. Integration tests use
both breakers with varying targets and nonzero skips; existing breaker tests
also pass trimming and explicit/automatic discretionary results through geometry
preparation. The pure allocator described below consumes this geometry without
changing it.

### Numeric Width Allocation

`allocateWidths` in `width_allocation.go` is a pure, kind-independent operation
over `KnotCore` width triples and a finite target. It returns a new width slice
and never changes input knots. Kind, flags, penalties and glyph information do
not influence the arithmetic. `geometryLine.planWidths` adapts the physical
sequence left skip, visual content, right skip to this operation and returns a
`lineWidthPlan` with separate edge widths and content widths. It validates cached
geometry totals and delta before planning; no shape is queried again.

The allocation policy is:

- Exact natural fit preserves every W, after validating the input.
- Positive adjustment uses only the highest available fill order:
  `Filll > Fill > Fil > finite`. Equal-order fill items receive equal shares of
  the adjustment; their original W values remain distinct. The current data
  model has no separate fill weights. All lower-order items stay at W.
- Without fill, positive adjustment is proportional to `MaxW - W`.
- Negative adjustment is proportional to finite `W - MinW`, even for items
  whose MaxW is a fill sentinel. Infinite shrink remains unsupported by the
  geometry contract. The rough negative-fill pseudocode is future scope.
- Integer quotas are floored; remaining DU are assigned by descending fractional
  remainder, ties in physical left-to-right order. This also applies to shrink
  magnitudes. Each eligible item receives at most one rounding-residue DU.

The resulting sum is exactly the target and each width respects its bounds.
Calculations use checked widened sums and bounded integer products rather than
floating point or saturation. Signed finite widths remain supported. If a fill
assignment would reach a reserved sentinel (for example when large negative
fixed widths offset it), allocation fails instead of publishing nonfinite W.
Insufficient finite capacity returns `errInsufficientCapacity`, distinguishable
with `errors.Is`; malformed/nonrepresentable geometry wraps `ErrIllegalArguments`.
Every failure returns no partial assignment. Complexity is `O(N log N)` time and
`O(N)` storage because of remainder ordering.

**Allocation is not realization.** The same numeric rules apply to a future
elastic textbox, but its proposed width does not update glyphs or positions.
Current encoding produces fixed textboxes; this stage neither discovers textbox
capacity nor calls the shaper. The internal freeze orchestrator restricts its
supported inputs to fixed textboxes rather than silently applying numeric proposals
to glyph-backed elastic text. Later support needs trustworthy upstream capacities
and a shaping contract that realizes requested widths or reports achieved widths
for redistribution. The internal frozen representation implements the fixed-text
subset; no public freeze API exists yet.

`width_allocation_test.go` covers finite stretch/shrink, fill precedence, ties,
exhaustion, signed widths, near-limit arithmetic, kind independence and 2,000
deterministic finite-capacity property cases. `width_plan_test.go` exercises real
geometry, side skips, LTR/RTL final fill, competing fill orders, stale snapshots,
and hypothetical textbox proposals without changing glyphs. It does not claim
typographic realization or paragraph-wide frozen publication.

### Frozen Publication

`Khipukamayuq.freezeParagraph` in `frozen.go` internally connects materialization,
visual ordering, geometry and allocation. It uses the same shape and skip values
as the completed breaker run. `geometryParagraph.freeze` publishes only after
every line succeeds; a failure returns nil, with line context and the underlying
error preserved. The elastic input and its selected decisions are unchanged.

The supported realization is deliberately **glue-only**. Every non-glue content
knot must have `MinW == W == MaxW`, including selected discretionary fragments.
Elastic textboxes, kerns and custom material return `errUnsupportedAdjustment`,
even on exact-fit lines or when a dominant fill would absorb all adjustment.
Fixed non-glue material is retained. Physical side skips are glue by role,
irrespective of their incidental Kind. No shaper/hyphenator is called, and glyph
sequences and positions within boxes remain unchanged.

The internal `frozenParagraph` stores aligned final `w`, `pos`, `len`, `kind`,
`flags` and provenance planes. Lines index half-open ranges in that visual knot
stream and retain inclusive final width, physical side-skip widths and untrimmed
original byte coverage. There are no `KnotCore` values, `MinW`/`MaxW` planes,
penalty planes, or candidate/selection maps. Paragraph fill remains a content
knot at its visual edge, with its assigned finite width; side skips are separate
numeric slots, counted exactly once.

Provenance retains original ranges, box IDs, selected variant/fragment roles and
the explicit paragraph-fill identity. `frozenShaping` retains only immutable
boxes, glyphs, clusters and contexts, not the arena's boundary/penalty tables.
Box IDs therefore remain meaningful after the elastic input is unwired. Unused
arena records may remain reachable; they are not applied output and compaction
is deferred. Source/styles/font services and published shaping records must
remain immutable. Final planes and line/provenance records are independently
owned. No conversion back to elastic input is offered.

`frozen_test.go` covers finite glue stretch/shrink, exact fit, side skips, final
fill, fixed/unsupported non-glue material, later-line failures and retry,
independent planes, selected changed-spelling and silent replacements, source
coverage, empty/trimmed input and retained arena lifetime. End-to-end tests in
`frozen_integration_test.go` consume both breakers with varying targets, nonzero
skips and LTR/RTL paragraphs. The test-only export bridge adds no public API.

### Accepted L1 Approximation

We deliberately do not re-read characters or reclassify Unicode bidi classes
at this stage. Before trimming, `trailingGlueL1` walks backwards over a
contiguous suffix of original-source `KTGlue` knots. Adjacent glue split by
styles is combined. Spans are clipped to the line's original source interval;
unexplained gaps are never bridged. Only the validated source-less final
ParFillSkip is ignored. Textboxes, replacement fragments (even silent ones),
kerns and other zero-source items stop the scan.

The saved interval survives removal of discardable glue. It is passed to
`ApplyL1` after splitting and before reordering, including on final and
all-glue lines. No qualifying suffix means no call. Glue kind, not elasticity,
width or the discardable flag, determines eligibility.

This policy is **not full UAX#9 L1 conformance**:

- Glue classification is broader than bidi WS. NBSP and other non-WS glue may
  be reset unnecessarily; a suppressed soft hyphen can also lie in a glue's
  retained source span.
- Omitted isolate controls or other source gaps can prevent detecting a
  qualifying trailing whitespace sequence. Whitespace inside a mixed text
  cluster is not recognized as glue either.
- Internal segment separators, including tabs, are not treated by this
  trailing-suffix scan. It is not a substitute for all parts of rule L1.

These limitations are accepted for now. If exact treatment becomes necessary,
retain L1 eligibility metadata during upstream character processing rather
than introducing character rescanning here.

[line_ordering_test.go](../line_ordering_test.go) checks real LTR/RTL resets,
successive splits inside an isolate, paragraph-relative offsets, trimmed and
all-glue lines, style-split glue, multibyte spans, suppressed-marker clipping,
discretionary barriers, accepted heuristic limits, validation and retry safety.

## Width Distribution Model

Once a line-local knot list is known, the freeze step must distribute:

```text
delta = target_line_width - natural_line_width
```

There are two cases:

- `delta > 0`: stretch
- `delta < 0`: shrink

The naive TeX-like case is:

- glue carries most or all adjustment

But your target model is broader:

- Western script: usually glue dominates
- Arabic: textboxes may absorb substantial width change (`kashida`)
- other scripts may need still different mechanisms

So the key design question is not "how much total stretch/shrink does a line
have?" We already know that from `W/MinW/MaxW`.

The key design question is:

- **which knots are allowed to absorb the delta, and with what weight?**
- **how are the assigned widths realized by the shaping machinery?**

Eligibility already comes from upstream capacities. Additional weighting or
priority metadata is only a proposal if proportional capacity allocation proves
insufficient; it is not an implemented or settled requirement.

## Important Finding: Per-Knot Capacity Is Not Enough For Policy

The current `Khipu` already provides per-knot width range:

- `W`
- `MinW`
- `MaxW`

This is enough to know *how much* a knot may change.

It is not yet enough to know *how the line should prefer to distribute* that
change.

For example:

- Western interword glue should usually take most of the adjustment
- Arabic textboxes may take substantial adjustment
- some knots may be adjustable only as a last resort

Additional adjustment policy or classes may become necessary, but are not a
prerequisite for the first implementation. Start with capacity-proportional
allocation and explicit fill precedence; add metadata only when a concrete
script-sensitive use case requires it.

Possible future examples:

- glue-like adjuster
- textbox stretchable by kashida
- textbox shrinkable only slightly
- fixed knot

This policy should come from `Khipukamayuq` / shaping logic, not from the
freeze algorithm guessing by `Kind` alone.

So the current understanding is:

- `W`, `MinW`, and `MaxW` describe capacity
- a possible future adjustment class could describe preference or mechanism

This distinction matters because a Western glue knot and an Arabic textbox may
both be stretchable, but not by the same mechanism and not with the same
priority.

## Suggested Decomposition Of The Freeze Step

The freeze pipeline can be decomposed into three independently testable stages:

### 1. Resolve line content

Input:

- elastic `Khipu`
- breakpoints
- selected discretionaries

Output:

- exact knot list per line

Responsibilities:

- remove trimmed material
- split discretionaries
- preserve logical line order and original-source coverage

### 2. Arrange Visual Segments

Use line-local bidi ordering to arrange the materialized groups, preserving
intra-box glyph sequences, direction and fragment associations. Attach generated
edge material explicitly. This changes order, not width capacity.

### 3. Set final widths

Input:

- one resolved line
- target line width
- adjustment policy for the line's knots

Output:

- same line with final `W` assigned to every knot

Responsibilities:

- compute line delta
- distribute delta across adjustable knots
- ensure the sum of final widths equals the chosen target

Keep structural materialization, directional ordering and numeric allocation
separate. Intermediate natural-width lines are not yet frozen output.

## Final-Line Treatment

Final-line treatment should not be modeled as an extra freeze-time switch.

Instead, freeze should simply honor the elastic paragraph data that already
survived linebreaking:

- paragraph-final fill is intended to absorb the remaining space and leave the
  text at its ordinary spacing when that fill takes precedence
- if paragraph-final fill behavior is absent, the last line can be justified
  like any other line

So the final-line result should fall out of ordinary width distribution, not a
special freeze policy.

The numeric allocator implements symbolic fill precedence and leaves finite
spacing unchanged when higher-order paragraph fill absorbs the adjustment.
Side skips of equal order share the adjustment, and higher-order side skips
override paragraph fill. Frozen publication uses the same allocator and stores
the resulting finite widths. The terminal fill must remain non-discardable.

## Original First Slice

The original first slice established logical line materialization only:

1. derive line-local knot lists from breakpoints
2. materialize selected discretionaries
3. trim line-edge discardables
4. retain glyph/context/source references without mutating the elastic paragraph
5. test natural line contents and widths without claiming a frozen result

This establishes:

- the freeze boundary in the pipeline
- the ownership requirements for the eventual frozen result type
- the line materialization rules

Subsequent slices added bidi ordering and fixed-textbox/glue-only freezing.
The internal end-to-end pipeline supports the existing bidi mapping contract,
not only simple LTR input; unsupported boundaries are rejected rather than
silently emitting logical-order lines. Textbox adjustment such as Arabic
kashida comes later.

## Responsibility Split

Clients should continue to face `Khipukamayuq` as the owner of both elastic and
frozen paragraph data.

Internally, freeze will likely be a collaboration:

- `Khipukamayuq` owns the paragraph, breakpoints, selected discretionaries, and
  line construction
- `Khipukamayuq` can directly freeze simple Western interword spacing
- `Khipukamayuq` may need to re-consult a streaming shaping service for script-sensitive
  adjustment mechanisms such as Arabic kashida

So the first implementation may be simple and mostly self-contained, but the
ownership boundary should already assume that some scripts will require a
roundtrip through shaping.

## Implementation Plan

The most realistic path looks incremental.

### Stage 1: Define Representation And Ownership

Intermediate `logicalParagraph`, `logicalLine` and `logicalKnot` types establish
snapshot and arena-lifetime rules and deliberately retain elastic KnotCore
values. A separate internal `frozenParagraph` now implements final-width SOA
storage and retained immutable shaping associations. Public API design remains
open.

Goals:

- preserve the elastic `Khipu`
- store only final widths
- preserve final line-local knot order
- be explicit about line boundaries
- preserve glyph/context references and original/replacement source provenance
- distinguish intermediate logical lines from successfully frozen output

The internal representation is now connected to width allocation by Stage 4.

### Stage 2: Materialize Logical Lines

Implemented internally as described above. No width allocation or bidi ordering
is included in this stage.

Build line-local knot streams from:

- the elastic `Khipu`
- chosen breakpoints
- selected discretionaries

Responsibilities:

- trim line-edge discardables
- emit `PreBreak` and `PostBreak` fragments for selected discretionaries
- preserve logical knot order within each line and exact untrimmed source coverage

At the end of this stage, each line should have a natural width but not yet a
final adjusted width.

Tests should cover ordinary and empty paragraphs, forced breaks, both-edge
trimming, first-line semantics, terminal fill, changed-spelling discretionaries,
post-break carry, stale/missing selections, missing glyph provenance and unchanged
input. Exercise results from both breakers. Keep this PR independent of bidi
mutation and width allocation.

### Stage 3: Integrate Line-Local Bidi Ordering

Ordering creation, clone ownership, the trailing-glue L1 policy and partial
source-backed knot/fragment mapping and final-fill placement are implemented
internally and connected by atomic visual-line construction using
`ParagraphDirection()`. Target widths and physical side skips are now captured by
the separate elastic line-geometry stage. Zero-source
fragments remain an explicit unsupported case until their attachment is defined. Test
visual sequences for LTR, RTL, mixed runs, isolates, trimmed spaces,
discretionary replacements and generated edge material through both breakers.
Reject unsupported boundaries inside inseparable boxes. Do not reverse glyph
arrays within boxes.

### Stage 4: Fixed-Textbox, Glue-Only Freezing

Implemented internally: the numeric allocator and geometry adapter feed atomic
frozen publication for the simplest realizable case:

- glue absorbs stretch and shrink
- textboxes keep their natural width

This stage now:

- computes line delta from natural width to target width
- distributes the delta over adjustable glue knots
- writes final widths
- omits `MinW`, `MaxW`, and `Penalty` from the frozen result

This establishes the freeze stage end to end without yet solving non-Western
adjustment.

This first slice requires all non-glue content to be fixed upstream. It reports
declared non-glue elasticity as unsupported rather than silently ignoring it,
even when the current allocation would not exercise that capacity.

Remainder rounding, insufficient-capacity errors and fill precedence are now
implemented in the pure allocator. Frozen tests cover exact fit, stretch, shrink,
exhaustion, skips, discretionary boundaries, and conservation of total assigned
width. An impossible target is reported rather than
hidden by widths outside the advertised bounds.

Treat fill sentinels as ordered capacities, not ordinary numeric dimensions.
Consume the geometry stage's inclusive target and count its side skips once.
Distribute integer rounding residue deterministically without exceeding bounds.
Publish frozen output only after every line succeeds; errors must leave the
elastic input and its selected decisions intact.

### Stage 5: Evaluate Adjustment Classes

Evaluate whether the upstream paragraph model needs additional metadata for
how a knot participates in width distribution. Capacity-proportional allocation
may be sufficient for the initial implementation.

Examples:

- ordinary interword glue
- fixed textbox
- textbox stretchable through kashida
- last-resort shrinkable content

This stage should still keep `Khipukamayuq` as the public owner, but may
internally require a streaming shaping roundtrip.

The current `StreamingShaper` supplies glyphs and positioning, and the arena
already retains shaped output. What is missing is a contract for requesting a
target width or adjustment mechanism and reporting the width actually achieved.
Simply changing a textbox's W does not realize kashida or other glyph changes.
Design this extension before enabling script-sensitive adjustment.

### Stage 6: Script-Sensitive Freezing

Once the required adjustment contract exists, freezing can support script-sensitive width
distribution by delegating the appropriate parts to `Khipukamayuq` and
its shaping service.

Arabic kashida is the clearest example, but the same architecture should allow
other non-Western justification mechanisms later.

## Risks

### 1. In-place mutation of elastic `Khipu`

This would make the freeze step hard to reason about because:

- selected discretionaries change knot identity
- trimmed material should disappear from lines
- `MinW`, `MaxW`, and `Penalty` disappear permanently

This is a strong reason to prefer a new frozen result structure.

### 2. Mixing materialization and width distribution

If these are not separated conceptually, the implementation may become hard to
debug.

### 3. Under-specifying adjustment policy

If freeze only sees `Kind` and `W/MinW/MaxW`, it will be hard to support
non-Western justification well.

## Provisional Recommendation

The most coherent direction currently looks like this:

- keep the current elastic `Khipu` unchanged after linebreaking
- introduce a separate frozen representation
- separate logical line materialization, visual segment ordering and width distribution
- start with glue-only width distribution
- later add richer per-knot adjustment policies from `Khipukamayuq`

This would establish a clear new stage in the typesetting pipeline without
committing too early to the final non-Western width-distribution strategy.
