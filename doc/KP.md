# Knuth-Plass Status And Design History

Reviewed against the code on November 4, 2026.

## Current State

The modern implementation in `linebreak/knuthplass` works directly on indexed
`Khipu` data. It returns the single best breakpoint sequence as `[]kinx`
(`kinx` is an alias for `int`) and records applied discretionary variants in
`Khipu.SelectedDiscretionaries`.

The legacy `linebreak/kp` implementation remains available and compiles. It uses
`KhipuAOS`, marks, and its own cursor support. It is a historical reference,
not evidence of complete behavioral parity with the modern implementation.
Its useful test scenarios have moved to the SOA suite; see the coverage map in
[MIGRATION.md](MIGRATION.md).

## Package Boundaries

- `khipu` owns paragraph storage, shaping coordination, flags, and discretionaries.
- `linebreak` owns shared `ParShape`, rectangular shapes, and `WSS`.
- `linebreak/knuthplass` owns dynamic programming, costs, passes, and its parameters.
- `linebreak/firstfit` owns a greedy indexed breaker and its parameters.
- `linebreak/kp` owns legacy cursor, measurement, width, and cost support.

The shared package no longer exposes the old cursor API. Modern breakers share
elastic-width arithmetic without sharing their different cost policies.

## Paragraph And Breakpoint Representation

The elastic SOA planes are `W`, `MinW`, `MaxW`, `Penalty`, `Pos`,
`Len`, `Kind`, and `Flags`. `KnotByIndex` exposes a `KnotCore`.
Penalties belong to knots, unlike the separate penalty items in `KhipuAOS`.

Physical traversal uses knot indices. Logical breakpoints use
`BreakRef{At, Variant}`: variant zero is an ordinary break, and a nonzero
variant identifies a discretionary inside the textbox at `At`.

The agreed paragraph trailer is a single final knot carrying `ParFillSkip`
dimensions and `InfinityMerits`. It must not carry `KFDiscardable`.
The breaker identifies the terminal position by the last knot index; it does
not append the trailer from its `ParFillSkip` parameter. `EncodeParagraph` now
appends this trailer automatically, including for empty paragraphs. Override its
dimensions through `khipu.Config.ParFillSkip`; callers may copy the corresponding
breaker parameter there. Width sums saturate to avoid overflow from fill plus
finite text or skips.

## Path Construction And Collection

`path.go` implements Candidate B from [GRAPH.md](GRAPH.md):
a breakpoint set and predecessor table keyed by `(BreakRef, line)`.
A strictly cheaper transition replaces the existing predecessor; equal costs
retain the existing entry. Distinct variants at the same knot remain distinct.

`kp.paths` holds accumulated costs, worst line badness, and segment bookkeeping.
The horizon contains active `BreakRef` values. Despite its historical name,
`constructBreakpointGraph` now builds predecessor states, not a general graph.

`collectOptimalBreakRefs` chooses the cheapest terminal state over line counts,
follows direct `Pred` lookups back to the root, and reverses the sequence.
The public result projects references to knot indices; selected variants are
recorded separately. Enumerating every solution is intentionally out of scope.

## Trimming

Both modern breakers consume `KFDiscardable` as the authority for trimming.
They no longer infer discardability from glue or kern kinds.

Bookkeeping maintains total width, leading trim, trailing trim, and whether
content has been encountered. Effective width subtracts both trims and adds
the configured left and right skips. Internal spacing remains once later
content arrives. There is no terminal-node override for a wrongly flagged
`ParFillSkip`.

Upstream flag assignment still uses a simple kind-based default. More refined
script-sensitive rules belong to `Khipukamayuq`; we deliberately deferred
an abstraction until there is a concrete policy to implement.

## Cost Calculation And Passes

The numerical cleanup implemented separate stretch/shrink denominators, cubic
badness, squared breakpoint penalties, inclusive tolerance thresholds, and
demerits outside the small penalty-sentinel range. Candidate evaluation
separates geometric feasibility, badness screening, and ranking.

Pass 1 uses `PreTolerance`. If its result is unavailable, unacceptable, or an
overfull fallback, a fresh pass runs with `hyphenating=true` and `Tolerance`.
The winning path records its maximum line badness.

Forced breaks are handled in construction control flow with neutralized local
penalties. Earlier active origins are retired at every forced break, preventing
later lines from bypassing it. Infeasible forced lines retain a fallback path;
64-bit costs prevent repeated fallback demerits from wrapping negative.
First-line discretionary extras and terminal-transition extras are
implemented. Fitness classes and `DoubleHyphenDemerits` remain deferred.
See [DEMERITS.md](DEMERITS.md) for formulas and future fitness-state design.
Parameter defaults differ between `DefaultParameters` and
`NewKPDefaultParameters`; consult the chosen initializer rather than assuming
TeX defaults.

## Lazy Discretionaries

During pass 2, a geometrically feasible candidate with badness above
`PreTolerance` may request local discretionary alternatives. Loose lines look
forward for a textbox; tight lines look backward. These local scans avoid
storing textbox identities on every path.

`Khipukamayuq` coordinates its `Hyphenator` and `Shaper`, measures pre-break
and post-break fragments, assigns their penalty, and caches candidates.
The breaker reads `PreBreak.Penalty` directly and evaluates variants as distinct
`BreakRef` states. Successor lines are seeded with the post-break fragment.
Only winning selections are committed; candidate caches remain available.

See [HYPHENATION.md](HYPHENATION.md) for the implemented interfaces and limits.

## Design Milestones

1. Ported the algorithm from cursor-driven `KhipuAOS` to indexed `Khipu`.
2. Implemented single-best-path backward collection.
3. Activated leading/trailing trim bookkeeping.
4. Replaced the general graph with a predecessor table.
5. Chose sparse in-`Khipu` discretionary storage and a separate flag plane.
6. Implemented numerical cleanup and explicit first/second passes.
7. Added `BreakRef`, lazy shaping roundtrips, and selection persistence.
8. Made discardability flag-driven and shared `WSS` with modern firstfit.

The storage decision considered three alternatives: external discretionary
caches, a dedicated linebreaking view, and sparse tables inside `Khipu`.
The third won because chosen decisions must survive into later paragraph
processing. Flags remain separate from penalty bits. No layout-performance
claim has been established by benchmarks.

## Remaining Work

- Freezing: materialize chosen lines and assign final widths; see [FREEZE.md](FREEZE.md).
- Script-sensitive fragment validity beyond UTF-8 and representable byte length.
- Stronger source/glyph provenance for discretionary fragments and future shaping.
- Removal of the legacy implementation following the completed coverage migration.
- Optional fitness classes and consecutive-discretionary demerits.
- Broader international-script integration coverage.

Path bookkeeping still scans maps by origin in parts of the algorithm.
Performance changes should follow measurement rather than a new representation
chosen only on theoretical grounds.

## Verification

On November 4, 2026, `go test ./...` and `go vet ./...` pass after the SOA coverage
migration. Tests include forced breaks, lazy discretionary selection, and the
short and long paragraph cases from the legacy suite. The root package now passes;
`linebreak/kp` compiles without its superseded tests. This does not establish
complete international-script support or end-to-end typesetting readiness.
