# Retiring Active States in Knuth-Plass

Diagnosis and plan, November 8, 2026, based on commit `a958c83`. This concerns
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
  - The leading trim is fixed once content has been seen. Before that, the
    line contains no content and cannot be overfull unless the skips alone are.
  - Side skips are the same for every line.

  So `w.Min` at `E'` is at least `w.Min` at `E`, as long as every knot has
  `MinW ≥ 0`.

Hence the line from `S` to any `E' > E` is overfull too, and so is every
discretionary line from `S` that contains all material up to `E` (see below).

**Precondition: `MinW ≥ 0`.** Encoded paragraphs satisfy it: textbox widths are
nonnegative and fixed, glue shrink is at most 100%. Manually constructed or
user-defined knots, or a configured `ParFillSkip`, could violate it. The final
trailer is irrelevant, since it is a forced break and ends the scan. The
breaker checks the precondition once per pass and disables early retirement for
paragraphs that violate it, keeping today's behavior for them.

### Discretionary Lookup

- **Pass 1** has no discretionaries. Retirement is unconditionally safe.
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

### Retroactive Discretionary Targets

Pass-2 lookup can create states at discretionary breakpoints inside a box the
scan has already passed. Such a state's width depends only on its breakpoint
(the PostBreak seed plus the knots after the box), not on its predecessor. If
`(target, line)` has been retired and another origin later offers a transition
to it, the new state would be overfull as well.

Today `evalNewSegmentSeed` asserts that a state exists whenever the predecessor
table has an entry for it. That assertion would fire for a retired state.
Retired states are therefore recorded (a `retired` set of origins), and
`evalNewSegmentSeed` ignores transitions to them instead of asserting.

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
  - in the **final pass**, the retired state with the lowest total cost (ties
    by `compareBreakRef`, then by line count) is connected to `E` with
    `AwfulDemerits` as an overfull line, and `E` becomes active;
  - in **other passes**, the pass fails without a path.
- At a **forced** breakpoint, lines from active states are accepted whatever
  their badness, as today. If no active state remains, the same fallback rule
  applies. A non-final pass fails.

This replaces both the `len(kp.horizon) <= 1` drain check and the connection
of infeasible states at forced breaks.

Pass control in `BreakParagraph` becomes simpler: pass 1 either returns an
acceptable path or fails, and failure (or `worstBadness > PreTolerance`)
starts pass 2. The current check for an overfull pass-1 result
(`totalCost >= AwfulDemerits`) is no longer needed, because pass 1 cannot
create fallback lines.


## Plan

Each step should leave the test suite passing.

### Step 1: Characterization Tests

Add failing tests first:

1. **Short lines without stretch** (Problem 2): `"aaa bbb ccc ddd"` at 200 pt
   with zero side skips yields `[7]`.
2. **Bounded horizon** (Problem 1): after scanning a prefix of the lorem corpus,
   every active state satisfies `w.Min ≤ t`, and the number of active
   breakpoints stays bounded by the number of breakpoints fitting into one
   line (independent of paragraph length).
3. **Tight-line hyphenation without shrink** (Problem 3): a single long word
   overflowing its line with zero side skips is hyphenated in pass 2 when the
   hyphenator offers a fitting variant.
4. **Final-pass fallback**: a paragraph with an unbreakable overlong word
   - yields one overfull line in pass 2,
   - makes a stand-alone pass 1 (`breakParagraphPassRefs(…, false)`) fail.
5. **Forced breaks**: a forced break after an overlong word is reached in the
   final pass, and the result is independent of map iteration order (repeat
   the run several times).
6. **Precondition**: a manually constructed khipu with a negative-`MinW` kern
   breaks exactly as before.

### Step 2: Line Dispositions

Replace `lineInfeasible` by an explicit geometric classification:

- `lineOverfull`: `w.Min > t`. Ratio is reported as below −1 (or −∞ without
  shrink), badness 10000.
- `lineScreenedOut`, `lineAccepted`: as now. A short line without stretch has
  badness 10000 and is screened out unless the tolerance admits it.

`evaluateCandidates` returns the evaluations only; the `canReach` result goes
away. `wantsHyphenation` and `hyphenationCandidateForPass2` treat
`lineOverfull` as a tight line.

### Step 3: Retirement of States

- After the evaluations for `E` (and, in pass 2, the tight-line lookup for
  overfull states), collect overfull states.
- After processing the whole horizon snapshot for `E`, delete each collected
  state from `kp.paths` and `kp.seeds`, add it to the `retired` set, and remove
  breakpoints without remaining states from the horizon.
- `evalNewSegmentSeed` ignores transitions to retired states.
- Check the `MinW ≥ 0` precondition once per pass; without it, skip retirement.

### Step 4: Final-Pass Fallback

- Add a `final` flag to the breaker, set by the pass control.
- Implement the fallback rule described above, using the states retired at
  `E`.
- Remove the `len(kp.horizon) <= 1` drain check and the connection of
  infeasible states at forced breaks.

### Step 5: Pass Control

Simplify `BreakParagraph` as described in [Final-Pass Fallback](#final-pass-fallback).
Keep the structure open for the emergency pass.

### Step 6: Benchmarks

- Report active states per knot as a benchmark metric.
- Add a pass-2 benchmark with a deterministic hyphenator, so the cost of
  discretionary lookups is covered.
- Expected effect at 1,000 words: active states drop from about 27,000 to
  about 270. The remaining growth comes from the widening band of line counts
  per breakpoint (see below).

### Step 7: Documentation

Update the implementation status in [khipu-algorithm.md](khipu-algorithm.md)
(remove the deviations that are fixed) and [KP.md](KP.md).


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
