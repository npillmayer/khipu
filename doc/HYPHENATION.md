# Option 3 Draft: Lazy Hyphenation With Sparse Discretionaries In `Khipu`

This note sketches a design for Option 3:

- keep base knot data in the current SOA-style `Khipu`
- add a sparse discretionary side-table inside `Khipu`
- discover discretionary candidates lazily
- retain selected discretionary decisions inside `Khipu` for rendering

The goal is not to finalize every API detail yet, but to establish a coherent direction for implementation.

## Problem Statement

There are two main consumers of paragraph data:

- the line-breaker
- the renderer

They need related but different information:

- the line-breaker works with break opportunities
- the renderer works with the break decisions that were actually chosen

In the old list-based `KhipuAOS` path, eager hyphenation is already implemented through `HyphenateTextBoxes()`. That approach directly imports the hyphenation package and eagerly rewrites the paragraph representation.

For the new design, this is not acceptable because:

- eager materialization of all hyphenation opportunities is too expensive
- it couples `Khipu` directly to one hyphenation implementation
- most paragraphs or most lines will never need discretionary lookup

So the target is:

- lazy discovery of hyphenation opportunities
- sparse storage of discovered opportunities
- durable storage of the finally selected discretionary decisions

## Core Constraints

These constraints appear stable.

### 1. No eager hyphenation

The first linebreaking pass should run without hyphenation.

Only if the resulting line costs are too high, or if no acceptable breakpoints can be found, should the line-breaker trigger a second pass that consults a hyphenator.

### 2. Discretionaries must be sparse

Only a minority of textboxes will ever need discretionary candidates.

Therefore discretionaries should not be stored inline with every knot in the base SOA planes.

### 3. Selected discretionary decisions must survive into rendering

The renderer does not work from rune strings and abstract text fragments alone. It renders shaped and measured paragraph state.

Therefore, once linebreaking chooses a discretionary at a breakpoint, that choice must remain attached to the paragraph object.

### 4. `Khipu` should not import the hyphenation package directly

The old eager path already does this. The new path should instead depend on a `Hyphenator` interface.

### 5. Discardability and hyphenation are upstream concerns

The line-breaker should consume:

- discardability flags
- discretionary candidates

It should not infer either from generic knot-type predicates once the system matures.

## High-Level Design

The proposed model separates three things:

1. Base knot data
2. Discretionary candidates
3. Selected discretionary decisions

### Base knot data

This remains the current SOA `Khipu`:

- `W`
- `MinW`
- `MaxW`
- `Penalty`
- `Pos`
- `Len`
- `Kind`

It will likely also need:

- a discardability flag plane or bitset

### Discretionary candidates

These are lazily discovered alternatives attached to a textbox knot index.

They are stored sparsely in a side-table owned by `Khipu`.

Each textbox may have:

- zero candidates
- one candidate
- multiple candidates

### Selected discretionary decisions

These record what the line-breaker actually chose for a given breakpoint.

This information also belongs in `Khipu`, because the renderer must later consume it.

The important distinction is:

- candidates represent opportunity space
- selections represent committed paragraph state

Those must not be conflated.

## Proposed Data Shape

The exact field names can change, but conceptually `Khipu` grows two sparse side structures.

### 1. Candidate table

Key:

- textbox knot index

Value:

- zero or more discretionary candidates

Conceptually:

```go
type discretionaryCandidate struct {
    source     kinx
    whole      fragmentRef
    preBreak   fragmentRef
    postBreak  fragmentRef
    penalty    Penalty
    hyphenW    dimen.DU
}
```

The fields above are intentionally schematic. The important point is that a candidate must carry enough information for:

- linebreaking cost evaluation
- later rendering if selected

### 2. Selected-decision table

Key:

- breakpoint knot index or linebreak decision site

Value:

- which discretionary candidate was chosen, if any

Conceptually:

```go
type discretionaryChoice struct {
    source    kinx
    candidate int
}
```

This can be very small. It only needs to point from a chosen breakpoint to the selected candidate.

## Draft `Hyphenator` Direction

The interface should still start small, but it should not be exposed directly to the line-breaker.

The cleaner boundary is:

- `Khipukamayuq` owns the `Hyphenator`
- `Khipukamayuq` owns the `Shaper`
- the line-breaker asks `Khipukamayuq` for discretionary candidates

So the line-breaker should depend on a narrow callback or provider interface, while `Hyphenator` remains an internal collaborator behind `Khipukamayuq`.

Conceptually:

```go
type DiscretionaryProvider interface {
    Candidates(k *Khipu, at kinx) ([]DiscretionaryCandidate, error)
}
```

and behind that:

```go
type Hyphenator interface {
    CandidatesForTextBox(k *Khipu, at kinx) ([]DiscretionaryCandidate, error)
}
```

Why this split is preferable:

- the line-breaker should not know about shaping services directly
- control over `Khipu` mutation should stay with `Khipukamayuq`
- the `Hyphenator` may need to re-consult a `Shaper` to obtain measured fragment data
- this keeps shaping and hyphenation contracts on the `Khipukamayuq` side of the boundary

So the first version should optimize for ownership clarity, not for making the line-breaker directly aware of every collaborator.

## Lazy Population Workflow

The intended runtime workflow is:

