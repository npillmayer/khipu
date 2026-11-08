# Retiring Active States in Knuth-Plass

Original diagnosis, November 8, 2026, based on commit `a958c83`; implementation
sequence revised after review. The diagnosis below records the pre-change
behavior, not a promise to preserve it. This concerns
`linebreak/knuthplass`. The design it implements is described in
[khipu-algorithm.md](khipu-algorithm.md#knuth-plass); see also [KP.md](KP.md).

## Summary

The breaker decides whether to keep an origin active by asking whether an
adjustment ratio can be computed for its current line. That is the wrong
criterion, and it fails in both directions:

1. **Dead states are kept.** A line that is too long but has *some* shrink
   still yields a ratio (below −1), so its origin stays active forever. About
   99% of the active states in a long paragraph are of this kind.
2. **Live states are retired.** A line that is too short and has *no* stretch
   yields no ratio, so its origin is retired, although later material may
   bring the stretch it needs. With zero side skips, this sets every word on a
   line of its own.

The fix is the retirement rule of TeX: a state is retired exactly when its line
cannot shrink enough (`r < −1`). Retirement must be coordinated with pass-2
discretionary lookup and with the final-pass fallback, which today relies on
dead states being kept.

Merging states that differ only in line count is a separate, later step; see
[Out of Scope](#out-of-scope).


## Terminology

- **Breakpoint**: a `BreakRef`, i.e. a knot index plus an optional
  discretionary variant.
- **State**: a breakpoint together with the number of lines set before it,
  `(bp, line)`. Its working data (`bookkeeping`) lives in
  `kp.paths[bp][line]`, its rescan seed in `kp.seeds[origin{bp, line}]`. The
  predecessor table (`pathTable`) is separate and owns path reconstruction.
- **Horizon**: the set of active breakpoints (`kp.horizon`). A breakpoint is
  active while at least one of its states can still start a line.
- **Line of a state**: a state `(S, l)` always starts line `l+1`, whose target
  width is `LineLength(l+1)`.
- **Retiring** a state means it is no longer extended or evaluated. Its
  predecessor entries remain, so paths already built through it stay valid.


## Diagnosis

### Current Behavior

In `constructBreakpointGraph`, for every active breakpoint `S` and every
permitted breakpoint `E`, `evaluateCandidates(S, …)` evaluates all states of
`S`. It returns `canReach = true` if *any* state of `S` yields an adjustment
ratio (`calcAdjustmentRatio`). If `canReach` is false, `S` is removed from the
horizon, after possibly creating an overfull fallback if the horizon is about
to drain (`len(kp.horizon) <= 1`).

`calcAdjustmentRatio` returns no ratio when the line is short without stretch,
or long without shrink. It returns a ratio below −1 for a line that is too long
for its shrink. `calcBadness` caps such a line at badness 10000, so it is
screened out, but `canReach` is still true.

### Problem 1: Overfull States Are Never Retired

Every line containing an interword space has shrink. Once an origin has seen a
space, it therefore stays active until the next forced break, no matter how far
the scan has moved on.

Measured with the `loremParagraph` benchmark corpus at 150 pt and
`DefaultLayout`, after scanning a prefix (so the final forced break does not
clear the horizon):

| Knots scanned | Active breakpoints | Active states | Of which overfull (`r < −1`) |
|---|---|---|---|
| 250 | 121 | 499 | 456 |
| 500 | 246 | 1,828 | 1,745 |
| 1,000 | 496 | 6,953 | 6,818 |
| 1,999 | 995 | 27,018 | 26,752 |

Almost every breakpoint ever reached stays active; only about 270 states are
alive at the end. Run time for complete paragraphs:

| Words | Time | Stored states |
|---|---|---|
| 250 | 32 ms | 1,881 |
| 1,000 | 0.82 s | 27,282 |
| 4,000 | 27 s | 424,328 |

Every knot extends all active states (`updatePath`) and every permitted
breakpoint evaluates them, so the dead states dominate the work.

**Pass 2 is worse than measured.** The corpus has no hyphenator, so these
numbers only cover pass 1. In pass 2, a dead state is screened out with badness
10000 > `PreTolerance`, so `wantsHyphenation` is true for it at every permitted
breakpoint. Each time, `evaluateDiscretionaryCandidates` rescans knots from the
state's seed (`appendActualKnots`). Discretionary candidates are cached, but the
rescans are not.

The subsequently added `BenchmarkSecondPassHyphenatingLorem` confirms this
workload: the 250-word case took about 17 seconds, and the combined profile
attributed about 85.5% of sampled CPU time cumulatively to `appendActualKnots`.
See [TESTING.md](TESTING.md#hyphenating-second-pass) for the measured baseline,
including repeated policy calls for uncached empty hyphenator responses.

### Problem 2: Short Lines Without Stretch Are Retired

A line ending at a breakpoint has its trailing space trimmed, so a line holding
a single word has no stretch of its own. If the side skips have no stretch
either, no ratio can be computed, and the origin is retired.

Reproduction: `"aaa bbb ccc ddd"`, encoded with `Config{}`, broken at 200 pt
with `DefaultLayout` but `RightSkip` set to zero:

- expected breakpoints: `[7]` (one line)
- actual breakpoints: `[1 3 5 7]` (one word per line)

At the first breakpoint, the root's only state ("aaa") has no ratio, so the
root is retired. The drain fallback then creates an `AwfulDemerits` transition
to that breakpoint, and the same happens for every following word.
`DefaultLayout` hides the problem only because its `RightSkip` has 50 bp of
stretch.

The design document specifies badness 10000 (`inf_bad`) for such lines: they
are bad, not infeasible, and must stay active.

### Problem 3: Overfull Lines Without Shrink Never Ask for Hyphenation

`wantsHyphenation` requires a disposition other than `lineInfeasible`. A line
that is too long and has no shrink (typically a single long word) is
infeasible, so pass 2 never requests discretionaries for it. Only the
loose-line lookup from a *previous* line can then hyphenate that word.

### Problem 4: Forced Breaks Depend on Dead States

At a forced break, every state of every active breakpoint is connected to the
break, infeasible ones with `AwfulDemerits`. This guarantees that forced breaks
are always reached, but only because dead states were kept alive. The drain
fallback (`len(kp.horizon) <= 1`) is checked while the horizon is being
modified during the same loop, so its effect depends on the order of
breakpoints in the horizon snapshot.

All four problems share one cause: retirement is keyed on "a ratio can be
computed" instead of on the geometry of the line.


## Retirement Criterion

For a state `(S, l)` evaluated at breakpoint `E`, let `w` be the effective
width triple of the line from `S` to `E` (after trimming, including both side
skips), and `t = LineLength(l+1)`.

- **Overfull**: `w.Min > t`. The line cannot shrink enough (`r < −1`, or
  `w.W > t` without any shrink). The state is **retired**.
- **Underfull without stretch**: `w.W < t` and no stretch. Badness is 10000.
  The state stays **active**.
- Otherwise the ratio and badness are computed as now. The state stays active.

`w.W == t` gives `r = 0`, also without stretch or shrink.

A breakpoint leaves the horizon when all its states have been retired.

### Why Retirement Is Safe

Consider a retired state `(S, l)`, overfull at `E`. For any later breakpoint
`E' > E`:

- **The target does not change.** It depends only on `l`, so it is the same at
  `E'` for any `LineShape`. Variable line shapes do not affect retirement.
- **The minimum width cannot decrease.** Extending the line from `E` to `E'`
  appends knots to the segment:
  - Content and neutral knots add their `MinW`.
  - A discardable knot is first held in the trailing trim, which is subtracted
    again. When content follows, it is counted with its `MinW`.
  - The leading trim is fixed once content has been seen. Before that,
    retained neutral material and side skips can still contribute width.
  - Side skips are the same for every line.

  So `w.Min` at `E'` is at least `w.Min` at `E`, as long as every knot has
  `MinW ≥ 0`.

Hence the line from `S` to any `E' > E` is overfull too, and so is every
discretionary line from `S` that contains all material up to `E` (see below).

**Precondition: `MinW ≥ 0`.** Ordinary encoded textboxes have nonnegative fixed
widths and glue shrink is at most 100%. Check the complete original sequence,
including the final trailer: a negative trailer could rescue a line retired
earlier. Negative-width input disables early retirement, not the new scoring
rules; identical historical break choices are not guaranteed.

The proof also needs nonnegative discretionary fragments. Cached negative
PreBreak/PostBreak minima disable pruning before scanning. Lazy providers must
return nonnegative minima; a violation returns `ErrIllegalArguments`, not a
fallback. Discovering a negative fragment after deleting states is too late to
disable retirement safely.

The original minimum sum is accumulated in int64. A sum exceeding int32 range
disables pruning. In a pruning pass each fragment minimum must fit half of the
remaining int32 headroom, leaving room for both a post-break seed and pre-break
prefix. Cached fragments outside this conservative bound disable pruning;
lazy violations return `ErrIllegalArguments`. Natural/max-width fill saturation
is unchanged; these guards concern the minimum-width monotonicity proof.

### Discretionary Lookup

- **Pass 1** has no discretionaries. Retirement is safe under the width
  precondition above.
- **Explicit soft hyphens** are evaluated when the scan reaches their box
  (`evaluateDiscretionaryCandidates(horiz_bp, last, last)`). An origin retired
  at a breakpoint before that box is already overfull without the box, so it
  is also overfull with a prefix of it.
- **Loose-line lookup** (next box after `E`) only happens for short lines,
  whose states are not retired.
- **Tight-line lookup** hyphenates the last box already on the line
  (`previousContentBox`). This is the one place where an overfull line can
  still be rescued, and it must happen **before** retirement:
  - At the first breakpoint `E` where `(S, l)` is overfull, pass 2 requests
    discretionaries for the last box `b` before `E` and evaluates them from
    `(S, l)`. Then the state is retired.
  - Nothing is lost later: at any `E' > E`, the last box before `E'` is either
    `b` again (only discardables in between, so the same lines result) or a
    later box, in which case the line contains all of `b` and everything up to
    `E`, and is overfull.

  This also fixes [Problem 3](#problem-3-overfull-lines-without-shrink-never-ask-for-hyphenation):
  the lookup is triggered by the overfull disposition, regardless of whether
  the line has shrink.

One limitation remains and is not caused by retirement: if several boxes lie
between two permitted breakpoints, only the last one is considered by the
tight-line lookup. Retirement makes this final for the retired state.

**Forced-boundary rescue needs its own handling.** For a single long textbox
followed immediately by the terminal fill, `previousContentBox` stops at that
non-discardable fill. Locate the textbox without globally changing discardable
semantics. A discretionary discovered at the forced boundary also creates a
successor outside the current horizon snapshot. Evaluate its already-consumed
post-break remainder through the forced boundary before clearing earlier
origins. Test this without an intervening ordinary space and with internal
forced boundaries, not only at paragraph end.

### Retroactive Discretionary Targets

Pass-2 lookup can create states at discretionary breakpoints inside a box the
scan has already passed. Such a state's width depends only on its breakpoint
(the PostBreak seed plus the knots after the box), not on its predecessor. If
`(target, line)` has been retired and another origin later offers a transition
to it, the new state would be overfull as well.

Today `evalNewSegmentSeed` asserts that a state exists whenever the predecessor
table has an entry for it. That assertion would fire for a retired state.
Retired states are therefore recorded (a `retired` set of origins), and
`evalNewSegmentSeed` ignores transitions to them instead of asserting. Insertion
must report whether the target has a live working state; callers must not add
rejected targets to the horizon unconditionally. A cheaper rejected transition
must not rewrite predecessor history already used by descendants.

This case is rare (it needs two lookups targeting the same box from different
origins, with a retirement in between), but it must not panic.


## Final-Pass Fallback

With retirement, dead states no longer keep forced breaks reachable. The
guarantee comes from the final-pass rule of the design document instead:

- Only the **final pass** may create fallback lines. Today that is pass 2,
  since the emergency pass is not implemented. Later it is pass 2 if
  `EmergencyStretch == 0`, otherwise the emergency pass.
- At each permitted breakpoint `E`, all states of the horizon snapshot are
  evaluated first. Retirements are collected and applied afterwards, so the
  outcome does not depend on the order of the snapshot.
- If after this step the horizon would be empty and `E` was not reached by any
  transition:
  - in the **final pass**, the eligible source with the lowest complete
    fallback transition cost (including terminal hyphen extras; ties
    by `compareBreakRef`, then by line count) is connected to `E` with
    `AwfulDemerits` as an overfull line, and `E` becomes active;
  - in **other passes**, the pass fails without a path.
- At a **forced** breakpoint, acceptable lines reach it normally. If none
  reaches it, the final pass may create an over-/underfull fallback; a non-final
  pass fails. Unlike today's unconditional forced acceptance, this honors the
  pass tolerance and follows the design document.

Choose and insert a fallback before deleting its source, or retain the source's
cost/quality explicitly: current insertion reads `kp.paths[from][line-1]`.
When negative widths disable retirement, fallback eligibility must not depend
on entries actually being deleted. Keep historical predecessors and terminal
working states required for collection. Deferred work must store values/IDs,
not borrowed evaluation slices that subsequent scoring calls overwrite.

This replaces both the `len(kp.horizon) <= 1` drain check and the connection
of infeasible states at forced breaks.

Pass control in `BreakParagraph` becomes simpler: pass 1 either returns an
acceptable path or fails, and failure (or `worstBadness > PreTolerance`)
starts pass 2. The current check for an overfull pass-1 result
(`totalCost >= AwfulDemerits`) is no longer needed, because pass 1 cannot
create fallback lines.

Exhausting a non-final pass is a normal no-solution outcome (`ok=false`), not
an error that causes `BreakParagraph` to return before attempting pass 2.
Provider/input errors must still propagate without being mistaken for a retry.


## Plan

Use three controlled changes. Tests and their implementation land together;
do not leave failing characterization tests in an otherwise finished change.

### Change 1: Shared Classification and Focused Tests

- Share local geometry and tolerance screening between ordinary and
  discretionary candidates. Keep penalty/first-line extras in their existing
  ranking paths.
- `lineOverfull` means `w.Min > t`, independently of tolerance. Its badness is
  10000 and its signed ratio is below -1, or negative infinity without shrink.
- Short lines without stretch have positive infinite ratio and badness 10000;
  they are screened out unless the tolerance admits that badness. Exact fit
  needs no elasticity, and `r = -1` remains geometrically feasible.
- Cover exact fit, both sides of the shrink limit, finite stretch, zero
  capacities, badness saturation and tolerance boundaries. Test the same
  geometry through ordinary and discretionary scoring, including `r = -1.1`
  at tolerance 200: neither path may accept it.
- Do not enable early retirement in this change. Temporarily retain the old
  ratio-availability signal for horizon/fallback control and lazy lookup.
  Thus zero-capacity end-to-end behavior and forced-break acceptance remain
  pending even though local classification is corrected. This compatibility
  bridge is explicitly removed in change 2, not used as a retirement criterion.

### Change 2: Boundary Processing, Retirement and Pass Control

Implement these together: deleting states independently of fallback and pass
control can drain the horizon, lose valid rescue transitions, or trigger asserts.

- Evaluate the horizon snapshot, perform tight-line rescue, and stage
  retirements. Handle newly discovered discretionary successors at forced
  boundaries before deciding whether the boundary is reachable.
- Apply the width precondition and provider contract. Delete overfull working
  states/seeds only after rescue/fallback decisions; preserve predecessors.
  Remove a breakpoint from the horizon only when no live states remain.
- Prevent resurrection through the insertion result contract. Keep fallback
  source data available, and rank fallback transitions deterministically.
- Replace `canReach` and ratio-availability gates with geometric disposition.
  A zero-shrink overfull line must get its rescue opportunity; a zero-stretch
  short line must stay active. Remove the old drain check.
- Add the final-pass flag and normal no-solution outcome. Enforce tolerance
  at forced boundaries in non-final passes; reserve fallback for the final
  pass. Keep pass 2 final until the emergency pass exists.

Required integration cases include:

1. `"aaa bbb ccc ddd"` at 200 pt and zero side skips yields `[7]`.
2. A single overlong word immediately followed by terminal fill is rescued
   by a fitting discretionary in pass 2; its successor reaches the terminus.
3. An unbreakable overlong word fails pass 1 normally and yields a final-pass
   fallback. An underfull forced line above tolerance likewise fails pass 1.
4. Mixed live/dead line counts at one breakpoint, varying line shapes,
   reconstruction through retired predecessors, and rejected resurrection.
5. Internal forced boundaries, multiple fallbacks, canonical ties and terminal
   hyphen extras; no dependency on map iteration order.
6. Negative original widths, including a rescuing negative trailer, opt out of
   pruning. Cached and lazy fragments obey the chosen width-safety contract.

### Change 3: Regression and Performance Verification

- Reuse ordinary and `BenchmarkSecondPassHyphenatingLorem` benchmarks. Add
  precisely defined peak active-state and state-evaluation counts; final
  retained-state totals do not establish pruning effectiveness.
- For the fixed-width lorem workload, examine mature states after complete
  permitted-boundary processing, not mid-update or future lookahead targets.
  Compare against an unpruned reference with the same scoring rules.
- The observed roughly 270 potentially live states out of 27,000 is a baseline
  estimate, not a general bound for arbitrary shapes or discretionaries.
- Update tests that explicitly assert dead working-state retention. Preserve
  path validity and canonical choices; document intended changes caused by
  rejecting overfull candidates and correcting pass/fallback policy.
- Update [KP.md](KP.md), [TESTING.md](TESTING.md), and only the final
  implementation-status section of [khipu-algorithm.md](khipu-algorithm.md).


## Implementation Status

Change 1 is implemented: `geometry.go` provides shared classification used by
ordinary and discretionary scoring. `geometry_test.go` checks the same width
triples through both paths, including seed replay and lookahead, pass thresholds,
zero capacities, and excessive shrink hidden by cubic rounding. Ranking still
uses the existing penalties and first-line discretionary extra.

Change 2 is implemented in `boundary.go`. Each knot first advances existing
origins, then discovers rescue transitions, then scores all mature successors
and commits the boundary. This last phase includes newly discovered post-break
remainders, so terminal and internal forced boundaries can be reached in the
same iteration. Retained boundary glue/kern is bypassed only for last-textbox
discovery; trimming flags are unchanged. Discretionaries on forced textboxes or
at/before their originating textbox are not generated.

Overfull working states/seeds are retired after rescue and fallback decisions;
short zero-stretch origins survive. Forced boundaries discard all working
states except the boundary's own bucket. Predecessors and retirement tombstones
remain, and rejected insertion cannot revive an origin. The collector reads
terminal costs/quality and walks preserved predecessor history. The temporary
`ratioAvailable`/`canReach` compatibility bridge has been removed.

Only pass 2 is final until the emergency pass exists. Non-final exhaustion is
an ordinary no-solution outcome, including forced lines above tolerance.
Fallback selection is boundary-wide and deterministic, includes terminal hyphen
extras, and occurs before source deletion. Provider errors propagate normally.

Discretionary rescans now use an immutable post-break-only seed; the current
working segment separately includes the suffix already consumed after its box.
All existing origins advance before insertion, avoiding double accumulation
when retroactive discovery replaces an existing target. Requested rescue scoring
visits the requesting line state rather than all siblings repeatedly.

`boundary_test.go` covers these contracts, negative/overflow safety, ordinary
drain fallback, provider errors, mixed line-count survival, and reconstruction
after retirement. On rectangular and varying shapes, pruned passes match an
unpruned reference using the same scoring rules, both with and without a lazy
hyphenator, while reducing peak live working states. The original 50-word
regression breakpoints, total costs and worst badness remain unchanged.

Change 3 is complete for the bounded synthetic investigation. Optional private
counters report peak live `(BreakRef, line number)` states and ordinary versus
discretionary local classifications. They include rejected/repeated evaluations
but exclude history and provider requests. Tests verify lifecycle accounting
and unchanged results with instrumentation enabled; public calls leave it off.

The 45-case workload matrix covers three paragraph sizes, narrow/standard/wide
and alternating shapes, internal forced breaks, and disabled/empty/productive
hyphenation. Three repeats per case and representative CPU/allocation profiles
are recorded in [TESTING.md](TESTING.md#work-counters-and-workload-matrix).
The evidence identified repeated empty discovery and duplicate ordinary scoring
as follow-ups. Successful empty discovery is now cached by the paragraph owner;
see [TESTING.md](TESTING.md#successful-empty-discovery-caching) for isolated
before/after measurements. Boundary-local ordinary evaluation reuse is also
implemented: discovery results are reused only for unchanged breakpoint buckets.
Successful working-state writes invalidate the target bucket; new/replaced
successors are rescored before retirement or fallback. Tests compare cached and
uncached working tables/history after every knot and the final selected path.
Font-backed/script-specific profiling remains future work; terminal
retained-state counts are not peak activity.

## Out of Scope

- **Merging line counts.** The roughly 270 remaining states are about a dozen
  breakpoints with about 20 line counts each, and this band widens as the
  paragraph grows. TeX merges states beyond the last special line of
  `\parshape`, where line numbers no longer matter. This requires knowing that
  `LineShape` is uniform from some line on, which the interface cannot tell
  today, and must stay compatible with `\looseness` and fitness classes.
- **Emergency pass.** The final-pass flag prepares for it.
- **Fitness classes and adjacent-line demerits.**
- **Over-/underfull handling in freezing** and reporting.
- **First-fit**, which has its own overfull handling.
