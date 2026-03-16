# Knuth-Plass / Khipu Inspection Notes

This note summarizes the current state of:

- `linebreak/kp`
- `linebreak/knuthplass`
- the relevant `khipu` code in the repository root

The findings are based on the current worktree as inspected on March 16, 2026.

## Executive summary

`linebreak/kp` is still the only working Knuth-Plass implementation in this tree. It is tied to the old array-of-interfaces `KhipuAOS` model, to `khipu.Mark`, and to the `linebreak.Cursor` abstraction. Its tests currently pass.

`linebreak/knuthplass` is not yet a finished replacement. It has already moved to the new `Khipu`/`KnotCore` representation and removed some of the older object-heavy structures. The single-path `BreakParagraph` API is now wired end-to-end, backward collection is implemented, and the current glue/kern trimming semantics are active in the width calculation. Its package tests now pass, but parity with `linebreak/kp` is still incomplete and several documented K&P features are still absent.

The main architectural pressure point is the split between two khipu models:

- old: `KhipuAOS` + `Cursor` + separate penalty knots
- new: `Khipu` + `KnotCore` + penalties stored on each knot

The rewrite is therefore not just a package rename. It is also a change in the linebreaking input model.

## What exists today

### 1. `linebreak/kp`

This is the old, working implementation.

Observed characteristics:

- directory name is `kp`, but the Go package name is still `knuthplass`
- it operates on `linebreak.Cursor`, which is currently tied to `*khipu.KhipuAOS`
- breakpoints are represented as `khipu.Mark`
- it builds an explicit breakpoint graph (`fbGraph`) with edges labeled by cost and line count
- it maintains a horizon of active feasible breakpoints using `github.com/emirpasic/gods/sets/hashset`
- it keeps per-breakpoint bookkeeping for segment width, discardable material, and total cost
- it can reconstruct linebreak solutions and return the best one

Practical status:

- `env GOCACHE=/Users/npi/prg/go/khipu/.gocache go test ./linebreak/kp` passes

This package is still the behavioral reference.

### 2. `linebreak/knuthplass`

This is the in-progress rewrite.

Observed characteristics:

- it uses the new `*khipu.Khipu` structure directly, not `linebreak.Cursor`
- breakpoints are plain knot indices (`kinx`)
- segment state is stored in `kp.paths map[origin]bookkeeping`
- the graph is simplified to `graph` with `map[kinx]map[origin]edge`
- the horizon is now `map[kinx]struct{}` instead of the old hashset wrapper
- many pieces of the old implementation are still present as commented-out code inside the file

Practical status:

- `BreakParagraph` now builds the graph, identifies the terminal knot by index, and returns the single best breakpoint path as `[]kinx`
- `collectOptimalBreakpoints(end kinx)` exists and backtracks through the pruned graph using line-number labels
- discardable-item bookkeeping is active through `leadingTrim`, `trailingTrim`, `seenContent`, `classifyLineItem(...)`, `appendItem(...)`, and `effectiveWidth(...)`
- the package tests now cover bookkeeping transitions as well as trailing and later-line leading trimming for glue and kern
- `FindBreakpoints` is no longer on the active path and has been removed from the rewrite
- GraphViz output was not ported
- `env GOCACHE=/Users/npi/prg/go/khipu/.gocache go test ./linebreak/knuthplass` passes

This package is not yet functionally equivalent to `linebreak/kp`.

## Relevant khipu code

### Old model: `KhipuAOS`

Relevant files:

- `khipu.go`
- `cursor.go`
- `linebreak/linebreak.go`
- `linebreak/measure.go`
- `khipukamayuq.go`

Properties:

- knots are stored as interface values (`[]Knot`)
- penalties are explicit knots (`PenaltyItem`)
- the linebreaker consumes the paragraph through `linebreak.Cursor`
- `linebreak.Cursor` exposes `Khipu() *khipu.KhipuAOS`, so the abstraction is not really independent of the storage type
- `linebreak.NewFixedWidthCursor` mutates the underlying `KhipuAOS` while iterating in tests

Implication:

The old K&P package is strongly coupled to the old storage and traversal model.

### New model: `Khipu`

Relevant file:

- `khipu_soa.go`

Properties:

