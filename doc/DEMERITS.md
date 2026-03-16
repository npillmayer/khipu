# Demerits Calculation: TeX vs Current Implementation

This note compares the demerits and badness calculation described in [linebreak/kp/knuthplass.pdf](/Users/npi/prg/go/khipu/linebreak/kp/knuthplass.pdf) with the current code in:

- [linebreak/knuthplass/knuthplass.go](/Users/npi/prg/go/khipu/linebreak/knuthplass/knuthplass.go)
- [linebreak/kp/knuthplass.go](/Users/npi/prg/go/khipu/linebreak/kp/knuthplass.go)

The older `kp` package still carries the inherited approximate formula. The rewrite in `linebreak/knuthplass` has now gone through the first cleanup stages and is closer to the original TeX model than the legacy code.

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

## What The Current Rewrite Now Preserves

The current rewrite keeps the broad structure of the original algorithm and now also matches TeX in several places where the older implementation differed:

- it computes an explicit TeX-style adjustment ratio
- it distinguishes stretch from shrink
- it computes cubic badness from `|r|^3`
- it uses `PreTolerance` for pass 1 and `Tolerance` for pass 2
- a line at exactly the active threshold is now admissible
- it computes demerits with a squared penalty term `p^2`
- it no longer caps ordinary demerits into the tiny `±10000` penalty range
- it now separates:
  - geometric feasibility
  - pass screening by badness
  - ranking by demerits

So the rewrite is no longer merely "roughly Knuth-Plass"; the numeric core is now much closer to the original model.

## What Is Still Different

### 1. Forced breaks are handled in control flow, not inside `calcDemerits(...)`

TeX treats `p <= -10000` inside the demerits formula as a special case:

```text
d = q^2
```

The current rewrite handles forced breaks earlier, in [constructBreakpointGraph()](/Users/npi/prg/go/khipu/linebreak/knuthplass/knuthplass.go). Before the cost function is called, forced-break penalties are neutralized to `0`, and the forced-break branch is taken in the graph-construction loop.

This still deviates structurally from TeX, but it is now an intentional design choice rather than a numerical bug. It keeps `calcDemerits(...)` simpler while preserving the intended effect.

### 2. Fitness classes are still missing

The original Knuth-Plass algorithm distinguishes fitness classes such as:

- tight
- decent
- loose
- very loose

and adds demerits when adjacent lines jump too far between classes.

The current implementation does not yet model fitness classes.

### 3. Hyphen-related extra demerits are not active yet

The parameter set already includes:

- `DoubleHyphenDemerits`
- `FinalHyphenDemerits`
- `HyphenPenalty`
- `ExHyphenPenalty`

However, the rewrite does not yet apply them in the original TeX sense, because discretionary / hyphenation support is not integrated yet.

### 4. Overfull fallback is still a local policy, not a TeX-faithful detail

When the horizon is about to drain, the rewrite inserts an explicit fallback path with [`AwfulDemerits`](/Users/npi/prg/go/khipu/linebreak/knuthplass/wss.go). That is a pragmatic policy to keep the paragraph breakable, and it interacts with the pass-2 trigger.

This is useful for robustness, but it is not a direct transcription of TeX's paragraph-breaking internals.

## Stage Status

### Implemented

The following cleanup stages are now in place in the rewrite:

- Stage 1:
  - stretch/shrink denominator fixed
  - threshold comparison changed from `<` to `<=`
  - ordinary demerits no longer collapsed into the penalty sentinel range
- Stage 2:
  - `abs(p)` replaced with `p * p`
  - forced-break handling moved to explicit control flow
- Stage 3:
  - badness split into:
    - adjustment ratio
    - badness
    - demerits
  - the math has focused table-driven tests
- Stage 4:
  - candidate evaluation now distinguishes:
    - infeasible
    - screened out
    - accepted
  - screening and ranking are explicit separate steps

### Still Missing

The main remaining TeX-related gaps are:

- fitness classes
- double-hyphen and final-hyphen extra demerits
- integration of discretionary hyphenation into the screening path

## Comparison With The Legacy `kp` Package

The old package in [linebreak/kp/knuthplass.go](/Users/npi/prg/go/khipu/linebreak/kp/knuthplass.go) still uses the inherited approximate formula:

