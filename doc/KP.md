# Knuth-Plass Status And Design History

Reviewed against the code on November 8, 2026, including emergency passes,
retirement, result ownership and downstream materialization.

## Current State

The modern implementation in `linebreak/knuthplass` works directly on indexed
`Khipu` data. It returns the single best breakpoint sequence as `[]kinx`
(`kinx` is an alias for `int`) and records applied discretionary variants in
`Khipu.SelectedDiscretionaries`.
Here, "best" means minimum accumulated cost among the candidates explored in
the first successful pass, not exhaustive enumeration of every possible
hyphenation. Successful calls replace the selected variants; errors preserve
earlier selections, although discovery may populate candidate caches.

The call is now `BreakParagraph(input, layout khipu.Params, algorithmParams)`.
Shape and side skips come exclusively from the resolved paragraph Params;
reuse that value for freezing. `DefaultLayout(shape)` starts with base paragraph
defaults and retains K&P's zero left skip and zero-natural-width right skip with
50 BP finite stretch. Apply overrides after initialization, including explicit
zero skips. Nil algorithm parameters do not substitute layout defaults.
The algorithm Parameters no longer has LeftSkip/RightSkip. Its ParFillSkip field
is still an upstream encoding convenience; actual trailer geometry comes from
the encoded input, not from defaults reapplied during breaking.

The legacy `linebreak/kp` implementation and its AOS/cursor support have been
removed. Its useful test scenarios moved to the SOA suite before deletion; see
the coverage map in [MIGRATION.md](MIGRATION.md). Behavioral parity with the old
breaker is not promised.

## Package Boundaries

- `khipu` owns paragraph storage, shaping coordination, flags, discretionaries,
  and shared paragraph geometry in Params/LineShape.
- `linebreak` owns rectangular shapes and `WSS`; `ParShape` aliases `khipu.LineShape`.
- `linebreak/knuthplass` owns dynamic programming, costs, passes, and its parameters.
- `linebreak/firstfit` owns a greedy indexed breaker and its parameters.

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
prefer the lower predecessor knot index, then variant, regardless of arrival
order. Distinct variants at the same knot remain distinct.

`kp.paths` groups working states as `map[BreakRef]map[lineNo]bookkeeping`.
Each state holds accumulated costs, worst line badness, and segment bookkeeping.
Width updates and ordinary/discretionary scoring visit only the requested
breakpoint's bucket. Full BreakRef keys keep variants at the same knot separate.
The rescan `seeds` map remains keyed by `(BreakRef, line)`; predecessor storage
is unchanged. Overfull working states/seeds are retired after discretionary
rescue; historical predecessors and retirement tombstones remain. Negative
minimum widths or unsafe sums disable early pruning. See [RETIREMENT.md](RETIREMENT.md).
The horizon contains active `BreakRef` values. Despite its historical name,
`constructBreakpointGraph` now builds predecessor states, not a general graph.

For built-in rectangular shapes, each completed ordinary boundary retains only
its cheapest working state across line counts (fewer lines wins a cost tie).
Actual line numbers and predecessor history survive. Root/discretionary buckets
are not compacted; custom or non-uniform shapes keep the baseline policy. This
dispatch is automatic in every pass, including public calls. See [UNIFORM.md](UNIFORM.md).

Each single-pass breaker owns separate reusable slices for ordinary evaluations,
discretionary candidates and the sorted horizon snapshot. Results borrow storage
until the next call to their producer; callers consume them before rescoring.
The ordinary buffer survives nested discretionary evaluation in passes 2 and 3. Horizon
membership may change while a snapshot is consumed. A completed discovery phase
is followed by a fresh boundary snapshot, so newly discovered post-break lines
can reach a forced boundary immediately. Retirement IDs use another scratch
slice. No global pool or cross-paragraph sharing is used.

At permitted boundaries in passes 2 and 3, `boundaryEvaluations` copies ordinary discovery
results into separate reusable storage, indexed by full breakpoint and penalty.
Commit reuses unchanged buckets rather than classifying them twice. A successful
working-state insertion/replacement, width update, or retirement invalidates the
entire affected bucket, including sibling line counts. Losing proposals do not.
Thus new and replaced discretionary successors still receive current scoring.
Accumulated predecessor costs are read live, not cached. Entries are scoped to
one boundary, including error exits; pass 1 and forbidden boundaries allocate no
cache. The existing stable-layout/parameter contract applies during a pass.