- storage is structure-of-arrays: `W`, `MinW`, `MaxW`, `Penalty`, `Pos`, `Len`, `Kind`
- access is by integer index via `KnotByIndex`
- the encoded unit is `KnotCore`
- this is much closer to what a lightweight dynamic-programming implementation wants

Important semantic change:

- the new `Khipu` stores penalty on the knot itself
- the old `KhipuAOS` model represented penalties as separate knots

This changes what a breakpoint "is". In the old code the candidate breakpoint is the penalty knot. In the new code the candidate breakpoint is an index whose knot also carries width, kind, and penalty.

## Key findings

### 1. The rewrite has changed representation, but not yet fully changed algorithm shape

The new package removes some old OOP-style scaffolding, but the core algorithm is still recognizably the old graph search:

- horizon of active breakpoints
- graph of feasible segments
- one best predecessor per line count
- backtracking from the final breakpoint

So far this is a transliteration, not yet a fresh minimal design.

### 2. `linebreak.Cursor` is a major coupling point

The shared `linebreak` package still defines:

- `Cursor.Khipu() *khipu.KhipuAOS`

That hardwires the linebreaking abstraction to the old storage. The rewrite can therefore not reuse the common API cleanly and has duplicated:

- `Parameters`
- `WSS`
- infinity constants
- demerit capping

This is the clearest sign that the current public linebreaking abstraction no longer matches the direction of `Khipu`.

### 3. The new `Khipu` breakpoint semantics are not settled yet

The old code assumes:

- explicit penalty knots
- a final forced break penalty (`-10000`) at paragraph end
- discardable material can be tracked around that breakpoint

The new code currently encodes one `Penalty` per `KnotCore`, and `EncodeParagraph` does not obviously append a paragraph-final forced break node. The tests for the rewrite therefore build synthetic `Khipu` values manually.

Before the rewrite can be finished, it needs an explicit answer to:

- what exact knot index denotes a feasible breakpoint?
- how is paragraph end represented?
- how are trailing spaces/glue around a breakpoint represented?

### 4. Some critical semantics have now been ported, but not all

In `linebreak/knuthplass` the new path bookkeeping now preserves the main glue/kern trimming semantics of the old implementation, but it still does not cover the full long-term notion of “discardable”.

What is now present:

- `leadingTrim` and `trailingTrim` are actively maintained in `bookkeeping`
- `seenContent` tracks whether a segment already contains retained content
- `effectiveWidth(...)` subtracts trimmed material from the active segment width
- terminal `ParFillSkip + InfinityMerits` is treated as retained-neutral, not as ordinary trimmed glue

Remaining gaps:

- paragraph-initial leading whitespace is not yet being treated as a separate design topic
- the implementation still uses explicit glue/kern handling, not a fully general international-script discardability model
- hyphenation and discretionary handling are still future work

This is especially important because the old implementation used discard accounting to avoid charging leading and trailing glue to a line.

### 6. The rewrite still scans more state than it probably needs

`updatePath` and `calcCost` both iterate over all entries in `kp.paths` and then filter by origin breakpoint.

That means the current structure is still map-heavy and scan-heavy:

- for each input knot
- for each active breakpoint
- scan all stored paths

This is simpler than the old code in some ways, but it does not yet look like the final lightweight design.

### 7. Duplicate-breakpoint handling is still rough

The failing rewrite test logs repeated messages such as:

- `Breakpoint at position 1 already known`
- `Breakpoint at position 2 already known`

This comes from `evalNewSegment`/`AddBP` behavior. It is not fatal, but it shows that the current graph API is still transitional and not yet cleanly aligned with the algorithm.

### 8. Penalty scaling differs between the old and new worlds

`linebreak/kp` and the old linebreaking code use TeX-like sentinel values around `±10000`.

The new `Khipu` code in `khipu_soa.go` defines:

- `MaxPenalty = 10000`
- `MaxMerit = -10000`

but `normPenalties(...)` currently compresses ordinary penalties into roughly `[-100, 100]`.

This is not necessarily wrong, but it means the new K&P design should explicitly decide:

- whether penalties are truly TeX-scaled
- or whether K&P only needs a small local penalty range plus separate forced/prohibited sentinels

That decision should not remain accidental.

### 9. The documented demerits formula is only partially migrated

The PDF spends substantial effort on the adjustment ratio, badness, and demerits calculation. The rewrite keeps the broad structure of the computation, but it still uses:

