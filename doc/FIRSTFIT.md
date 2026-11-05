# Firstfit Status And Notes

Streaming U+00AD support is now connected through the existing discretionary
provider. Configure the encoding Khipukamayuq as `DiscretionaryProvider`.
On textbox overflow firstfit requests explicit fragments and applies its existing
fit/MinHyphenGain selection. Unlike automatic discovery, a retained explicit
marker can also salvage a first word without an earlier feasible checkpoint.
The selected candidate retains its source split and shaped fragment box IDs;
base knots are not rewritten. Automatic streaming discovery now uses the same
provider after structural single-word/context eligibility checks, under the
existing loose-line policy requiring an earlier checkpoint. Multi-box and
repeated same-box splits remain unsupported.

Streaming integration regressions now cover narrow lines and repeated spaces.
After an earlier checkpoint is committed on overflow, the already-scanned
current candidate is reconsidered against the next line's width and retained
as its checkpoint when it fits. Previously that checkpoint was lost, allowing
avoidable overfull lines. This fix applies equally to metric and streaming input.

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
- `linebreak.WSS`, used through a local type alias

All former shared legacy types now live locally or are gone:

- `Parameters` is local to `firstfit`
- `WSS` is again shared in `linebreak`
- demerit sentinels are local constants

The root package now contains shared paragraph shapes and elastic-width
arithmetic. Legacy cursor support and `linebreak/kp` have been removed.

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
- broad discretionary search

However, `firstfit` now does have one deliberately small hyphenation feature:

- local loose-line discretionary rescue when the next textbox overflows

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
- loose-line discretionary rescue with `MinHyphenGain`
- clearing stale selected discretionary decisions on rerun
- local segment-state transitions for leading/trailing trim
- reset preserving carried trim state
- unflagged glue staying neutral

So the core trimming model is no longer just a plan; it is active and tested.

## What Is Still Missing

### 1. Hyphenation is deliberately minimal

`firstfit` now supports a narrow form of hyphenation:

- only the loose-line case
- only when the overflowing textbox has discretionary candidates
- only when the best candidate improves the line by at least `MinHyphenGain`

The current implementation is intentionally still much smaller than
`knuthplass`.

Still missing on purpose:

- tight-line hyphenation
- second pass
- path-dependent hyphen penalties
- global candidate comparison across multiple line variants

The result of a chosen discretionary is stored in `Khipu.SelectedDiscretionaries`,
but the package remains a local greedy breaker, not a path-optimizing one.

### 2. No cost model beyond first-fit choice

The package does not implement:

- demerits
- badness / tolerance thresholds
- paragraph-quality measurement
- path comparison

That is by design, not a migration defect.

### 3. Remaining spacing policy

Paragraph trailer handling is implemented and covered by encoder-to-breaker
integration tests. The encoder supplies exactly one non-discardable final fill
knot. Further work includes paragraph-level kern cases and script-sensitive
upstream flags; these are not blockers for the completed storage migration.

## Relationship To `knuthplass`

The relationship is now clearer than before.

Shared concepts:

- `ParShape`
- `WSS`
- indexed traversal over `Khipu`
- discardability consumed from `Khipu.Flags`

Not shared intentionally:

- parameter types
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
- `WSS`

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
2. Decide whether tight-line hyphenation is worth the extra local complexity.
3. Later, improve upstream discardable-flag assignment in `Khipukamayuq`.

At this point, the migration of `firstfit` itself should be considered done.

## Verification And Next Pipeline Stage

On November 4, 2026, `go test ./...` and `go vet ./...` pass after migration of
legacy coverage to SOA. Both modern breakers exercise the short and long paragraph
regressions, and the root package passes. Step 5 removed the legacy breaker
after its coverage migrated (see [MIGRATION.md](MIGRATION.md)).

Linebreaking chooses indices and discretionary variants; it does not assign
final knot widths. Freezing is the next planned stage, described in
[FREEZE.md](FREEZE.md), before later box creation.
