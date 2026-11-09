# Knuth-Plass Line Breaker

Reference for `linebreak/knuthplass`. It describes the contract, the algorithm,
its parameters, the design decisions behind it and its known limits. The
paragraph-level design is in [KHIPU_ALGORITHM.md](KHIPU_ALGORITHM.md#knuth-plass);
cost formulas are detailed in [DEMERITS.md](DEMERITS.md), discretionary
discovery in [HYPHENATION.md](HYPHENATION.md). For a short orientation, start
with the package documentation in [doc.go](../linebreak/knuthplass/doc.go).


## Overview

The breaker chooses line endings for one encoded paragraph (`khipu.Khipu`) by
minimizing accumulated demerits, in the style of TeX. It works on indexed
knots in logical order, runs up to three passes, and discovers hyphenation
alternatives lazily through a provider.

It neither shapes text nor assigns final widths. Its result is a sequence of
breakpoints, plus the discretionary variants chosen at some of them. Freezing
turns that result into fixed-width lines.

Here, "optimal" means minimal total demerits among the candidates the search
explores in the first pass that succeeds. Hyphenation is discovered locally and
lazily, so not every possible hyphenation is considered (see
[Lazy Discretionaries](#lazy-discretionaries)).


## API

```go
func BreakParagraph(khp *khipu.Khipu, layout khipu.Params, params *Parameters) ([]int, error)
func DefaultLayout(shape khipu.LineShape) khipu.Params
func NewDefaultParameters() *Parameters
func NewTolerantParameters() *Parameters
```

**Input.** A paragraph produced by `Khipukamayuq.EncodeParagraph`, or an
equivalent hand-built `Khipu`. Its last knot must be the paragraph fill: a
non-discardable glue knot carrying the mandatory-break penalty
`InfinityMerits`. The breaker identifies the paragraph end by this last knot;
it never appends a trailer itself.

**Geometry.** Line shape and side skips come from `layout` (`khipu.Params`).
`DefaultLayout(shape)` sets a zero left skip and a zero-width right skip with
50 bp finite stretch. Callers apply overrides afterwards (explicit zeros are
kept) and must pass the same `Params` to freezing.

**Algorithm parameters.** `params` holds tolerances, demerit settings, emergency
stretch and the optional `DiscretionaryProvider`. `nil` selects
`NewDefaultParameters()`. See [Parameters](#parameters).

**Result.** Zero-based indices of line-ending knots, the last one being the
paragraph fill. Discretionary choices are not part of the indices: they are
stored in `Khipu.SelectedDiscretionaries`, keyed by the knot whose textbox is
split.

**Ownership and side effects.**
- A successful call replaces `SelectedDiscretionaries` with the choices of the
  winning path, and clears it if none were chosen.
- A failed call leaves earlier selections untouched. Discovery may still have
  added candidates to the paragraph's discretionary cache; they stay valid.
- Errors from the provider or from invalid input are returned unchanged and are
  not logged by the breaker. They abort the call; no further pass is tried.
- `ErrNoBreakpoints` means that no pass produced a complete path.
- A `Khipu` must not be broken concurrently, and its owner must not be used
  concurrently while breaking.


## Paragraph Representation

The breaker reads the struct-of-arrays planes `W`, `MinW`, `MaxW`, `Penalty`,
`Kind` and `Flags`. A knot's elasticity is the triple (`MinW`, `W`, `MaxW`);
infinite stretch is encoded by fill sentinels in `MaxW`.

A **penalty** belongs to the knot it follows: it is the cost of breaking after
that knot. `InfinityDemerits` (10000) prohibits a break, `InfinityMerits`
(−10000) forces one. The encoder maps every allowed Unicode line-break
opportunity to penalty 0; only mandatory and prohibited boundaries keep their
sentinels (see [DEMERITS.md](DEMERITS.md#unicode-boundary-translation)). Other
penalty values come from hand-built knots or from discretionary fragments.

A **breakpoint** is a `BreakRef{At, Variant}`. Variant 0 is an ordinary break
after knot `At`. A nonzero variant is a break inside the textbox at `At`, using
one of its discretionary alternatives.

**Trimming.** The `KFDiscardable` flag, not the knot kind, decides what is
removed at line edges. For each line the breaker tracks the total width, the
discardable width before the first content (leading trim), the discardable
width after the last content (trailing trim), and whether content has been seen.
The effective line width is total minus both trims plus both side skips.
Discardables between content stay in the line. The paragraph fill must not be
flagged discardable; there is no exception for it.

Width sums saturate instead of overflowing, so fill plus finite material stays
representable.


## Algorithm

### States and Paths

A **state** is a breakpoint together with the number of lines completed before
it: `(BreakRef, line)`. Different line counts at the same breakpoint are
different states, since the next line's target width depends on its number.

The search keeps, for every state:
- **working bookkeeping**: the accumulated path cost, the worst line badness on
  the path, and the width bookkeeping of the line currently being extended;
- a **rescan seed** for discretionary evaluation (see below);
- a **predecessor**: the best known transition into the state.

Working bookkeeping is grouped by breakpoint
(`map[BreakRef]map[lineNo]bookkeeping`), so updates and scoring visit only the
states of one breakpoint. The set of breakpoints that still have working states
is the **horizon** (TeX's active list).

Predecessors live in a separate table keyed by state (`path.go`). A strictly
cheaper transition replaces the stored one. Equal costs prefer the lower
predecessor knot index, then the lower variant, so results do not depend on
map iteration or arrival order.

### Processing a Knot

Each knot is processed in three phases (`processKnot` in `boundary.go`):

1. **Advance.** The knot is appended to the line bookkeeping of every working
   state. This happens before any new state is inserted, so no state can
   accumulate the same knot twice.
2. **Discover** (passes 2 and 3 only). Discretionary alternatives are requested
   and evaluated; accepted alternatives create states at discretionary
   breakpoints (see [Lazy Discretionaries](#lazy-discretionaries)).
3. **Commit** (only if a break is permitted after the knot). Every working
   state, including states created in phase 2, is evaluated as the start of a
   line ending here. Acceptable lines become transitions to the new state
   `(here, line+1)`. Then states are retired and, where applicable, a fallback
   is created (`finishBoundary`).

Evaluation results of phase 2 are reused in phase 3 for breakpoints whose
working states did not change in between (`evaluation_cache.go`). Any change to
a breakpoint's working states invalidates its cached results.

### Line Evaluation

All candidate lines, ordinary and discretionary, are classified by one function
(`classifyLine` in `geometry.go`). With effective width `w` (after trimming,
including side skips) and target `t = LineLength(line+1)`:

| Case | Condition | Ratio | Badness | Disposition |
|---|---|---|---|---|
| Overfull | `w.Min > t` | below −1, or −∞ without shrink | 10000 | overfull |
| Exact fit | `w.W = t` | 0 | 0 | screened by tolerance |
| Long | `w.W > t ≥ w.Min` | `(t − w.W) / (w.W − w.Min)` | `100·|r|³` | screened by tolerance |
| Short | `w.W < t` | `(t − w.W) / (w.Max − w.W)`, +∞ without stretch | `100·|r|³`, max 10000 | screened by tolerance |

Badness is capped at 10000. A line whose badness is within the pass's
tolerance is **accepted**; otherwise it is **screened out**. Overfull lines are
never accepted, whatever the tolerance.

A short line without stretch is bad (badness 10000) but not overfull: later
material may still bring the stretch it needs.

Accepted lines get demerits `(LinePenalty + b)² + p²` for a positive penalty
`p`, or `(LinePenalty + b)² − p²` for a negative finite one. Forced breaks count
with `p = 0`. Discretionary breaks use the penalty of their `PreBreak` fragment
and add `FirstHyphenDemerits` if they end the first line. A transition into the
paragraph end adds `FinalHyphenDemerits` if the last line starts after a
discretionary break. Costs are 64-bit, so repeated fallback demerits cannot
wrap around. See [DEMERITS.md](DEMERITS.md).

### Retirement

A state is **retired** when its current line is overfull: it is no longer
extended or evaluated. Its working bookkeeping and seed are deleted; its
predecessor entry stays, so paths already built through it remain valid. A
tombstone prevents a later discretionary discovery from recreating it. A
breakpoint leaves the horizon when none of its states remains.

Retirement is applied after the whole commit phase of a boundary, so the
outcome does not depend on the order in which states are evaluated. In passes
2 and 3, an overfull state first gets its chance at
[tight-line hyphenation](#lazy-discretionaries).

**Why this is safe.** For a state `(S, l)` that is overfull at breakpoint `E`,
every later line from that state is overfull too:
- its target `LineLength(l+1)` does not depend on where the line ends, so this
  holds for any line shape;
- its minimum width can only grow as knots are appended, provided all minimum
  widths are nonnegative (discardables are either trimmed or counted with
  their `MinW` once content follows).

A tight-line hyphenation can only rescue the state at the first overfull
boundary: at any later boundary the line contains all material up to `E`.

**Preconditions.** Retirement is disabled for the pass (states are then kept
until the next forced break) if
- any knot, including the paragraph fill, has a negative `MinW`,
- the sum of all minimum widths exceeds the 32-bit range, or
- a cached discretionary fragment has a negative or unsafely large `MinW`.

Fragments returned lazily by a provider must have nonnegative `MinW` and, while
retirement is enabled, fit half the remaining 32-bit headroom. Violations are
reported as `ErrIllegalArguments`: once states have been deleted, retirement
can no longer be switched off safely.

**Forced breaks.** At a forced break, all working states except those at the
forced breakpoint itself are retired, so no later line can bypass it.

### Passes and Fallback

| Pass | Discretionaries | Tolerance | Emergency stretch | Final |
|---|---|---|---|---|
| 1 | no | `PreTolerance` | 0 | no |
| 2 | yes | `Tolerance` | 0 | if `EmergencyStretch <= 0` |
| 3 | yes | `Tolerance` | `EmergencyStretch` | yes; runs only if `EmergencyStretch > 0` |

Each pass starts from scratch: fresh states, predecessors and evaluation
caches. Only the paragraph's discretionary cache carries over. The first pass
that completes a path wins; costs from different passes are never compared.

A **non-final** pass enforces its tolerance everywhere, including at forced
breaks. If it runs out of states, it ends without a path. This is a normal
outcome that starts the next pass.

Only the **final** pass may create **fallback** lines. At a boundary where the
horizon would otherwise become empty, or at a forced break that no acceptable
line reaches, the final pass connects the cheapest candidate (including
terminal hyphen demerits; ties by breakpoint order, then fewer lines) with
`AwfulDemerits`. The fallback is chosen before its source state is retired.
Such lines are overfull or underfull; freezing them is a separate concern.

**Collection.** After the last knot, the cheapest state at the paragraph end
wins (equal costs prefer fewer lines). Its predecessors are followed back to
the start and the sequence is reversed. Only that single path is returned.

### Emergency Pass

The emergency pass prefers loose lines over overfull fallback lines. It adds
the virtual stretch `E = EmergencyStretch` to every short line's finite stretch
when computing badness: `100·(s/(Y+E))³` instead of `100·(s/Y)³`. Everything
else is as in pass 2: shrink, overfull detection, retirement, tolerance,
penalties and demerits are unchanged.

`E` affects the choice of breakpoints only. Knots are not changed, and freezing
uses the real stretch, so lines chosen thanks to `E` are usually underfull in
physical terms.

Lines whose summed maximum width already reaches the fill range
(`w.Max >= dimen.Fil`) are evaluated without `E`. The width summary cannot tell
real fill from a saturated finite sum, so this keeps the existing numerical
behavior rather than claiming exact fill semantics.

### Uniform-Shape Compaction

For the built-in `linebreak.RectangularParShape`, every line has the same
target. At an ordinary breakpoint, states with different line counts then face
identical futures, so only the cheapest one can matter. After a boundary has
been committed, the breaker keeps only its cheapest state (fewer lines on equal
cost) and retires the others (`uniform.go`). This keeps the number of states
per breakpoint constant instead of growing with the paragraph.

Restrictions:
- The shape is recognized by its type. Custom shapes, even constant ones or
  wrappers around a rectangle, use the general algorithm.
- The root and discretionary breakpoints are not compacted. Discretionary
  breakpoints can receive improved predecessors retroactively, after
  descendants exist.
- All compacted states have completed at least one line, so first-line
  demerits are already charged. Any future scoring that depends on the line
  number must revisit this dominance argument.


## Lazy Discretionaries

Discretionary alternatives (hyphenations) are supplied by a
`DiscretionaryProvider`, normally the paragraph's `Khipukamayuq`. The provider
shapes the fragments, assigns their penalties and caches the results on the
`Khipu`, including "no alternatives" answers. Without a provider, the breaker
uses only what is already cached.

Passes 2 and 3 ask for alternatives in three situations:

- **Explicit markers.** When the scan reaches a textbox containing a soft
  hyphen (U+00AD), its alternatives are requested, independently of badness.
- **Cached alternatives at the scanned box.** When the scan reaches a textbox
  with cached alternatives, they are evaluated from every working state.
- **Bad lines.** For a line that is overfull or whose badness exceeds
  `PreTolerance`, one textbox is chosen:
  - a loose line asks for the next textbox after the breakpoint, to pull part
    of it onto the line;
  - a tight or overfull line asks for the last textbox already on the line, to
    push part of it to the next line.

  Textboxes followed by a forced break, and boxes at or before the line's start,
  are not considered. Pass 3 uses its emergency-adjusted badness for this
  trigger.

An alternative is evaluated as a line from the requesting state: the material
up to the textbox, followed by the `PreBreak` fragment. An accepted
alternative creates a state at `BreakRef{box, variant}` whose next line starts
with the `PostBreak` fragment. A loose-line alternative lies ahead of the scan
and becomes active only once the scan has passed its textbox. The rescan seed
of a discretionary state holds only the `PostBreak` fragment; material already
scanned after the textbox is part of its working bookkeeping.

This lookup is a deliberate deviation from TeX, which hyphenates every word in
pass 2. Shaping alternatives is costly, so the breaker only asks where a line
suggests it might help. It can miss solutions that hyphenate a different word,
and only the last textbox is considered when several lie between two
breakpoints.


## Parameters

| Setting | Meaning |
|---|---|
| `PreTolerance` | badness limit in pass 1 |
| `Tolerance` | badness limit in passes 2 and 3 |
| `LinePenalty` | added to every line's badness before squaring |
| `FirstHyphenDemerits` | extra demerits if the first line ends at a discretionary break |
| `FinalHyphenDemerits` | extra demerits if the last line starts after a discretionary break |
| `EmergencyStretch` | virtual stretch for pass 3; `<= 0` disables pass 3, `>= dimen.Fil` is rejected before pass 1 |
| `ParFillSkip` | dimensions for the paragraph fill, for use by the encoder (`khipu.Config.ParFillSkip`); not read while breaking |
| `DiscretionaryProvider` | source of discretionary alternatives; optional |

The hyphen penalty is not a breaker parameter. It comes from the fragment
(`PreBreak.Penalty`), or from `khipu.Params.Hyphenpenalty` when the provider
supplies none.

### Parameter Profiles

Both constructors return independent values; changing one never affects later
calls.

| Setting | `NewDefaultParameters()` | `NewTolerantParameters()` |
| --- | ---: | ---: |
| PreTolerance | 100 | 100 |
| Tolerance | 200 | 5000 |
| LinePenalty | 10 | 10 |
| FirstHyphenDemerits | 0 | 0 |
| FinalHyphenDemerits | 10000 | 50 |
| EmergencyStretch | 0 | 50 bp |
| Final pass | 2 | 3 |

Neither profile installs a provider. `nil` parameters mean
`NewDefaultParameters()`.


## Design Decisions

**Predecessor table instead of a graph.** The search only needs, for each
state, its best incoming transition, plus the ability to replace it with a
cheaper one. A general graph with edge deletion was replaced by a table keyed
by state. Distinct discretionary variants at one knot remain distinct states.

**Discretionary data inside the `Khipu`.** Candidate alternatives and selected
variants are stored in sparse tables on the paragraph, not in a separate
breaker view or external cache. Chosen variants must survive into freezing,
and other passes or breakers can reuse discovered candidates.

**Flags decide discardability.** Trimming reads `KFDiscardable` instead of
inferring discardability from knot kinds. Script-sensitive flag policies belong
to the encoder.

**Allowed breaks cost nothing.** Finite UAX#14 values express "allowed", not a
typographic preference. Read as TeX penalties, their squares rewarded extra
lines. The encoder therefore maps them to 0.

**Retirement by geometry.** A state is retired exactly when its line is
overfull, the criterion TeX uses for deactivating breakpoints. Earlier criteria
based on whether an adjustment ratio could be computed kept dead states alive
and retired live ones.

**Fallback only in the final pass.** Non-final passes fail cleanly, so a later
pass gets its chance before overfull lines are accepted. The fallback is chosen
over the whole boundary, deterministically, and before its source is retired.

**Emergency pass is opt-in.** The default profile disables it, so `nil`
parameters keep the two-pass behavior. Pass 3 uses its emergency-adjusted
badness for the hyphenation trigger as well, so a line made acceptable by `E`
does not cause further shaping work. Overfull lines and explicit markers still
trigger discovery. The internal pass result keeps the pass identity and its
quality; the quality reflects virtual stretch, not physical line quality.

**Compaction only where dominance is certain.** Merging line counts is exact
only when the line shape is known to be uniform. The `LineShape` interface
cannot state this, so only the built-in rectangle qualifies. Sampling widths
cannot prove that future lines are uniform.


## Known Limits and Deviations from TeX

- **No fitness classes**, hence no `\adjdemerits`. Adding them would refine
  states and interact with retirement and compaction; see
  [DEMERITS.md](DEMERITS.md).
- **No demerits for consecutive hyphenated lines** (`\doublehyphendemerits`).
  `FirstHyphenDemerits` has no TeX counterpart.
- **Lazy, local hyphenation** as described above, rather than hyphenating every
  word in pass 2.
- **Fill orders are approximated numerically.** Width summaries do not separate
  finite stretch from fil, fill and filll.
- **Uniform-shape compaction only for the built-in rectangle.** Custom shapes,
  including shapes that differ only in their first lines (TeX's
  `\parshape`/`\hangindent` with uniform tail), keep all line counts. An
  explicit "uniform from line N" capability on `LineShape` would remove this
  restriction.
- **Discretionary breakpoints keep all their line counts.** In pass 2 and 3
  with productive hyphenation, the number of states still grows with the
  paragraph.
- **No `\looseness`** and no enumeration of alternative solutions.
- **Pass 1 always runs**, even with a negative `PreTolerance` (TeX skips it).
  It then simply fails.
- **Path quality is internal.** Callers cannot see which pass succeeded or how
  bad its lines are; reporting of over- and underfull lines is an open design
  question (see [KHIPU_ALGORITHM.md](KHIPU_ALGORITHM.md#reporting-over--and-underfull-lines)).
- **Retirement depends on nonnegative minimum widths.** Paragraphs with
  negative widths are broken correctly but more slowly.


## Performance Characteristics

- Without discretionaries and with a rectangular shape, the work per knot is
  bounded by the number of breakpoints that fit into one line, so breaking is
  linear in the paragraph length.
- With a custom line shape, each breakpoint keeps one state per reachable line
  count. That band widens with the paragraph, making the search superlinear.
- In passes 2 and 3, discretionary evaluation rescans the material between a
  state's start and the textbox. Provider calls and fragment shaping usually
  dominate the cost of productive hyphenation.
- Paragraphs that pass 1 cannot set run two or three complete passes.
- Predecessor entries and tombstones are kept until the pass ends.

Benchmarks and measurements are described in
[PERFORMANCE.md](PERFORMANCE.md).


## Code Reading Guide

| File | Contents |
|---|---|
| `doc.go` | package overview |
| `knuthplass.go` | `BreakParagraph`, bookkeeping, demerits, collection |
| `passes.go` | pass sequence and pass configuration |
| `boundary.go` | per-knot phases, commit, retirement, fallback |
| `geometry.go` | shared line classification, emergency stretch |
| `uniform.go` | uniform-shape compaction |
| `discretionary.go` | provider access, lookup heuristics, discretionary evaluation |
| `evaluation_cache.go` | reuse of evaluations within one boundary |
| `path.go` | `BreakRef`, predecessor table |
| `definitions.go` | `Parameters`, profiles, constants |
| `metrics.go` | optional work counters for tests and benchmarks |

Start with `BreakParagraph`, follow it into `selectParagraphBreaks`
(`passes.go`) and then into `processKnot` (`boundary.go`).


## Tests

The package tests cover, among others:
- line classification at all geometric boundaries, shared by ordinary and
  discretionary lines, with and without emergency stretch;
- retirement, its preconditions, tombstones and reconstruction through retired
  states, compared against a reference run without retirement;
- pass control, final-pass fallback, forced breaks and deterministic ties;
- lazy discovery, provider errors and failure-atomic publication of
  selections;
- uniform-shape compaction against the general algorithm, on rectangular,
  alternating and opaque constant shapes;
- evaluation-cache equivalence with the uncached path.

Integration tests in the root package run encoding, breaking and later stages
together. The tests use synthetic shaping; they do not establish behavior with
real fonts or across scripts.