- penalty handled as `abs(p)` rather than `p^2`
- no explicit stretch/shrink distinction in the badness denominator
- aggressive demerit capping into the penalty sentinel range
- no explicit separation between screening and ranking

So at this point the rewrite is meaningfully closer to the PDF's TeX model than the legacy implementation.

## Bottom Line

The current rewrite should now be described as:

- structurally Knuth-Plass
- numerically much closer to TeX than before
- still incomplete in the areas of:
  - fitness classes
  - hyphen-related extra demerits
  - full discretionary integration

The most important remaining differences are no longer in the basic badness and demerits arithmetic. They are now in the secondary heuristics and the hyphenation-specific parts of the original algorithm.

## Future Work: Fitness Classes

One important part of the original Knuth-Plass algorithm is still missing entirely: fitness classes.

This section records the concept and its implementation consequences in enough detail to resume the work later.

### What Fitness Classes Are

TeX does not only look at the demerits of one line in isolation. It also tries to avoid visibly uneven spacing from one line to the next.

The mechanism for this is the notion of a line's *fitness class*. A fitness class is derived from the line's adjustment ratio:

- a line that is stretched a lot belongs to a loose class
- a line that is shrunk a lot belongs to a tight class
- a line near its natural width belongs to a more neutral class

Knuth-Plass then adds extra demerits when two adjacent lines differ too much in fitness class. This discourages sequences like:

- one very tight line followed by one very loose line
- one heavily stretched line followed by one heavily shrunk line

The goal is not to forbid such transitions absolutely, but to make them less desirable than smoother spacing from line to line.

### Typical Fitness Classes

The original algorithm uses a small number of classes. Conceptually they are:

- very tight
- tight
- loose
- very loose

The exact thresholds come from the adjustment ratio `r`, not from the total demerits.

So fitness is a classification of spacing behavior, not of absolute paragraph quality.

### Where Fitness Comes From In The Current Rewrite

The rewrite already computes:

- an adjustment ratio via [`calcAdjustmentRatio(...)`](/Users/npi/prg/go/khipu/linebreak/knuthplass/knuthplass.go)
- a badness value via [`calcBadness(...)`](/Users/npi/prg/go/khipu/linebreak/knuthplass/knuthplass.go)

Therefore the natural place to classify fitness would be immediately after the adjustment ratio is known.

The likely implementation shape would be:

- introduce `type fitnessClass uint8`
- add a helper:
  - `classifyFitness(ratio float64) fitnessClass`
- store the resulting class on candidate line evaluations

This means fitness belongs logically next to:

- adjustment ratio
- badness
- demerits

in the candidate-evaluation pipeline.

### What Would Need To Be Added

At the data-model level, a future implementation would likely need:

1. A fitness-class type.
   Example conceptual values:
   - `veryTight`
   - `tight`
   - `loose`
   - `veryLoose`

2. A classifier from adjustment ratio to class.
   This should operate on the output of `calcAdjustmentRatio(...)`.

3. A new linebreaking parameter for adjacency penalties.
   Something like:
   - `AdjFitnessDemerits merits`

4. Storage of the current line's fitness on candidate evaluations.
   The current [`lineEvaluation`](/Users/npi/prg/go/khipu/linebreak/knuthplass/knuthplass.go) would need an additional field:
   - `fitness fitnessClass`

5. Storage of the previous line's fitness on active path states.
   The current [`bookkeeping`](/Users/npi/prg/go/khipu/linebreak/knuthplass/knuthplass.go) would need a field such as:
   - `fitness fitnessClass`

### Where The Extra Demerits Would Be Applied

The extra demerits from fitness incompatibility belong at the point where a new candidate line is appended to an existing path.

In the current rewrite, that point is [`evalNewSegment(...)`](/Users/npi/prg/go/khipu/linebreak/knuthplass/knuthplass.go).

The logic would be conceptually:

1. Look at the predecessor path's fitness class.
2. Look at the current candidate line's fitness class.
3. Compute the distance between the two classes.
4. If the distance is greater than one class step, add `AdjFitnessDemerits`.
5. Add that penalty to the candidate's demerits before comparing path totals.

In rough form:

