# Knuth-Plass Emergency Pass

## Status and Purpose

Stages 1-4 (shared emergency evaluation, pass control, regression coverage and
targeted benchmarks) are implemented. Positive finite `EmergencyStretch` enables a third pass;
nonpositive values retain two-pass behavior. The constructor default is zero,
while the tolerant profile retains 50 bp. The design decisions below describe
the implemented policy. Benchmark baselines and scope limitations are recorded
in [TESTING.md](TESTING.md#emergency-pass-workloads). Freezing still uses only real capacities.

The algorithmic intent is defined in
[khipu-algorithm.md](khipu-algorithm.md#knuth-plass). That document remains the
design authority, with current implementation status restricted to its final
section. Related background is in [KP.md](KP.md), [DEMERITS.md](DEMERITS.md),
[HYPHENATION.md](HYPHENATION.md), and [FREEZE.md](FREEZE.md).

The emergency pass should prefer loose lines over overfull recovery lines by
adding virtual finite stretch for breakpoint evaluation only. It does not
increase real knot capacities, relax shrink limits, or implement underfull
freezing.

## Current Foundation

- `classifyLine` in `linebreak/knuthplass/geometry.go` provides shared geometry
  and tolerance screening for ordinary and discretionary candidates, including
  an explicit virtual-stretch input. Both routes use the same private
  pass-local `emergencyStretch` field, zero in passes 1 and 2.
- `breakParagraphPassRefs` constructs a fresh breaker from an explicit
  `passConfig`. `selectParagraphBreaks` runs the sequence and returns the first
  complete `passResult`, without publishing intermediate selections.
- `finishBoundary` already restricts fallback transitions to the final pass.
  Non-final exhaustion is normal no-solution control flow, not an error.
- Retirement depends on effective minimum width exceeding the line target.
  Adding stretch does not change that condition or its existing safety checks.
- The paragraph owner caches successful positive and empty discretionary
  discovery results. Errors are not cached. These caches survive breaker passes.
- Boundary-local ordinary evaluation reuse belongs to an individual breaker
  and boundary. A fresh pass cannot accidentally reuse another pass's scores.
- `NewKPDefaultParameters` specifies zero emergency stretch;
  the explicitly tolerant `DefaultParameters` profile specifies 50 bp.
  The latter now enables emergency recovery when earlier passes fail.
- Public `BreakParagraph` returns breakpoint indices and an error. It does not
  expose pass identity or the private `pathQuality` value.

## Design Decisions

### 1. Defaults and Parameter Validation

Implemented policy:

- Set `NewKPDefaultParameters().EmergencyStretch` to zero. This preserves the
  existing two-pass behavior for callers supplying nil algorithm parameters.
- Retain 50 bp in the explicitly tolerant `DefaultParameters` profile. Its
  nonzero value would become an intentional recovery policy rather than an
  unused setting.
- Treat `EmergencyStretch <= 0` as disabled; use an effective value of zero.
- Reject `EmergencyStretch >= dimen.Fil` with `ErrIllegalArguments`. Emergency
  stretch must be a finite dimension, not a fill-order sentinel.
- Validate before starting pass 1, even when that pass could succeed without
  using emergency stretch.

Enabling the feature changes recovery only when the earlier passes do not find
an acceptable complete path. It does not change every paragraph. Callers using
the tolerant profile must nevertheless be told that its recovery behavior and
runtime can change.

The previous constructor value of 20 bp was unused. It was changed to zero
instead of silently activating emergency recovery for default/nil callers.

### 2. Hyphenation Discovery During Pass 3

Implemented: use emergency-adjusted badness for the existing lazy discovery
trigger as well as for screening and demerit calculation.

This keeps one local evaluation model. A loose line made acceptable by virtual
stretch may no longer justify additional shaping work. Overfull states still
receive their discretionary rescue opportunity: the existing trigger tests
overfull disposition independently of the badness threshold. Explicit soft
hyphens must remain requested when their textbox is reached, independently of
the badness trigger.

This is a policy choice for khipu, not literal adherence to TeX: our lazy,
badness-driven discovery already differs from the eager discovery described in
the design document.

Alternative: use real badness for discovery and emergency-adjusted badness for
screening/ranking. This deliberately continues looking for discretionaries even
when emergency scoring already accepts the line. It needs an additional
computation and a separate value or explicitly separate trigger calculation.
It cannot guarantee the same candidate coverage as pass 2, since the passes
may explore different states and reach different parts of the paragraph.

Pass 3 reuses previously queried positive and empty results, but it can still
make new provider requests. Pass 2 may have stopped early or never requested a
particular textbox. New provider errors must propagate normally.

### 3. Reporting

Implemented: `passResult` retains pass identity, effective emergency stretch,
completion and quality. Successful selection logs the pass, stretch, worst
badness and cost at debug level. The public return type is unchanged and no
public Khipu flag is added.

`pathQuality.worstBadness` describes optimization badness under the selected
pass's assumptions. For pass 3, it is not the physical badness computed from
real capacities. The same qualification applies to accumulated demerits.

An emergency-pass result is not proof that every selected line is underfull.
Freezing must determine actual insufficient capacity from real widths. A future
public diagnostic result can report pass identity and physical quality as
distinct facts. It should not derive one from the other.

### 4. Fill Representation and Compatibility

Agreed and implemented: preserve existing fill behavior rather than silently
introducing a new exact fill-order model.

The original proposal assumed that infinite stretch already receives exactly
zero badness. The current implementation does not explicitly guarantee that:
`calcAdjustmentRatio` treats `WSS.Max` numerically. `WSS` stores saturated width
totals, not separate finite and symbolic stretch components. A saturated total
also cannot reliably distinguish real fill from a sufficiently large sum of
finite maxima.

For genuine infinite stretch, adding finite emergency stretch should have no
effect. Implementing that distinction exactly would require richer bookkeeping,
including correct trimming of fill-bearing items. That is a separate scope
decision, not a safe inference from `Max >= dimen.Fil` alone.

The compatibility rule is explicit: positive E augments stretch only for short
lines with `WSS.Max < dimen.Fil`. Summaries with `Max >= dimen.Fil` retain the
existing numerical calculation unchanged, including saturated finite summaries.
This is an approximation imposed by the current representation, not exact fill
semantics. Exact fits, shrink and nonpositive E retain the original calculation.

## Implemented Pass Sequence

The internal `passConfig` separates finality from the hyphenation boolean. It
carries pass identity, whether discovery is enabled, whether fallback is
permitted, and effective emergency stretch. These are not new public parameters.

| Pass | Discretionary discovery | Screening limit | Virtual stretch | Final |
| --- | --- | --- | --- | --- |
| 1 | Disabled | PreTolerance | 0 | No |
| 2 | Enabled | Tolerance | 0 | Only when emergency stretch is disabled |
| 3 | Enabled | Tolerance | E > 0 | Yes |

```text
validate layout and algorithm parameters
run pass 1 with a fresh breaker
if error: return error
if acceptable complete path: publish its selections and return
run pass 2 with a fresh breaker
if error: return error
if complete path: publish its selections and return
if emergency stretch is disabled: return no-breakpoints error
run pass 3 with a fresh breaker and emergency stretch E
if error: return error
if no complete path: return no-breakpoints error
publish pass 3 selections and return
```

With emergency stretch enabled, pass 2 is non-final and must not manufacture
fallback transitions. Its successful result is therefore acceptable under
Tolerance. With emergency stretch disabled, pass 2 retains today's final-pass
fallback behavior.

The first successful pass wins. Compare complete paths within that pass; do not
compare demerits across passes using different virtual capacities. Keep selected
discretionary publication at the successful public-call boundary, not at the end
of a failed intermediate pass.

## Implementation Stages

### Stage 1: Shared Emergency Evaluation (Implemented)

The shared classifier takes an explicit emergency-stretch input. Both ordinary
and discretionary scoring pass the same private breaker field into the pure
calculation. Passes 1 and 2 leave it zero; Stage 2 supplies positive E to pass 3.

For a short line with finite real stretch Y and shortfall s:

```text
ratio = s / (Y + E)
badness = existing capped and rounded cubic badness(ratio)
```

- Compute subtraction and addition in `int64`, not merely the final addition.
  Convert the wide numerator and denominator to floating point for the ratio;
  do not narrow or clamp the temporary denominator back into `dimen.DU`.
  Only the separately stored adjustment magnitude saturates if it exceeds DU's
  range; it does not affect the ratio or badness.
- Keep the existing badness cap and rounding. Positive E does not guarantee
  badness below Tolerance or even below the existing cap.
- Leave exact fits, shrink, minimum widths, and overfull classification unchanged.
- Keep E out of `WSS`, knot planes, segment/rescan seeds, and shaped fragments.
- Preserve existing E=0 results, including extreme summaries, and apply the
  fill/saturation compatibility rule above when E is positive.

Emergency stretch is constant for a whole breaker pass. Boundary evaluation
reuse therefore needs no additional cross-pass cache key: every pass constructs
its own breaker and cache. Existing within-boundary invalidation remains required.

`emergency_geometry_test.go` covers finite and zero real capacity, tolerance and
badness caps, exact fits, shrink and overfull lines, fill/saturated summaries,
wide subtraction/addition, ordinary/discretionary agreement (lookahead and seed
replay), repeated cached scoring, and unchanged knots/fragments/seeds. It also
checks nonpositive-E equivalence and that only explicit pass configuration can
activate virtual capacity: passes 1 and 2 still score with real capacity alone.

### Stage 2: Pass Control and Internal Result (Implemented)

`passes.go` builds and validates the pass sequence, runs each pass with fresh
state and retains internal result context. `final = hyphenating` has been
replaced by explicit configuration. Pass 3 runs only after non-final pass 2
reports no solution and E is positive.

Ordinary no-solution remains distinct from operational errors. Costs are not
used to infer pass failure. Provider errors never trigger a later pass.
Retirement, fallback ranking and predecessor collection are unchanged.

Individual-pass tests now specify finality and E explicitly. The public call
publishes only the selected result; failed searches leave existing selections
unchanged. No comparison of costs across passes is performed.

`emergency_pass_test.go` covers defaults, disabled and invalid E, validation
before a potentially successful first pass, loose recovery, insufficient E,
unavoidable overfull fallback, first-success ordering even with high costs,
unchanged width planes, errors in passes 2 and 3 without retries, new discovery
after early pass-2 exhaustion, and publication of an emergency-selected variant.

### Stage 3: Tests (Implemented)

The classifier and focused control-flow tests cover items 1-5 and the initial
error/publication scenarios. The broader regression suite now covers items 6-8:

- `emergency_discovery_test.go` runs owned synthetic paragraphs through failed
  passes 1/2 and successful pass 3. Automatic and explicit requests each exercise
  positive and empty responses. Pass 3 re-requests the same textbox without new
  policy calls or shaping streams; a warm whole-call rerun preserves pass,
  quality and path. Original knot cores and cached fragments stay unchanged,
  and only the public successful call replaces selected discretionaries.
- Automatic discovery is tested with real-only capacity, emergency-adjusted
  badness equal to PreTolerance, above PreTolerance but within Tolerance, still
  screened out, and overfull geometry. An explicit marker inside an otherwise
  forbidden-break textbox is requested and made selectable even when ordinary
  badness does not justify automatic lookup.
- `evaluation_cache_test.go` compares cached and uncached state after every knot
  across rectangular, varying and overlong-line shapes, forced/ordinary
  boundaries and empty/productive providers, now with active E as well. Paths,
  predecessor history, rescan seeds, retirement, horizon, discovered candidates,
  final quality and work counters agree apart from the intended scoring reuse.
  Replacement/new-sibling writes and forced-boundary new/replaced successors
  are also checked with geometry that is acceptable only through virtual E.
- `emergency_stability_test.go` alternates the arrival order of equal-cost
  variants over 20 fresh searches and three repeated whole-call searches per
  paragraph, checking canonical selection, quality, pass identity and unchanged
  candidate storage. A retroactive discovery fixture checks consumed suffix
  bookkeeping and subsequent rescan: virtual stretch never enters physical
  post-break seeds, in either cached or uncached mode.

The complete coverage checklist is:

1. **Disabled emergency pass:** zero and negative E retain two-pass behavior;
   pass 2 remains final. Existing E=0 paths, costs, and selections are unchanged.
2. **Parameter validation:** reject symbolic/invalid positive E before any pass,
   including paragraphs that would otherwise succeed immediately in pass 1.
3. **Shared classification:** exercise ordinary and discretionary routes with
   identical width triples: exact fit, finite stretch, zero stretch, shrink,
   overfull minimum width, fill/saturated summaries, and near-limit arithmetic.
4. **Recovery purpose:** a paragraph without an acceptable pass-2 path reaches
   pass 3 and selects loose, non-overfull lines without requiring fallback.
5. **Recovery limits:** an unbreakable overlong word still requires final fallback
   in pass 3. Positive E must not create any fallback in pass 2.
6. **Discovery:** test cached positive and empty results, a new textbox reached
   or queried only in pass 3, explicit markers, the chosen badness trigger, and
   a provider error arising only during pass 3.
7. **Reuse and determinism:** compare cached and uncached boundary scoring with
   E active, including replacement/new-successor invalidation. Repeat runs and
   check identical selected paths and quality.
8. **Ownership and geometry:** assert no artificial stretch is written into knot
   capacities, fragment metrics or seeds. Verify internal pass identity and
   publication of only the winning pass's selections.

### Stage 4: Benchmarks and Documentation (Implemented)

`BenchmarkEmergencyWorkloads` in `emergency_benchmark_test.go` covers 16, 128 and
512 repeated words with four policies: loose recovery without a provider, loose
recovery with a declining provider, productive discretionary recovery, and
unavoidable overfull fallback with a declining provider. Fixture assertions
require both earlier passes to fail and the emergency pass to complete. Recovery
cases select no fallbacks; the productive case selects one shaped alternative
per word. `TestEmergencyBenchmarkWorkloads` runs these checks in the normal suite.

The `public` scope times `BreakParagraph`, including validation, all passes,
lazy shaping and selection publication. The `isolated` scope times pass 3 setup,
scanning and collection, after the real first two passes run outside timing to
establish representative discovery caches. Encoding and owner construction are
outside both timers and fresh per iteration; this is not end-to-end encoding or
freezing latency.

Timed calls leave work instrumentation disabled. A separate untimed diagnostic
records per-pass counters using production pass configurations and scan/collector
functions. Tests check its pass identity, quality and path against the production
coordinator. Every timed iteration checks results and provider/policy call counts
against the diagnostic; isolated calls also check quality and pass identity.
This avoids introducing production measurement hooks or timing counter updates.

Do not assume three passes cost three times one pass. Earlier passes may stop
early, cache discovery for later passes, and maintain different active-state
populations. Benchmarks report selected pass, timed pass count, peak working
states, actual/reused ordinary evaluations, discretionary evaluations, provider
requests, policy calls, allocations and timing. Public counters sum actual work
over all passes, but take the maximum peak rather than summing peaks. Isolated
counters describe only pass 3 and exclude its untimed prelude. Public output also
includes per-pass evaluation totals; the verbose fixture test logs all per-pass
work counters. Counters describe one operation, not sums over benchmark iterations.

Reproduction commands, measured timings, allocation/work counts and interpretation
are in [TESTING.md](TESTING.md#emergency-pass-workloads). The initial measurements
show early exhaustion of passes 1/2, not worst-case three-pass overhead. These
narrow, repeated-word synthetic fixtures do not cover late pass-2 failure, broad
active-state populations, font-backed shaping or high discretionary fan-out.

## Freezing Boundary and Non-Goals

The existing width allocator rejects insufficient real capacity with
`errInsufficientCapacity`. Some emergency-selected lines will therefore remain
unfreezable until underfull allocation/reporting is implemented. Emergency
stretch must not be smuggled into real capacities to make allocation succeed.

The pass and freezing support can be developed independently, but the limitation
must be visible. Calling it universally "no regression" would be too strong:
changed break choices and custom tolerances can affect downstream outcomes.
Neither every emergency result nor every line within one is necessarily underfull.

This change does not implement over-/underfull freezing, exact fill-aware width
summaries, fitness classes, additional hyphen demerits, new provider APIs, mutable
paragraph cache invalidation, or a public diagnostic-result API.