1. `Khipukamayuq` shapes text and produces the base `Khipu`.
2. The line-breaker runs its first pass without hyphenation.
3. If the paragraph quality is below threshold, or if no feasible breaks are found, the line-breaker starts a second pass with hyphenation enabled.
4. During that second pass, when a specific candidate line produces demerits above a configurable threshold, the line-breaker asks its discretionary provider for candidates at the relevant textbox index.
5. `Khipukamayuq` consults its `Hyphenator` and `Shaper`, stores returned candidates in `Khipu`'s sparse candidate table, and hands them back to the line-breaker.
6. The second pass of the line-breaker may now consider these discretionary candidates as additional break opportunities for that local context.
7. When a discretionary is selected, that selection is written into `Khipu`'s decision table.
8. The renderer later consults only the selected decisions, not the whole candidate set.

This preserves laziness while still making the final paragraph state durable.

## Responsibility Split

### `Khipukamayuq`

Responsible for:

- owning and mutating `Khipu`
- preparing base knot data
- preparing discardability information
- coordinating access to the `Hyphenator`
- owning the contract to the `Shaper`
- storing lazily discovered discretionary candidates into `Khipu`

### `Hyphenator`

Responsible for:

- discovering discretionary opportunities for a given textbox or range
- cooperating with shaping/measurement logic as needed, via `Khipukamayuq`
- returning candidates, not decisions

### Line-breaker

Responsible for:

- deciding when hyphenation is needed
- requesting discretionary candidates from its provider callback during the second pass when local line demerits justify it
- evaluating discretionary candidates under linebreaking parameters
- choosing one candidate if a breakpoint uses hyphenation
- writing back the chosen decision

### Renderer

Responsible for:

- consuming only the selected discretionary decisions
- not re-running linebreaking logic

## Parameters Affected By This Design

The design has to leave room for hyphenation-related linebreaking parameters such as:

- minimum hyphenated fragment length
- cost of a single hyphen
- cost of consecutive hyphens
- cost of a hyphen in the first line
- cost of a hyphen in the last line

These belong to linebreaking parameters, not to `Khipu` storage itself.

`Khipu` stores opportunities and decisions; it should not store policy.

## Why A Sparse Side-Table Fits Best

A sparse side-table inside `Khipu` has these advantages:

- no eager expansion of the paragraph
- no per-knot hyphenation payload for knots that will never need it
- selected decisions stay on the paragraph object for rendering
- repeated second-pass attempts can reuse already discovered candidates

This is the main reason Option 3 currently looks strongest.

## Important Non-Goals

This design should avoid:

- eager `HyphenateTextBoxes()`-style rewriting of the whole paragraph
- forcing every knot to carry discretionary fields
- packing semantic flags into `Penalty`
- making the line-breaker depend on a concrete hyphenation package

## Open Design Questions

These points still need concrete design decisions.

### 1. What exactly is the key for a discretionary candidate set?

Chosen first model:

- textbox knot index

This is the simplest stable anchor in the current `Khipu` design.

If future shaping constraints make one textbox insufficient, the key can later widen to a span model. For now, textbox index is the correct first slice.

### 2. What is stored in a candidate?

We need enough information for:

- linebreaking cost calculation
- rendering of the chosen outcome

The simplest useful answer currently looks like:

- measured widths for pre-break and post-break fragments, i.e. `WSS` for both discretionary fragments
- a stable candidate identity

For the first version, it is acceptable to assume that hyphenation variants for a given textbox are reproducible and stably ordered. Under that invariant, storing the selected candidate index is sufficient.

### 3. Should candidate discovery be cached permanently?

Yes for one paragraph instance.

The current assumption is:

- the knots created by `Khipukamayuq` do not change after shaping
- only discretionary side-data is added to `Khipu`

Under that invariant, caching discovered candidates inside `Khipu` is straightforward and does not require complex invalidation logic.

### 4. Where does shaping happen during candidate creation?

Shaping should remain solely the responsibility of `Khipukamayuq`.

So the refined contract is:

- `Khipukamayuq` calls the line-breaker to split the khipu into lines
- if the line-breaker needs candidate discretionaries, it uses a callback/provider to ask `Khipukamayuq`
- `Khipukamayuq` consults the `Hyphenator` and the `Shaper`
- `Khipukamayuq` stores the resulting candidates in `Khipu`
- the line-breaker only receives the candidates needed for evaluation

This keeps control over the fields and side-tables of `Khipu` with `Khipukamayuq`, which is the correct owner of paragraph mutation.

### 5. How are explicit user hyphens and algorithmic hyphenation unified?

The model should eventually describe both:

- explicit discretionary insertion
- lazy discovered discretionary candidates

## Recommended First Design Slice

The safest first slice of Option 3 looks like this:

1. Add a discardability flag plane to `Khipu`.
2. Add a sparse candidate side-table to `Khipu`, keyed by textbox knot index.
3. Add a sparse selected-decision table to `Khipu`.
4. Introduce a minimal discretionary-provider callback for the line-breaker.
5. Introduce a minimal `Hyphenator` interface behind `Khipukamayuq`.
6. Keep all of this inactive until the first linebreaking pass reports “need hyphenation”.

This is enough to make the architecture real without committing to the full final hyphenation model too early.

## Provisional Recommendation

Proceed with Option 3.

More specifically:

- keep base knot storage SOA
- add sparse side structures inside `Khipu`
- make candidate discovery lazy
- make selected decisions persistent
- introduce a `Hyphenator` interface instead of importing the hyphenation package in the new path
- keep the line-breaker dependent on a `Khipukamayuq`-owned provider callback, not on shaping or hyphenation directly

This best matches the actual division of labor in the system:

- `Khipukamayuq` and shaping create paragraph state
- the line-breaker explores and decides
- the renderer consumes the decided paragraph state