- `p2 := abs(p)`

instead of the classic penalty-square term that the documented derivation expects.

This is not just an implementation detail. It changes the ranking of feasible breakpoints, so even after the rewrite starts returning paths, it may still disagree with the documented algorithm for reasons other than incomplete migration.

### 10. Several parameters exist but are not active in the rewrite

`linebreak/knuthplass` still defines:

- `PreTolerance`
- `DoubleHyphenDemerits`
- `FinalHyphenDemerits`
- `ParFillSkip`

but the rewrite does not currently implement the behavior these parameters imply.

This matters because the PDF narrative assumes:

- the algorithm can compare linebreaks under explicit demerit rules
- last-line and hyphen-related effects are part of the total-cost calculation
- the paragraph is completed by a proper terminal condition

At the moment the parameter surface is ahead of the implementation.

## Comparison With `knuthplass.pdf`

Using `qpdf --qdf --object-streams=disable ... | strings`, the first half of the bundled PDF confirms the following algorithmic expectations.

### 1. Active nodes form a sliding one-line horizon

The PDF describes the algorithm as maintaining a list of active nodes, then advancing a one-line-wide sliding window across the paragraph. Nodes are removed when the next possible breakpoint is either too far away or too close to produce a feasible line.

Status in `linebreak/knuthplass`:

- partially present
- the rewrite still has a `horizon`, and it removes entries when `stillreachable` becomes false
- final-node selection and backward path reconstruction now exist for the single best result
- however, the multi-variant API and some semantic details from the PDF are still missing

### 2. Feasible breakpoints need proper boundary handling for spaces

The PDF repeatedly treats the considered breakpoint as the space after a word and talks about fitting the text from one active node to the next candidate breakpoint. That only works correctly if leading and trailing discardable material are handled carefully.

Status in `linebreak/knuthplass`:

- partially migrated
- leading and trailing trimming are now active in the implementation for explicit glue and kern items
- `bookkeeping` maintains `leadingTrim`, `trailingTrim`, and `seenContent`
- `effectiveWidth(...)` subtracts trimmed material before demerits are computed
- tests now cover bookkeeping transitions, consecutive discardables, trailing trim at a breakpoint, and leading trim on later lines

Remaining limitations:

- paragraph-initial leading whitespace is intentionally deferred
- the current approach is still explicit glue/kern logic, not a general “discardable item” model for international script

### 3. The final result is chosen only after the end of the paragraph is known

The PDF's worked example makes two points very clearly:

- the optimal layout cannot be known until the last feasible breaks are considered
- the number of lines is an output of the optimization, not an input

Status in `linebreak/knuthplass`:

- migrated for the active API
- `constructBreakpointGraph()` now sets `kp.end` to the final knot index
- `collectOptimalBreakpoints(...)` chooses the cheapest final `(end, line)` state and reconstructs one winning path
- `BreakParagraph(...)` returns that path as `[]kinx`
- the earlier multi-variant API is no longer on the active path in this rewrite

This was the biggest functional gap in the rewrite. It is now closed for the current public API of the package.

### 4. The documented algorithm relies on a paragraph-final terminal condition

The PDF's discussion of the penultimate and final lines implies a true paragraph terminator: the algorithm must know where the paragraph ends, and the last line must be evaluated under different constraints from interior lines.

Status in `linebreak/knuthplass`:

- partially modeled
- the current collector assumes the paragraph ends in a real terminal knot and identifies it by final index
- the agreed convention is a single terminal node carrying both `ParFillSkip` dimensions and `InfinityMerits`
- the current tests exercise that convention by constructing synthetic `Khipu` values with a final combined glue-and-penalty node
- the general paragraph encoder still does not appear to guarantee this invariant automatically

Important nuance:

- this gap is not only in the rewrite
- the older `linebreak/kp` package also carries `ParFillSkip` as configuration without really integrating it into the algorithm; tests append a final `-10000` penalty manually

So paragraph-final handling is a migration gap, but also a deeper design debt in the existing implementation.

### 5. The PDF assumes best-predecessor selection and backward traversal

The PDF describes how multiple active candidates may reach a later breakpoint, but only the best path to a node should survive for a given optimization state. It then traces the optimal path back once the paragraph end has been determined.

Status in `linebreak/knuthplass`:

