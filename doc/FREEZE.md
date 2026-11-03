# Freezing A Khipu

This note records first design findings for the post-linebreaking step which
assigns final widths to the knots of each line.

For now, the working term will be **freezing the khipu**.

Status checked November 3, 2026: this is a design and implementation plan.
No `FrozenKhipu` type or freeze entry point is implemented. Both modern
linebreakers already return breakpoints and retain chosen discretionary
variants, providing the inputs for this next stage.

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
2. determine the target line width
3. compute the line delta between natural width and target width
4. distribute that delta across the adjustable knots of the line
5. write final knot widths

The freeze step therefore sits between:

- linebreaking
- later box creation / rendering

## Inputs To Freeze

Freezing will need at least these inputs:

- an elastic `Khipu`
- a breakpoint sequence (`[]kinx`)
- selected discretionaries (`SelectedDiscretionaries`)
- paragraph shape (`ParShape`) or explicit target line lengths
- the same left/right skip dimensions used during linebreaking

The public entry point belongs to `Khipukamayuq`; its internal collaboration
with `Shaper` remains to be designed.

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
- a selected discretionary splits one textbox into:
  - one pre-break fragment on the current line
  - one post-break fragment on the next line

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
content. The current discretionary cache stores metrics, not persistent glyph
sequences or fragment strings. Fragment provenance and ownership must be
settled alongside the frozen representation.

The exact shape is open, but the main point is stable:

- freezing should produce a new final structure
- not destroy the elastic one

## Line Construction Before Width Distribution

For each output line, freeze will first need to build the exact knot sequence.

That line-construction phase must:

1. start at the previous breakpoint
2. end at the next breakpoint
3. remove trimmed discardables at both line edges
4. replace selected discretionary textboxes with:
   - `PreBreak` on the broken line
   - `PostBreak` on the following line

Only after this materialization step does it make sense to compute the final
width adjustment of the line.

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

Therefore freeze will likely need, in addition to width ranges, a notion of
adjustment policy or class per knot.

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

The freeze step can be decomposed into two subproblems:

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
- preserve line order

### 2. Set final widths

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

This split looks important because the first half is about structure, while the
second half is about numeric width assignment.

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

## Suggested First Implementation Slice

A reasonable first slice would be deliberately conservative:

1. derive line-local knot lists from breakpoints
2. materialize selected discretionaries
3. trim line-edge discardables
4. freeze only Western-style glue adjustment
5. keep textboxes fixed in that first slice

That would already establish:

- the freeze boundary in the pipeline
- the frozen result type
- the line materialization rules

Only after that foundation is stable should textbox-based adjustment policies
such as Arabic kashida be added.

## Responsibility Split

Clients should continue to face `Khipukamayuq` as the owner of both elastic and
frozen paragraph data.

Internally, freeze will likely be a collaboration:

- `Khipukamayuq` owns the paragraph, breakpoints, selected discretionaries, and
  line construction
- `Khipukamayuq` can directly freeze simple Western interword spacing
- `Khipukamayuq` may need to re-consult the `Shaper` for script-sensitive
  adjustment mechanisms such as Arabic kashida

So the first implementation may be simple and mostly self-contained, but the
ownership boundary should already assume that some scripts will require a
roundtrip through shaping.

## Implementation Plan

The most realistic path looks incremental.

### Stage 1: Define A Frozen Representation

Introduce a new data structure for frozen khipus.

Goals:

- preserve the elastic `Khipu`
- store only final widths
- preserve final line-local knot order
- be explicit about line boundaries

This stage is about data shape only. No real width distribution logic needs to
exist yet.

### Stage 2: Materialize Frozen Lines

Build line-local knot streams from:

- the elastic `Khipu`
- chosen breakpoints
- selected discretionaries

Responsibilities:

- trim line-edge discardables
- emit `PreBreak` and `PostBreak` fragments for selected discretionaries
- preserve knot order within each line

At the end of this stage, each line should have a natural width but not yet a
final adjusted width.

### Stage 3: Western Glue-Only Freezing

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

### Stage 4: Adjustment Classes

Evaluate whether the upstream paragraph model needs additional metadata for
how a knot participates in width distribution. Capacity-proportional allocation
may be sufficient for the initial implementation.

Examples:

- ordinary interword glue
- fixed textbox
- textbox stretchable through kashida
- last-resort shrinkable content

This stage should still keep `Khipukamayuq` as the public owner, but may
internally require a `Shaper` roundtrip.

The current `Shaper` only supplies cluster metrics. Requesting an adjusted
run, reporting the achievable width, and retaining the resulting shaped data
require an explicit contract before script-sensitive freezing is implemented.

### Stage 5: Script-Sensitive Freezing

Once adjustment classes exist, freezing can support script-sensitive width
distribution by delegating the appropriate parts to `Khipukamayuq` and
`Shaper`.

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
- split freezing into:
  - line materialization
  - width distribution
- start with glue-only width distribution
- later add richer per-knot adjustment policies from `Khipukamayuq`

This would establish a clear new stage in the typesetting pipeline without
committing too early to the final non-Western width-distribution strategy.
