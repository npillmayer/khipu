# Firstfit Status And Notes

This note records the current state of `linebreak/firstfit` after its migration
to the new `Khipu` representation, its remaining overlap with the root
`linebreak` package, and the functional gaps that still remain.

## Current State

`linebreak/firstfit` is no longer cursor-based.

Its runtime API is now:

```go
func BreakParagraph(khp *khipu.Khipu, parshape linebreak.ParShape, params *Parameters) ([]kinx, error)
```

So the package has already moved away from:

- `KhipuAOS`
- `khipu.Mark`
- `linebreak.Cursor`
- `FixedWidthCursor`
- cursor rewinding / backtracking helpers

The old cursor-based implementation and tests have been deleted.

## Remaining Dependency On `linebreak`

The remaining dependency on the root `linebreak` package is now intentionally
small.

`firstfit` only depends on:

- `linebreak.ParShape`
- `linebreak.RectangularParShape` in tests and call sites

All former shared legacy types now live locally or are gone:

- `Parameters` is local to `firstfit`
- `WSS` is local to `firstfit`
- demerit sentinels are local constants

This means the root `linebreak` package has successfully shrunk to the shared
paragraph-shape abstraction.

## Algorithmic Shape

The current `firstfit` implementation is intentionally simple.

It:

- walks a paragraph from left to right over knot indices
- accumulates one current line segment
- remembers the most recent feasible breakpoint as a checkpoint
- commits that checkpoint when a later candidate would overflow the line
- carries the already scanned post-break material forward to the next line

Compared to `knuthplass`, it still intentionally avoids:

- graph construction
- global path ranking
- line demerits / multiple-pass optimization
- discretionary / hyphenation logic

## Discardable-Item Handling

`firstfit` has been aligned substantially with the improved trimming semantics
from `knuthplass`.

The segment bookkeeping now tracks:

- `leadingTrim`
- `trailingTrim`
- `seenContent`
- a carried segment snapshot for checkpoint reset

The current line width calculation subtracts:

- discardable material at the start of the line
- discardable material at the trailing candidate breakpoint

while retaining:

- internal glue / kern between visible content

Checkpoint reset now restores the full carried segment state, not just raw
width, so later-line leading trimming works after a committed breakpoint.

### Source of truth for discardability

`firstfit` no longer infers discardability from `Kind`.

It now treats `Khipu.Flags` as the source of truth:

- flagged knots with `KFDiscardable` trim at line edges
- unflagged knots are not treated as trim-discardable

This matches the current direction in `knuthplass`.

### Current limitation

Although the linebreaker now consumes flags only, `Khipukamayuq` still assigns
those flags through a simple kind-based default during paragraph encoding.
That upstream policy is transitional and should later be replaced by richer
script- and shaping-aware logic.

## Tests

The package tests now use explicit new-`Khipu` fixtures instead of cursor-based
paragraph construction.

Current coverage includes:

- first-fit checkpoint breaking
- trimming trailing discardables at a checkpoint
- trimming leading discardables on later lines
- forced breaks
- local segment-state transitions for leading/trailing trim
- reset preserving carried trim state
- unflagged glue staying neutral

So the core trimming model is no longer just a plan; it is active and tested.

## What Is Still Missing

### 1. No discretionary / hyphenation support

`firstfit` is still deliberately non-hyphenating.

Missing on purpose:

- discretionary-provider integration
- second pass
- storing selected discretionaries back into `Khipu`

This is currently consistent with the intended role of `firstfit` as a simple
breaker.

### 2. No cost model beyond first-fit choice

The package does not implement:

- demerits
- badness / tolerance thresholds
- paragraph-quality measurement
- path comparison

That is by design, not a migration defect.

### 3. Some edge-case hardening is still possible

The main remaining trimming-related hardening items are:

- explicit paragraph-level kern regressions
- a final review of paragraph trailer handling once the upstream flag policy is
  settled

The latter should be straightforward if the final `ParFillSkip` is guaranteed to
arrive without `KFDiscardable`.

## Relationship To `knuthplass`

The relationship is now clearer than before.

Shared concepts:

- `ParShape`
- indexed traversal over `Khipu`
- discardability consumed from `Khipu.Flags`

Not shared intentionally:

- parameter types
- width helper types
- cost model
- discretionary machinery

This is a healthier split than the old `linebreak` package, which mixed shared
concepts with legacy cursor-era implementation support.

## Package-Structure Outcome

The package structure is now close to the intended clean state:

### `linebreak`

Keeps only:

- `ParShape`
- `RectangularParShape`

### `linebreak/firstfit`

Owns:

- the simple indexed breaker
- its own parameters
- its own width bookkeeping

### `linebreak/knuthplass`

Owns:

- the richer path-based breaker
- cost calculation
- second-pass discretionary logic

## Recommended Next Steps

The next reasonable work items are:

1. Add explicit paragraph-level kern regressions for trimming.
2. Leave hyphenation out of `firstfit` unless there is a concrete product need.
3. Later, improve upstream discardable-flag assignment in `Khipukamayuq`.

At this point, the migration of `firstfit` itself should be considered done.