- now present for the single best path

What is present:

- `evalNewSegment(...)` keeps one best path per `(to, line)` key
- `graph.predecessorForLine(to, line)` recovers the surviving predecessor for backward traversal
- `collectOptimalBreakpoints(...)` walks the best path backward from `kp.end`

What is missing:

- multi-variant result enumeration is no longer present in the rewrite

### 6. The PDF treats the algorithm as dynamic programming over optimal subpaths

The "optimality horizon" section in the PDF explicitly states the DP principle: an optimal paragraph consists of optimal sub-paragraphs. That is what justifies pruning suboptimal predecessors.

Status in `linebreak/knuthplass`:

- conceptually present
- operationally incomplete

The rewrite has the right idea with `paths map[origin]bookkeeping`, but it still lacks the full end-to-end machinery that makes the DP invariant observable and testable.

### 7. The rewrite still carries legacy and dead migration paths

The PDF comparison also makes it clear which parts of the rewrite are no longer on the active path:

- the old `feasibleBreakpoint` and `newFeasibleLine(...)` logic is still present as commented code
- `penaltyAt(...)` still exists, but the rewrite no longer drives the algorithm through `linebreak.Cursor`
- GraphViz support is commented out in the new package

This is not a correctness bug by itself, but it makes it harder to see which algorithm is actually implemented today.

## Backward collection with the new graph

The old implementation of `collectFeasibleBreakpoints(...)` in `linebreak/kp` collects all valid final variants, sorts them by total cost, and returns every breakpoint sequence.

For the new version, that turned out to be unnecessary. The current rewrite only needs the single best result, so the collector now:

1. find the cheapest final state at `kp.end`
2. walk backward through the unique predecessor chain for that state
3. reverse the collected indices
4. return `[]kinx`

### What changes in the new structure

In `linebreak/knuthplass`, the natural sources are:

- `kp.end`: final breakpoint index
- `kp.paths[origin{end, line}]`: total cost of reaching `end` on `line`
- `kp.graph.edgesTo[end][origin{pred, line}]`: predecessor edge for that `(end, line)` state

The implemented collector therefore no longer inspects `last.books`. It:

- scan `kp.paths` for entries whose key is `origin{kp.end, line}`
- choose the one with minimum `totalcost`
- walk predecessor edges backward by matching the line number in `edgesTo`

### Placement

The new collector now lives immediately after `constructBreakpointGraph()` in `linebreak/knuthplass/knuthplass.go`. Earlier legacy collector code has been removed from the active implementation path.

### Graph helper

The new `graph` type now includes:

```go
func (g *graph) predecessorForLine(to kinx, line lineNo) (kinx, edge, bool)
```

Its job is:

- look at `g.edgesTo[to]`
- find the unique edge whose key has `origin.line == line`
- return that predecessor

Because pruning is already supposed to keep only one predecessor per `(to, line)`, this lookup should be unique in normal operation.

### Implemented collector shape

The implemented logic is structurally the same as the earlier sketch:

```go
func (kp *linebreaker) collectOptimalBreakpoints(end kinx) ([]kinx, merits, bool) {
    bestLine := lineNo(0)
    bestCost := merits(0)
    found := false

    for st, path := range kp.paths {
        if st.from != end {
            continue
        }
        if !found || path.totalcost < bestCost {
            bestLine = st.line
            bestCost = path.totalcost
            found = true
        }
    }
    if !found {
        return nil, 0, false
    }

    breaks := make([]kinx, 0, bestLine)
    cur := end
    line := bestLine

    for cur != kp.root {
        breaks = append(breaks, cur)
        pred, _, ok := kp.graph.predecessorForLine(cur, line)
        if !ok {
            return nil, 0, false
        }
        cur = pred
        line--
    }

    slices.Reverse(breaks)
    return breaks, bestCost, true
}
```

### Why this is the right translation

This preserves the original algorithmic intent while dropping the unneeded variant enumeration:

- the final candidate is still chosen from all end states
- predecessor recovery still depends on the pruned one-predecessor-per-line invariant
- backward traversal still walks from paragraph end to root
- the result type now matches the new implementation model: `[]kinx`

## Test observations

With a workspace-local build cache:

```sh
env GOCACHE=/Users/npi/prg/go/khipu/.gocache go test ./linebreak/kp ./linebreak/knuthplass
```

