# Lazy Hyphenation With Sparse Discretionaries

This design has the following overall goals for introducing hyphenation (“discretionaries”) in linebreaking:

- add a sparse discretionary side-table inside `Khipu`
- discover discretionary candidates lazily
- retain selected discretionary decisions inside `Khipu` for rendering

## Current Status

Currently in place:

- `Khipu` has sparse side-tables for discretionary candidates and selected discretionary decisions.
- `linebreak/knuthplass` has a second pass with discretionary-provider lookup.
- the line-breaker uses internal `BreakRef{At, Variant}` identities so multiple discretionary variants may coexist at one knot index during path search.
- the public result of `BreakParagraph(...)` remains `[]kinx`; the chosen discretionary variant is recorded separately in `Khipu`.
- after a successful linebreaking run, `Khipu.SelectedDiscretionaries` contains exactly the applied discretionary choices from the winning path, and stale selections from earlier runs are cleared.
- discretionary candidates now carry their own penalty in `PreBreak.Penalty`, and the line-breaker reads that value directly during cost calculation.
- first-line and final-line discretionary demerits are now implemented as local linebreaker adjustments.

Currently still incomplete:

- no consecutive-hyphen demerits yet
- no minimum-fragment-length filtering yet (has been pushed into Khipukamayuq's responsibility)
- no special support of final renderer-side application of selected discretionaries yet

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
The current implementation already has these as maps on `Khipu`.

### 1. Candidate table

Key:

- textbox knot index

Value:

- zero or more discretionary candidates

Current shape:

```go
type DiscretionaryCandidate struct {
    Variant   uint16
    PreBreak  KnotCore
    PostBreak KnotCore
}
```

The important point is that a candidate must carry enough information for:

- linebreaking cost evaluation
- later rendering if selected

In the current model, this means:

- `PreBreak` carries the shaped/measured pre-break fragment
- `PostBreak` carries the shaped/measured post-break fragment
- `PreBreak.Penalty` carries the discretionary penalty to be used by the cost calculator

This is intentional. The line-breaker should not inject a hyphen glyph or infer a discretionary penalty itself. It should read both width and penalty data from the candidate supplied by `Khipukamayuq`.

### 2. Selected-decision table

Key:

- breakpoint knot index or linebreak decision site

Value:

- which discretionary candidate was chosen, if any

Current shape:

```go
type DiscretionarySelection struct {
    Source  int
    Variant uint16
}
```

This can be very small. It only needs to point from a chosen breakpoint to the selected candidate.

This is now the minimum committed result of linebreaking beyond the returned `[]kinx`.

## Draft `Hyphenator` Direction

The interface should still start small, but it should not be exposed directly to the line-breaker.

The cleaner boundary is:

- `Khipukamayuq` owns the `Hyphenator`
- `Khipukamayuq` owns the `Shaper`
- the line-breaker asks `Khipukamayuq` for discretionary candidates

So the line-breaker should depend on a narrow callback or provider interface, while `Hyphenator` remains an internal collaborator behind `Khipukamayuq`.

Current line-breaker seam:

```go
type DiscretionaryProvider interface {
    DiscretionaryCandidates(k *Khipu, at int) ([]DiscretionaryCandidate, error)
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
7. Internally, these opportunities are represented as `BreakRef{At, Variant}` states so multiple variants at one textbox can coexist during path search.
8. When a discretionary is selected, that selection is written into `Khipu`'s decision table.
9. The renderer later consults only the selected decisions, not the whole candidate set.

Current invariant after step 8:

- `SelectedDiscretionaries` contains only the decisions from the winning path
- unused candidate discretionaries remain cached in the candidate table
- rerunning linebreaking on the same `Khipu` replaces the old committed decision set

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

The current implementation already does the first four items, except that the richer hyphen-related demerits are not active yet.
It also already writes back the minimum committed discretionary decision set to `Khipu`.

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

One refinement has emerged from implementation:

- the cost calculator should read the penalty from the discretionary candidate itself, i.e. from `PreBreak.Penalty`
- first-line and final-line discretionary extras can stay as local linebreaker demerit hooks, because they do not require any richer `Khipu` storage

This does not make `Khipu` the owner of hyphenation policy. It only means that `Khipukamayuq`, as the producer of discretionary candidates, is the authority on what penalty belongs to that candidate for the current script/shaping context.

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

- measured widths for pre-break and post-break fragments, currently represented as `KnotCore`
- a stable candidate identity

For the first version, it is acceptable to assume that hyphenation variants for a given textbox are reproducible and stably ordered. Under that invariant, storing the selected variant ID is sufficient.

The current implementation also relies on:

- `PreBreak.Penalty` being the effective discretionary penalty for cost calculation

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

### 6. How should the renderer consume a selected discretionary?

The line-breaker now records `DiscretionarySelection{Source, Variant}` into `Khipu`.
The renderer-side contract for consuming that choice is still open.

Before rendering, there will be further stages such as box creation and glue setting. So for now the design stops deliberately at the minimum persistence point:

- the line-breaker returns ordinary breakpoints as `[]kinx`
- the final `Khipu` retains only the applied discretionary selections from that same winning path
- no renderer-facing helper layer is introduced yet

## Recommended First Design Slice

The safest first slice of Option 3 looks like this:

1. Add a discardability flag plane to `Khipu`.
2. Add a sparse candidate side-table to `Khipu`, keyed by textbox knot index.
3. Add a sparse selected-decision table to `Khipu`.
4. Introduce a minimal discretionary-provider callback for the line-breaker.
5. Introduce a minimal `Hyphenator` interface behind `Khipukamayuq`.
6. Keep all of this inactive until the first linebreaking pass reports “need hyphenation”.

This slice is now mostly in place. The next implementation topics are no longer storage and pass-2 entry, but:

1. candidate filtering rules such as minimum fragment length
2. richer discretionary demerits
3. renderer-side consumption of `SelectedDiscretionaries`

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
