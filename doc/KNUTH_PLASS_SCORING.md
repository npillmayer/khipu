# Knuth-Plass Scoring: TeX vs Current Implementation

This note compares the demerits and badness calculation described in
[the Knuth–Plass paper](knuthplass.pdf) with the current
[scoring implementation](../linebreak/knuthplass/scoring.go).

This reference describes the supported cost model. Completed migration notes and
unimplemented fitness-class designs are in [the history record](history/SCORING_DESIGNS.md).

## Unicode Boundary Translation

Encoding translates finalized accumulated UAX#14 values into layout penalties:
mandatory boundaries become `MaxMerit`, prohibited boundaries become `MaxPenalty`,
and every finite allowed opportunity becomes zero. The existing hard-separator
precedence correction remains in place. Translation happens only after all rule
contributions have been accumulated, including EOF revisions.

UAX rule weights are not calibrated typographic preferences. Previously an
ordinary space could carry -29 into K&P, yielding `(10 + badness)^2 - 841` with
the default line penalty. This rewarded additional low-badness lines, including
an empty leading-space line in one integration fixture. Neutral ordinary breaks
remove that accidental reward without changing K&P's demerit formula.

Client-assigned knot penalties and discretionary `PreBreak.Penalty` values do
not pass through this translation. Intentional negative preferences remain
valid, so negative total costs are not themselves an error. No configurable
Unicode preference mapping is introduced by this policy.

## Original TeX / Knuth-Plass Model

For each candidate line, the original algorithm conceptually proceeds as follows:

1. Compute the line's natural width, total stretch, and total shrink.
2. Compute the adjustment ratio `r`.
   - If the line is short:
     `r = (target - natural) / stretch`
   - If the line is long:
     `r = (target - natural) / shrink`
3. Compute badness:
   - `b = 100 * |r|^3`
   - cap at an "infinite" or awful badness when necessary
4. Reject the candidate in the current pass if `b` exceeds the active threshold.
   - first pass: `pretolerance`
   - second pass: `tolerance`
5. Compute demerits from badness and penalty.
   - let `q = line_penalty + b`
   - if `p >= 0`: `d = q^2 + p^2`
   - if `-10000 < p < 0`: `d = q^2 - p^2`
   - if `p <= -10000`: `d = q^2`
6. Add extra demerits for special situations.
   - consecutive hyphenated lines
   - hyphen in the final line / paragraph end cases
   - incompatible fitness classes between adjacent lines
7. Choose the path with minimum total demerits.

This is the classical Knuth-Plass / TeX formulation.

## Implemented Cost Model

Scoring separates geometric feasibility, pass screening and path ranking:

1. Compute an adjustment ratio using stretch for short lines and shrink for long
   lines. A line whose minimum width exceeds the target is overfull regardless
   of tolerance.
2. Compute cubic badness from `100 * |r|^3`, capped at 10000. Pass 1 uses
   `PreTolerance`; hyphenating passes use `Tolerance`. Equality with the active
   threshold is accepted. A short line without stretch gets badness 10000.
3. Rank accepted lines with squared line-penalty/badness and knot-penalty terms.
   Accumulated demerits use 64 bits and are not capped to the `±10000` knot-penalty
   sentinel range. First/final-hyphen extras are added where applicable.

The [geometry implementation](../linebreak/knuthplass/geometry.go) supplies the
ratio and screening result; [scoring.go](../linebreak/knuthplass/scoring.go)
computes demerits. Pass selection and emergency capacity are documented in
[the breaker guide](KNUTH+PLASS.md#emergency-pass).

## What Is Still Different

### 1. Forced breaks are handled in control flow, not inside `calcDemerits(...)`

TeX treats `p <= -10000` inside the demerits formula as a special case:

```text
d = q^2
```

The implementation handles forced breaks in boundary control flow,
[finishBoundary()](../linebreak/knuthplass/boundary.go), called by graph
construction. Forced-break penalties are neutralized to `0` before scoring.
Normal transitions must satisfy pass tolerance even at a forced boundary;
only the final pass may supply a fallback when no acceptable transition reaches it.

This keeps the forced-boundary policy separate from `calcDemerits(...)` while
preserving the neutralized penalty's effect on the score.

### 2. Fitness classes are still missing

The original Knuth-Plass algorithm distinguishes fitness classes such as:

- tight
- decent
- loose
- very loose

and adds demerits when adjacent lines jump too far between classes.

The current implementation does not yet model fitness classes.

### 3. Discretionary Costs Are Active; Consecutive Costs Are Deferred

Discretionary evaluation reads `PreBreak.Penalty`, assigned upstream by
`Khipukamayuq`, and passes it into the ordinary squared-penalty calculation.
It does not inject a glyph. The unused linebreaker `HyphenPenalty` field and
its conflicting default of 50 have been removed. The provider's nonzero penalty
takes precedence; otherwise `khipu.Params.Hyphenpenalty` supplies the fallback
(default 10). The modern parameter set has no separate `ExHyphenPenalty`.

`FirstHyphenDemerits` is added when a discretionary line starts at the root.
`FinalHyphenDemerits` is added when a terminal transition starts at a
discretionary breakpoint, penalizing the break before the final line.
These are additive demerits, not increments to the squared penalty.

Consecutive-discretionary demerits remain optional future work. The unused
`DoubleHyphenDemerits` field and its defaults have been removed; a parameter
should return only with an implementation.
The existing predecessor `BreakRef` already identifies whether the preceding
break was discretionary. That fact alone does not require the extra fitness
state discussed in [the fitness proposal](history/SCORING_DESIGNS.md#future-work-fitness-classes).

### 4. Overfull fallback is still a local policy, not a TeX-faithful detail

In the final pass, when the horizon is about to drain or no acceptable line
reaches a mandatory boundary, the breaker inserts a fallback with
`AwfulDemerits`. Earlier passes fail normally and trigger the next pass. Mandatory
boundaries retire earlier origins so they cannot be skipped. Accumulated costs
use 64 bits to accommodate repeated fallbacks. This policy keeps the paragraph
breakable. Fallback selection is boundary-wide, deterministic and includes
terminal hyphen extras; insertion precedes source deletion.
Positive finite `EmergencyStretch` enables a third, final pass after two failed
non-final passes. It adds E only to finite stretch used for badness, demerits and
lazy discovery; shrink and knot capacities remain unchanged. For short lines
with `Max < dimen.Fil`, the ratio is `(target-W)/(Max-W+E)` using wide arithmetic.
Fill/saturated summaries keep the existing numerical ratio. Nonpositive E leaves
pass 2 final; values at or above Fil are rejected before searching.
The constructor default is now zero; the tolerant profile retains 50 BP, now
active. The first complete pass wins without comparing costs across passes.
Reported emergency quality is optimization quality, not physical badness.
Freezing reports over-/underfull outcomes from real capacity, independently of
virtual emergency stretch. See [KNUTH+PLASS.md](KNUTH+PLASS.md#emergency-pass).

This is useful for robustness, but it is not a direct transcription of TeX's paragraph-breaking internals.

## Current Limits And Further Reading

Fitness classes and consecutive-discretionary extra demerits are not implemented.
For pass profiles, discovery, retirement and deterministic fallback selection,
use the [breaker guide](KNUTH+PLASS.md). Physical width allocation and fit outcomes
are specified in [the freezing guide](FREEZE.md); virtual emergency capacity is
not added to that allocation.

Completed arithmetic changes, the removed `linebreak/kp` comparison and the full
unimplemented fitness-class proposal are preserved in
[Scoring History And Fitness-Class Proposal](history/SCORING_DESIGNS.md).