`collectOptimalBreakRefs` chooses the cheapest state in the terminal bucket over line counts
(preferring fewer lines on equal cost),
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

The encoder now translates finite allowed UAX#14 opportunities to zero penalties;
mandatory and prohibited classifications retain their sentinels. Raw Unicode
rule weights no longer act as squared rewards for extra lines. Explicit knot
preferences and discretionary penalties remain unchanged. See
[DEMERITS.md](DEMERITS.md#unicode-boundary-translation).

Pass 1 uses `PreTolerance`. If no acceptable complete path exists, a fresh pass
runs with `hyphenating=true` and `Tolerance`. With positive finite
`EmergencyStretch`, pass 2 is non-final and failure starts a fresh emergency
pass with the same tolerance. Otherwise pass 2 is final. Only the final pass
may fall back to over-/underfull lines. The winning path records its maximum
line badness under that pass's scoring assumptions.
Shared classification rejects `MinW > target` independently of tolerance;
short lines without stretch get badness 10000 and remain active.

Forced breaks are handled in construction control flow with neutralized local
penalties. Earlier active origins are retired at every forced break, preventing
later lines from bypassing it. Non-final passes must satisfy tolerance there;
the final pass can create a deterministic fallback before deleting its source.
64-bit costs prevent repeated fallback demerits from wrapping negative.
First-line discretionary extras and terminal-transition extras are
implemented. Fitness classes and consecutive-discretionary demerits remain deferred.
See [DEMERITS.md](DEMERITS.md) for formulas and future fitness-state design.
The unused `HyphenPenalty` and `DoubleHyphenDemerits` fields and defaults have
been removed. Candidate penalties come from upstream `PreBreak.Penalty`;
`Params.Hyphenpenalty` (default 10) is the owner's fallback when the provider
does not supply a nonzero penalty.
`EmergencyStretch` augments finite stretch only for emergency scoring and lazy
discovery, without changing knots or shrink capacity. Nonpositive values disable
pass 3; values at or above `dimen.Fil` fail validation before pass 1. The first
complete pass wins, and only its discretionary selections are published.
`Max >= dimen.Fil` keeps existing numerical fill/saturation behavior; exact fill
semantics are not added. Freezing still needs over-/underfull output support.
`NewDefaultParameters` uses zero E to preserve default two-pass behavior;
`NewTolerantParameters` enables 50 BP emergency stretch.
See [PASS3.md](PASS3.md) for regression coverage and
[TESTING.md](TESTING.md#emergency-pass-workloads) for public/isolated emergency
benchmarks, baselines and workload limitations.

### Parameter Profiles

Both constructors return independent parameter values; neither reads a mutable
global. Nil algorithm parameters select `NewDefaultParameters`, never the
tolerant profile. Numeric defaults have not changed:

| Setting | `NewDefaultParameters()` | `NewTolerantParameters()` |
| --- | ---: | ---: |
| PreTolerance | 100 | 100 |
| Tolerance | 200 | 5000 |
| LinePenalty | 10 | 10 |
| FirstHyphenDemerits | 0 | 0 |
| FinalHyphenDemerits | 10000 | 50 |
| EmergencyStretch | 0 | 50 BP |
| Final configured pass | 2 | 3 |

Both leave the discretionary provider unset and provide fill glue as the upstream
ParFillSkip convenience. Geometry remains in separately resolved `khipu.Params`.
There is no shared mutable profile or compatibility constructor. Use the two
named constructors and customize the returned value for each caller.

Provider/input errors are returned to the caller without being logged at the
internal pass boundary. The caller owns error reporting; failed searches are
not retried after such errors.

## Lazy Discretionaries

During passes 2 and 3, candidates above `PreTolerance` and all overfull candidates
may request local discretionary alternatives before retirement. Loose lines look
forward for a textbox; tight lines look backward. These local scans avoid
storing textbox identities on every path.
Pass 3 uses emergency-adjusted badness for this threshold. Explicit markers
request discovery independently of badness. The historical helper names
`hyphenationCandidateForPass2` and `requestPass2Discretionaries` cover both passes.

`Khipukamayuq` coordinates its `Hyphenator` and `Shaper`, measures pre-break
and post-break fragments, assigns their penalty, and caches candidates.
The breaker reads `PreBreak.Penalty` directly and evaluates variants as distinct
`BreakRef` states. Successor lines are seeded with the post-break fragment.
Only winning selections are committed; candidate caches remain available.
Immutable rescan seeds contain only the post-break fragment; already-consumed
suffix knots belong to live bookkeeping. Lazy provider fragments must have
nonnegative minima and obey the accumulation safety bound documented in
[RETIREMENT.md](RETIREMENT.md#why-retirement-is-safe).

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
9. Added breakpoint indexing, scratch reuse, safe active-state retirement,
   successful-empty discovery caching and boundary-local evaluation reuse.
10. Implemented the optional emergency pass, deterministic final fallback,
    and public-call versus isolated-pass benchmark coverage.

The storage decision considered three alternatives: external discretionary
caches, a dedicated linebreaking view, and sparse tables inside `Khipu`.
The third won because chosen decisions must survive into later paragraph
processing. Flags remain separate from penalty bits. Synthetic performance
benchmarks and the logging-guard/breakpoint-indexing measurements are described in
[TESTING.md](TESTING.md#knuth-plass-performance-baseline); they do not establish
font-backed or international-script performance.

## Remaining Work

- Public freeze/result integration: logical-line materialization, line-local bidi
  ordering, visual-line assembly, width allocation and frozen output already
  exist internally. The public contract and entry point remain separate work;
  see [FREEZE.md](FREEZE.md) and [FROZEN-API.md](FROZEN-API.md).
- Script-sensitive fragment validity beyond UTF-8 and representable byte length.
- Future script-sensitive realization of elastic textboxes. Shaped discretionary
  fragments already retain glyphs, positioning, shaping contexts and original
  source splits; this provenance is not an outstanding linebreaker migration.
- Optional fitness classes and consecutive-discretionary demerits.
- Broader international-script integration coverage.

Path bookkeeping is now indexed by breakpoint rather than repeatedly scanning
the entire state table. Candidate-result and horizon-snapshot buffers now reuse
capacity within a pass. Dead working states are now deleted; widening line-count
bands, predecessor history and tombstones remain memory/performance targets.
Expensive debug arguments and final info dumps are now guarded before formatting;
enabled tracing retains path diagnostics. Boundary-wide decisions make retirement
and final-pass fallback independent of horizon iteration order. Further changes should follow the measured
baseline rather than a representation chosen only on theoretical grounds.

The pre-translation synthetic 1,000-word/150 pt measurements reported about 88 ms
with an empty provider and 844 ms with a productive provider, with approximately
28 MB and 99 MB allocated respectively. Halving ordinary classifications saved
only about 7% and 3% of runtime. Discovery, seed replay, updates and retained
history remain profiling targets. The newer emergency fixtures keep very few
states active; their lower timings do not supersede the broader workloads.
See [TESTING.md](TESTING.md#boundary-local-ordinary-evaluation-reuse) for recorded
conditions and limitations; these figures are not a fresh measurement here.
Penalty translation changes ranking and can change selected paths and discovery
work, so those measurements are historical, not current performance guarantees.

## Code Reading Guide

Start with the package overview in [doc.go](../linebreak/knuthplass/doc.go).
Then follow `BreakParagraph` into `selectParagraphBreaks` in `passes.go` and
`processKnot` in `boundary.go`. The latter separates advancement, discovery and
boundary commit. Read `path.go` for retained predecessor history, not active
state management; retirement deletes working bookkeeping and seeds while keeping
the history needed by collection. `geometry.go` and `evaluation_cache.go` explain
classification and its boundary-local reuse. The graph and Pass2 helper names
are historical; neither implies a second graph representation or a pass-2-only
discovery path.

## Verification

The November 8 review passed `go test ./... -count=1 -timeout=60s`,
`go vet ./...`, and `go test -race ./linebreak/... -count=1 -timeout=60s`.
Package-local K&P statement coverage was 91.6%. Tests cover forced boundaries,
trimming, lazy discretionaries, deterministic collection, retirement, cache
invalidation and emergency-pass behavior, alongside root-package streaming
integration. Coverage and synthetic benchmarks do not establish complete
international-script support, real-font performance or end-to-end typesetting
readiness.