Observed result:

- `./linebreak/kp`: passes
- `./linebreak/knuthplass`: passes
- `.`: fails in the root `khipu` package
- `./...`: still fails because the root `khipu` package fails; `linebreak/firstfit`, `linebreak/kp`, and `linebreak/knuthplass` pass

`linebreak/knuthplass` now does reconstruct and return the optimal breakpoint path for `BreakParagraph(...)`. The remaining repository-wide failures are upstream of this package.

In addition, the package-local tests now explicitly cover the current discardable-item semantics:

- bookkeeping transitions for leading trim, trailing trim, and retained-neutral terminal nodes
- effective width after trimming
- retention of internal discardable material
- accumulation of consecutive discardables
- trailing trim at a chosen breakpoint
- leading trim on a later line

The root-package `khipu` failure is also relevant context for the rewrite. It shows that the new paragraph encoding path is itself still in flux, so `linebreak/knuthplass` does not yet sit on top of a fully stable upstream representation.

Therefore the full repository test suite is not currently a clean migration oracle for K&P work.

## Design implications

The current inspection suggests that the safest route is:

1. Keep `linebreak/kp` as the reference implementation until parity is proven.
2. Treat `linebreak/knuthplass` as an experimental branch of the algorithm, not as a drop-in replacement yet.
3. Define a small, stable input abstraction for linebreaking before finishing the rewrite.

That abstraction should probably expose only what K&P needs:

- knot count
- indexed access to `W`, `MinW`, `MaxW`, `Penalty`, `Kind`
- paragraph-end sentinel behavior
- a clear rule for what counts as trimmed boundary material today and what should become future international-script discardability later

This can be an interface or a small generic accessor layer. The important part is that it should not depend on `KhipuAOS`, `khipu.Mark`, or the current `linebreak.Cursor` API.

## Recommended next steps

### Short term

- stop extending commented-out old code in `linebreak/knuthplass`: **DONE**
- remove the old multi-variant API from the active path: **DONE**
- port current glue/kern discardable handling before adding parity tests: **DONE**
- fix line-number accounting and verify it against non-rectangular paragraph shapes
- decide how final forced paragraph break is represented in the new `Khipu`: **DONE**
- decide how paragraph-initial whitespace should be modeled separately from ordinary discardable glue

### Medium term

- introduce a linebreaking input abstraction that is independent of `KhipuAOS`
- make the old and new implementations run against comparable fixtures
- add parity tests that compare:
  - number of lines
  - breakpoint positions
  - total demerits

### Longer term

- once the new implementation is feature-complete and tested, delete:
  - package `linebreak/kp`
  - the commented-out legacy blocks in `linebreak/knuthplass`

## Bottom line

The rewrite is moving in the right direction structurally: integer indices, `KnotCore`, and SOA storage are better foundations for a lightweight implementation than the current `KhipuAOS` + `Cursor` + `Mark` design.

However, the new package is still a transitional port. It has not yet reached semantic or API completeness, and the real blocker is not just missing code. The blocker is that the repository still contains two different models of what a breakable paragraph is. The next design step should therefore be to settle the linebreaker input model first, then finish the algorithm on top of that model.

## Treating of “discardable” Items

TeX introduces the term “discardable” for items in the hlist which should be disregarded when breaking lines (and building hboxes). For TeX, this is mainly about whitespace. However, for truly international script there may be much more complicated scenarios. 

The current rewrite now implements a deliberately narrow version of this idea:

- explicit `KTGlue` and `KTKern` handling
- separate tracking of `leadingTrim` and `trailingTrim`
- `seenContent` to distinguish empty-from-contentful segments
- special handling for the terminal `ParFillSkip + InfinityMerits` node as retained-neutral material

This is enough for the current migration target and is covered by package tests. It should not yet be mistaken for the final general model of discardability.

The final implementation will probably need to attach a `isDiscardable` bit with every khipu node (as in TeX). One example: `\parindent` whitespace at the beginning of a paragraph is not discardable, but rather flags the typographic custom to indent the first line of a paragraph for readability.   

Another level of complexity will stem from hyphenation. Hyphenation will insert discretionary items in the khipu, which complicates the algorithm for linebreaking. However, we will tackle this problem after normal linebreaking is fully in place.

Implementation note:

- the old list-based `KhipuAOS` implementation already has `HyphenateTextBoxes()`, which performs eager hyphenation and therefore imports the hyphenation package directly
- the new solution should avoid this coupling
- instead, the rewrite should introduce a separate `Hyphenator` interface
- that interface can be kept small at first and developed incrementally as the actual API surface becomes clearer during implementation

## Khipu SOA data-structure, Hyphenation

The SOA-style data-structure for type `Khipu` was an experiment of mine to perhaps profit during line-breaking, where width-information is the most important property of a khipu knot. However, this advantage did not materialize. The fields of a khipu knot relevant for line-breaking are:  
- W
- MaxW
- MinW
- Penalty

Field `KnotType` is currently relevant for deciding about discarding/trimming. However, in the long run the only correct handling of this is with a `isDiscardable` flag, to be set by the khipukamayuq. The line-breaker should only be a consumer of this information, and the information will – at least for international script – not be provided by out-of-band information (i.e., not be provided by calling some general predicate function).   
  
We should think about a possible improvement with the memory layout of khipu properties. We could define a `{ W, MaxW, MinW, Penalty, isDiscardable }` knot core, which is more SOA designed and would make it possible for the linebreaker to load a sequence into memory and sequentially operate on them.   
  
The `isDiscardable` flag needs just one bit. It seems overkill to use a bool field, but bit-packing seems less expressive. The only field in the core knot available for bit-packing would be `Penalty`, which will never exceed 10000, thus an int16 would leave us with room for an additional bit for `isDiscardable` . It would complicate the usage of knot core in the line-breaker, so I am unsure if it's worth the effort.   
  
If the first line-breaking path does not result in a overall (de-)merit below a given (parameter-)threshold, the line-breaker should do a second run with hyphenation enabled. To support this, the khipu needs to support storing hyphenation opportunities. TeX calls these opportunities *discretionary*. Such a discretionary consists of 3 parts:  
  
- unhyphenated fragment (usually a word, e.g. “breakpoint”)
- pre-break fragment (e.g., “break-”)
- post-break fragment (e.g, “point”)

An external hyphenator will be able to be queried for break opportunities, i.e. : “how can 'breakpoint' be broken up?”. The hyphenator gives a discretionary as a result. Therefore we will have to be able to store discretionaries in the khipu and later access them from the line-breaker. A discretionary will be related to a knot of type `TextBox`. I propose to store discretionaries in a separate AOS within the khipu. Khipu knots should then somehow point/be associated with discretionary items. More than one discretionary item may be associated with a textbox. (e.g.,  “fragmented” may result in “{fragmented}{frag-}{mented}” or “{fragmented}{fragment-}{ed}”). 

## Evolution Options For `Khipu`

The clarification is important: hyphenation opportunities are not to be materialized eagerly. The line-breaker will only consult a hyphenator if an initial pass produces line costs above a configurable threshold or fails to find acceptable breakpoints. Therefore the design target is not “store all possible discretionaries in every khipu”, but “support lazy attachment or lookup of discretionaries when the second pass asks for them”.

Below are three plausible evolution options.

### Option 1: Keep SOA, add a flag plane, keep discretionaries external/lazy

This is the smallest evolution from the current `Khipu`.

Idea:

- keep the current parallel slices for `W`, `MinW`, `MaxW`, `Penalty`, `Pos`, `Len`
- add a new slice or bitset for flags
- one of those flags would be `isDiscardable`
- a second-pass line-breaker can query a hyphenation service when it sees a high-cost segment and receive discretionary candidates on demand
- those discretionary candidates stay outside the base `Khipu`, e.g. in a side cache keyed by knot index

Advantages:

- minimal disruption to the current `Khipu`
- preserves the current SOA layout
- discardability becomes explicit and prepared by `Khipukamayuq`
- no eager hyphenation cost

Disadvantages:

- line-breaker still has to combine several parallel slices mentally
- discretionary lookup becomes a second data source beside the base `Khipu`
- debugging may be more cumbersome because some paragraph state lives outside the khipu itself

Assessment:

- safest near-term option
- probably the best choice if the main goal were only to stabilize linebreaking semantics first
- however, it is weaker once rendering is taken into account, because finally selected discretionary decisions need to remain attached to the paragraph object

### Option 2: Keep SOA, but add a dedicated linebreaking view over `Khipu`