```text
evalCost = predecessor.totalcost + line.demerits + adjacencyPenalty
```

where:

- `adjacencyPenalty = 0` if the classes are compatible
- `adjacencyPenalty = AdjFitnessDemerits` if they are too far apart

### Why Fitness Is Not "Just Another Demerit Term"

This is the crucial design point.

Most line cost terms depend only on the current line:

- its badness
- its breakpoint penalty
- eventually whether it hyphenates

Fitness-class adjacency demerits are different. They depend on the *previous* line as well as the current one.

That means fitness introduces **path dependence**.

Two paths that arrive at the same breakpoint and line number may no longer be equivalent if their previous line fitness differs, because:

- one path may combine well with the next line
- the other may incur extra adjacency demerits on the next step

This is the reason fitness affects state identity, not just cost arithmetic.

### Consequences For The Predecessor Table

The current predecessor table in the rewrite is keyed effectively by:

- `(to, line)`

This works as long as total cost is the only state that matters for the future.

With fitness classes, a more faithful state key becomes:

- `(to, line, fitness)`

because the future cost of extending a path depends on the previous line's fitness class.

This does **not** mean the predecessor-table abstraction has to be abandoned.

The operations stay the same:

- set/update best predecessor for a state
- look up predecessor for a state
- choose the cheapest final state
- walk backward from that final state

So:

- the predecessor-table design still fits
- access patterns do not fundamentally change
- asymptotic lookup complexity with hash maps stays essentially the same

What changes is:

- the identity of a state
- the number of states that may coexist
- the pruning rule

### The Real Change: Pruning Semantics

Without fitness classes, it is usually safe to keep only one best predecessor per:

- `(to, line)`

With fitness classes, that is no longer always safe.

Why?

Because a path that is slightly worse *now* may be better *later* if its fitness class is more compatible with the next line.

So if the implementation prunes too aggressively and keeps only the cheapest path for `(to, line)`, it may incorrectly discard a path that would lead to the best global solution once adjacency demerits are taken into account.

Therefore a more faithful implementation should prune per:

- `(to, line, fitness)`

not just per:

- `(to, line)`

This is the main architectural consequence of fitness classes.

### Minimal Approximation vs Faithful Implementation

There are two plausible implementation strategies.

#### Minimal approximation

Keep the current state key:

- `(to, line)`

Store only the currently cheapest path, including its fitness.

Pros:

- minimal code churn
- same predecessor-table shape
- easy to integrate

Cons:

- can prune away a path that is worse locally but better globally because of fitness compatibility
- therefore not fully faithful to Knuth-Plass

#### More faithful implementation

Refine the state key to:

- `(to, line, fitness)`

Pros:

- consistent with the path-dependent nature of fitness
- much closer to TeX's intended behavior

Cons:

- more states in the predecessor table
- more path variants survive at once
- backward collection must start from the cheapest final fitness-state, not just the cheapest final `(end, line)` state

### Expected Implementation Steps

If fitness classes are added later, a sensible order would be:

1. Introduce `fitnessClass`.
2. Implement `classifyFitness(ratio float64)`.
3. Store fitness on `lineEvaluation`.
4. Add `AdjFitnessDemerits` to `Parameters`.
5. Add predecessor-path fitness to `bookkeeping`.
6. Decide whether the state key will remain:
   - `(to, line)` as an approximation
   or become:
   - `(to, line, fitness)` for correctness
7. Add adjacency demerits in `evalNewSegment(...)`.
8. Update final-state selection and backward collection if the richer state key is chosen.
9. Add focused tests.

### Tests That Would Be Needed

At minimum, future tests should cover:

- fitness classification from representative adjustment ratios
- no extra penalty when adjacent classes are equal or neighboring
- extra penalty when adjacent classes differ by more than one step
- a paragraph where fitness adjacency changes the winning path
- if the richer state key is adopted:
  - coexistence of multiple `(to, line)` states with different fitness classes

### Bottom Line On Fitness

The most important reminder is this:

- fitness classes do **not** invalidate the predecessor-table design
- they **do** refine what a "state" means

So the operations remain the same, but the state key may need to become:

- `(to, line, fitness)`

That is the central reason fitness classes are more than "just another demerit term".
