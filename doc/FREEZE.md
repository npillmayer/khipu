# Freezing A Khipu

This note records the design and implementation plan for the post-linebreaking step which
assigns final widths to the knots of each line.

For now, the working term will be **freezing the khipu**.

Status updated November 8, 2026: internal logical-line materialization and
line-local bidi ordering creation are implemented. Visual knot permutation and
final width assignment remain a design and implementation plan. No `FrozenKhipu`
type or public freeze entry point exists. Both modern
linebreakers already return breakpoints and retain chosen discretionary
variants, providing the inputs for this next stage.

The upstream rewrite is complete for the supported knotting and linebreaking
contracts. `EncodeParagraph(ctx, para)` is the sole encoder. Textboxes retain
glyphs, positioning, source provenance and shaping contexts. Lazy explicit and
automatic discretionaries retain shaped replacement fragments and their original
source split. `BreakpointPositions` maps chosen breaks to paragraph-relative
UTF-8 byte endpoints. The metric-only shaping path has been removed.

The next work is applying line-local orderings to knots and assigning final widths, not
another encoder migration. See
[the overall algorithm](khipu-algorithm.md#line-local-bidi-ordering) and
[discretionary ownership](HYPHENATION.md).

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
for replacements, the selected variant and pre-/post-break role.

Selected pre-break material ends one line and post-break material seeds the
next. No hyphenator or shaper is called. Empty/silent stored replacements remain
represented. Untrimmed source intervals come from `BreakpointPositions`, not
visible knot lengths, so whitespace, omitted controls and changed spellings do
not corrupt later bidi coordinates.

Trimming matches both breakers: flagged knots before the first or after the
last non-discardable textbox are removed. Retained neutral glue/kern survive
but do not stop trimming. A line without content keeps neutrals and removes all
discardables. The final fill survives. An empty line selected by a breaker is
preserved; in particular, current K&P may select a leading-space breakpoint.
This stage does not change the deferred first-line whitespace policy.

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
- paragraph shape (`ParShape`) or explicit target line lengths
- the same left/right skip dimensions used during linebreaking
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

Decide whether frozen data owns copies or retains immutable arena storage with
an explicit lifetime guarantee. Bare owner-local IDs without their arena are
not sufficient. Freezing must not rediscover alternatives or reshape an already
selected fragment merely to recover its stored glyphs. Manual metric-only
fixtures remain useful for arithmetic tests, but cannot supply missing glyph
or source provenance for general line materialization.

The exact shape is open, but the main point is stable:

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

This internal stage creates orderings only. Mapping their segments to knots,
handling generated edge material and checking inseparable box boundaries are
still outstanding. No public freeze entry point calls it yet.

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

The numeric representation still needs a concrete rule: `dimen.Fill` and
`dimen.Filll` are special dimension values, not an implemented freeze
allocator. Ordinary finite stretch alone does not guarantee that only the
paragraph fill stretches. Define and test fill precedence before claiming
ragged final-line behavior. The terminal fill must remain non-discardable.

## First Implementation Slice

The implemented first slice establishes logical line materialization only:

1. derive line-local knot lists from breakpoints
2. materialize selected discretionaries
3. trim line-edge discardables
4. retain glyph/context/source references without mutating the elastic paragraph
5. test natural line contents and widths without claiming a frozen result

This establishes:

- the freeze boundary in the pipeline
- the ownership requirements for the eventual frozen result type
- the line materialization rules

Follow with bidi ordering and fixed-textbox/glue-only allocation. An initial
end-to-end subset may be explicitly limited to simple LTR input; it must not
silently emit logical-order lines as a fully ordered result for mixed-direction
text. Textbox adjustment such as Arabic kashida comes later.

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

Intermediate `logicalParagraph`, `logicalLine` and `logicalKnot` types now
establish snapshot and arena-lifetime rules. The final frozen representation
remains open; these internal types deliberately retain elastic KnotCore values.

Introduce a new data structure for frozen khipus.

Goals:

- preserve the elastic `Khipu`
- store only final widths
- preserve final line-local knot order
- be explicit about line boundaries
- preserve glyph/context references and original/replacement source provenance
- distinguish intermediate logical lines from successfully frozen output

This stage is about data shape only. No real width distribution logic needs to
exist yet.

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

Ordering creation, clone ownership and the documented trailing-glue L1 policy
are implemented internally. Next, map the orderings to materialized knot/box
groups. Test visual sequences for LTR, RTL, mixed runs, isolates, trimmed spaces,
discretionary replacements and generated edge material. Reject unsupported
boundaries inside inseparable boxes. Do not reverse glyph arrays within boxes.

### Stage 4: Fixed-Textbox, Glue-Only Freezing

Implement the first actual width distribution pass for the simplest case:

- glue absorbs stretch and shrink
- textboxes keep their natural width

This stage should:

- compute line delta from natural width to target width
- distribute the delta over adjustable glue knots
- write final `W`
- drop `MinW`, `MaxW`, and `Penalty` from the frozen result

This establishes the freeze stage end to end without yet solving non-Western
adjustment.

Restrict this first slice to inputs whose textboxes are fixed upstream.
Do not silently ignore textbox capacity used by the linebreaker; report an
unsupported adjustment if realizing it requires an unavailable shaper service.

Before implementation, settle deterministic dimension rounding, behavior when
capacity is insufficient, and fill precedence. Tests should cover exact fit,
stretch, shrink, exhaustion, skips, discretionary boundaries, and conservation
of total assigned width. An impossible target must be reported rather than
hidden by widths outside the advertised bounds.

Treat fill sentinels as ordered capacities, not ordinary numeric dimensions.
Count left/right skips exactly once and define whether the target includes them.
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