This option keeps `Khipu` as storage, but introduces a consumer-oriented layer for the line-breaker.

Idea:

- `Khipu` remains the mutable paragraph store
- `Khipukamayuq` prepares a lightweight linebreaking view exposing exactly the fields K&P needs
- that view can provide:
  - `W`, `MinW`, `MaxW`, `Penalty`
  - flags such as `isDiscardable`
  - terminal-paragraph semantics
  - optional accessors for lazy discretionary lookup

Advantages:

- separates paragraph storage concerns from linebreaking concerns
- avoids overloading `Khipu` with every future K&P-specific requirement
- gives room for a second-pass hyphenation API without mutating the core representation too early
- likely the cleanest long-term architecture

Disadvantages:

- introduces another abstraction layer
- more design work up front
- requires discipline to keep the view small and stable

Assessment:

- architecturally the cleanest option
- likely best if `Khipu` is expected to serve more consumers than linebreaking

### Option 3: Keep SOA for base knots, add a sparse discretionary side-table inside `Khipu`

This option accepts that discretionary data belongs logically to the paragraph object, but keeps it sparse.

Idea:

- base knot properties stay in SOA slices
- add a side-table owned by `Khipu` for discretionary candidates
- entries are keyed by textbox knot index
- each entry holds zero or more discretionary alternatives
- the table is filled lazily when the line-breaker reaches for hyphenation in the second pass
- the finally selected discretionary decisions are also retained in `Khipu` for later rendering

Advantages:

- discretionary data stays attached to the paragraph object
- avoids bloating every knot with hyphenation fields
- fits the fact that only a minority of knots will ever need discretionary alternatives
- makes repeated second-pass attempts cheaper because discovered alternatives can be cached
- matches the real pipeline more closely: opportunities are discovered lazily, but decisions survive into rendering

Disadvantages:

- `Khipu` becomes more specialized toward linebreaking
- lifecycle and invalidation rules for cached discretionaries must be designed
- still needs an explicit flag plane or equivalent for discardability

Assessment:

- strongest option if hyphenation is expected to be queried repeatedly on the same paragraph
- more invasive than Option 1, but still much better than eager materialization
- now looks like the strongest semantic fit, because the renderer must consume the finally chosen discretionary outcomes

## Provisional Evaluation

Given the current status of the rewrite, I would now rank them like this:

1. Option 3 as the most faithful model of the full pipeline
2. Option 2 for architectural cleanliness
3. Option 1 only as the smallest short-term simplification

Reasoning:

- the current blocker is not raw performance, but semantic stabilization across both linebreaking and rendering
- explicit discardability is needed sooner than full hyphenation support
- lazy hyphenation still suggests a sparse side structure, not eager expansion of the base khipu
- selected discretionary decisions must survive into rendering, so they cannot remain purely external

## Design Constraints That Seem Clear Already

Regardless of the option chosen, these points look stable:

- `isDiscardable` should be prepared upstream by `Khipukamayuq`
- the line-breaker should consume discardability, not infer it from `KnotType`
- eager materialization of all hyphenation opportunities is not acceptable
- discretionary data should be sparse and tied to textbox positions
- finally selected discretionary decisions must remain attached to `Khipu` for the renderer
- linebreaking parameters for hyphenation must stay outside the khipu storage itself

The most likely next design question is therefore not “should khipu store all discretionaries?”, but:

- how should `Khipu` distinguish between lazily discovered discretionary candidates and the finally selected discretionary decisions?

## Refined Direction

The current discussion suggests a refinement of Option 3.

The important distinction is:

- the line-breaker consumes break opportunities
- the renderer consumes break decisions

That means:

- discretionary candidates may still be discovered lazily
- they may still be cached sparsely, only where the second pass asks for them
- but once a discretionary has been chosen for an actual breakpoint, that choice must be retained in `Khipu`

So the most plausible next-step model is:

- base knot data remains SOA
- `Khipu` owns a sparse discretionary side-table keyed by textbox index
- the side-table can hold zero or more lazily discovered discretionary candidates
- linebreaking records which discretionary candidate, if any, was chosen
- rendering consumes only the chosen decisions, not the entire search space

This keeps eager materialization off the table while still respecting the fact that the renderer works on shaped/measured paragraph state rather than on the original rune stream.
