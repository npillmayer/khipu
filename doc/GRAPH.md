# Graph Inventory For `linebreak/knuthplass`

This note records how the current graph data structure in `linebreak/knuthplass` is actually used by the rewrite of the Knuth-Plass linebreaking algorithm.

The purpose is not yet to redesign the graph, but to establish:

- which operations are genuinely needed by the algorithm
- which operations exist only because the graph was ported from a more general graph model
- which parts of the representation look like candidates for simplification

The findings are based on the current worktree as inspected on March 16, 2026.

## Executive Summary

The current graph is not used as a general-purpose graph.

At runtime, `linebreak/knuthplass` uses it only as:

- a set of feasible breakpoints
- a store of inbound edges keyed by destination breakpoint and line number
- a predecessor lookup table for backward path reconstruction

The paragraph-breaking algorithm does not currently:

- traverse outgoing edges
- remove edges
- run a general graph search
- inspect arbitrary neighbors
- need a fully general node/edge abstraction

This strongly suggests that the real required abstraction is closer to a predecessor table for `(to, line)` states than to a reusable graph library.

## Current Representation

The graph is defined in `linebreak/knuthplass/graph.go` as:

- `feasBP map[kinx]struct{}`
- `edgesTo map[kinx]map[origin]edge`
- `prunedEdges map[kinx]map[origin]edge`

with:

- `kinx` = khipu knot index
- `origin` = `{from kinx, line lineNo}`
- `edge` = `{from, to kinx; cost, total merits; lineno lineNo}`

Operationally, this means:

- nodes are only breakpoint indices
- all adjacency is stored inbound, by destination `to`
- line number is part of the edge key

That last point is important: the graph is not really representing plain connectivity. It is representing dynamic-programming states of the form:

- predecessor `from`
- successor `to`
- line number `line`
- edge cost `cost`
- accumulated total cost `total`

## Inventory Of Graph Operations

### 1. Construction

Used by runtime:

- `newGraph()`

Current use:

- called once from `newLinebreaker()`
- one graph per paragraph
- not shared across paragraphs
- not long-lived outside one linebreaking run

Implication:

- no need for a persistent or reusable graph container

### 2. Registering Breakpoints

Used by runtime:

- `Breakpoint(bp kinx) kinx`
- `AddBP(bp kinx) error`

Current use:

- `evalNewSegment(...)` checks whether `to` is already a breakpoint
- if not, `AddBP(to)` registers it

Access pattern:

- membership test in a set
- insert into a set

What is not used:

- no node payload
- no node metadata other than existence

Implication:

- this is just a breakpoint set, not a node store

### 3. Checking Whether An Exact Edge Exists

Used by runtime:

- `Edge(from, to, line) edge`

Current use:

- `evalNewSegment(...)` probes whether an exact edge already exists for `(from, to, line)`
- used before deciding whether the new path replaces an existing one

Access pattern:

- exact lookup by destination and labeled origin

What is not used:

- no scanning over all edges from `from`
- no scanning over all line variants between two nodes except via exact lookup

Implication:

- current code uses the graph as a keyed store, not as adjacency to traverse

### 4. Inserting Edges

Used by runtime:

- `AddEdge(from, to, cost, total, line)`

Current use:

- `evalNewSegment(...)` inserts the chosen edge for a feasible segment

Access pattern:

- append-mostly insertion during graph construction
- edges are keyed by `to`, then by `(from, line)`

Important detail:

- line number is part of the identity of an edge
- the graph therefore stores DP-labeled transitions, not plain node-to-node edges

### 4a. Replacing A Cheaper Transition Implies Deletion

This point is easy to miss, because the current rewrite does not fully enforce it in the graph.

In the old implementation in `linebreak/kp`, pruning worked like this:

- if a new segment to `(to, line)` was cheaper than the existing one
- the old predecessor edge was explicitly removed with `RemoveEdge(...)`
- then the new edge became the unique surviving predecessor for that state

In the current rewrite, `evalNewSegment(...)` does this only partially:

- it deletes the old path state in `kp.paths`
- it inserts the new cheaper path state
- it adds the new edge
- but it does not remove the old edge from `kp.graph`

Why current code still works:

- `collectOptimalBreakpoints(...)` uses `predecessorForLine(...)`
- `predecessorForLine(...)` scans all predecessors for a given `(to, line)`
- it chooses the one with minimal `total`

So stale edges do not currently break reconstruction, because the lookup re-minimizes over them.

However, the old pruning invariant is no longer faithfully represented in the graph:

- `kp.paths` has one surviving state for `(to, line)`
- `kp.graph` may still contain older, dead predecessors for `(to, line)`

Implication for a future replacement data structure:

- we do need some form of delete-or-overwrite operation for a `(to, line)` predecessor state
- it does not have to be called `RemoveEdge(...)`
- but the data structure must support replacing the current predecessor of `(to, line)` with a cheaper one

This is important enough to count as a required operation.

### 5. Backward Predecessor Lookup

Used by runtime:

- `predecessorForLine(to, line) (kinx, edge, bool)`

Current use:

- `collectOptimalBreakpoints(...)` reconstructs the winning path by repeatedly asking:
  - “for this `to` and this `line`, what predecessor survived pruning?”

Access pattern:

- query by `(to, line)`
- expect at most one valid predecessor in normal operation

This is the most specialized and most important graph read in the current algorithm.

### 6. Debug Printing

Used by runtime:

- `String()`

Current use:

- tracing/debug output from `constructBreakpointGraph()`

Implication:

- not algorithmically essential

### 7. Direct Field Inspection

Used by runtime:

- direct reads of `kp.feasBP`

Current use:

- logging number and contents of feasible breakpoints

Implication:

- this is not a real graph operation, just debug access to internal storage

## Operations Not Used By The Runtime Algorithm

These functions exist in `graph.go`, but the current `knuthplass` algorithm does not call them.

### `To(bp kinx) []kinx`

Purpose:

- returns all predecessors of a destination breakpoint as sorted indices

Actual status:

- not used by `knuthplass.go`
- only used by tests

Observation:

- the runtime algorithm never needs “all predecessors” as a slice
- it only needs “the predecessor for this line number”

### `Cost(from, to, line) (merits, bool)`

Purpose:

- fetches the edge cost for an exact labeled edge

Actual status:

- not used by `knuthplass.go`
- only used by tests

Observation:

- runtime already gets the full edge from `Edge(...)`
- no separate cost accessor seems necessary for the linebreaker itself

### `RemoveEdge(from, to, line)`

Purpose:

- removes an edge and stores it in `prunedEdges`

Actual status:

- not used by `knuthplass.go`
- only used by tests

Observation:

- the current runtime algorithm does not prune by deleting graph edges
- it prunes effectively by keeping only the best state in `kp.paths`
- therefore edge deletion is not part of the current algorithmic path

### `StartOfEdge(edge) kinx`

Purpose:

- returns the edge's origin if it is a registered breakpoint

Actual status:

- not used by `knuthplass.go`
- not used by current tests either

Observation:

- leftover from a more general graph abstraction

## Data Structures That Are Present But Currently Dead

### `prunedEdges`

Current status:

- maintained only by `RemoveEdge(...)`
- never read by the runtime algorithm
- never used in path reconstruction

Implication:

- dead for current K&P execution

### General Inbound Neighbor Enumeration

The structure `edgesTo[to]` supports enumeration of all predecessors.

Current runtime need:

- only predecessor lookup for a specific `line`

Implication:

- the current structure stores more than the algorithm currently consumes

## Test-Only Graph API Surface

The tests in `linebreak/knuthplass/graph_test.go` currently exercise:

- `newGraph()`
- `AddBP(...)`
- `AddEdge(...)`
- `To(...)`
- `Cost(...)`
- `Edge(...)`
- `RemoveEdge(...)`
- `predecessorForLine(...)`

This is broader than the runtime use.

Therefore, some of the existing graph API is being preserved mainly because the ported graph tests still assert it, not because the K&P algorithm actually needs it.

## Minimal Required Operations For The Current Algorithm

If we ignore legacy shape and focus only on what `linebreak/knuthplass` currently needs at runtime, the indispensable operations appear to be:

### Breakpoint set

- create empty breakpoint set
- test whether a breakpoint index is known
- insert a breakpoint index

### State transition store

- test whether a transition exists for exact key `(from, to, line)`
- insert a transition for `(from, to, line)` with:
  - local `cost`
  - total `total`
- replace the currently stored predecessor for `(to, line)` when a cheaper one is found

### Backward reconstruction

- given `(to, line)`, recover the predecessor transition

### Optional debug support

- stringify the stored transitions

Everything else is non-essential for the current algorithm.

## Practical Access Pattern Summary

The runtime access pattern is:

1. start with an empty graph
2. register breakpoints lazily as feasible segments are discovered
3. add inbound labeled transitions as feasible lines are accepted
4. replace an existing predecessor state when a cheaper one is found
5. never traverse forward
6. never perform graph search
7. reconstruct the winning path by repeated predecessor lookups from `(end, line)`

This is not the access pattern of a general graph library.

It is the access pattern of a dynamic-programming predecessor table.

## What Looks Overbuilt Today

From the current inventory, these aspects look larger than necessary for the actual use case:

- storage of `prunedEdges`
- forward-style or neighbor-list style utility functions
- graph-style terminology where the algorithm really needs DP-state terminology
- tests that preserve generic graph behavior not exercised by the linebreaker

Important nuance:

- we do still need logical replacement of an old predecessor by a cheaper one
- what looks overbuilt is not deletion itself, but the old general-purpose `RemoveEdge(...) + prunedEdges` machinery

## What Looks Essential Today

These aspects appear justified by the current algorithm:

- indexing by destination breakpoint
- line-number-labeled predecessor states
- storage of accumulated total cost on the transition
- fast lookup for predecessor of `(to, line)`
- explicit breakpoint registry

## Design Direction Suggested By This Inventory

If the graph is later simplified, the likely target should be something like:

- breakpoint set
- predecessor map keyed by `(to, line)`

rather than a slimmer general-purpose graph.

In other words, the data structure might more honestly be modeled as:

- a set of feasible breakpoints
- a table of best predecessor states

That would align better with the actual Knuth-Plass usage pattern in this rewrite.

## Open Questions For The Next Step

Before changing representation, these are the useful questions to answer:

1. Do we ever need more than one predecessor for the same `(to, line)` in the final design?
2. Is `Edge(from, to, line)` really needed once `kp.paths` remains authoritative?
3. Should total cost live only in `kp.paths`, only in the predecessor store, or in both?
4. Do we still want graph-level tests, or should tests move to algorithm-level state transitions instead?
5. Is the graph still the right abstraction name, or should it be renamed toward “predecessor table” or “breakpoint DAG state”?

These questions should make it easier to judge whether a structural simplification is warranted or whether the current graph is already “small enough”.

## Candidate Representations

Based on the inventory above, there are three realistic directions.

### Candidate A: Keep The Current Shape, But Trim The API

Representation:

- `feasBP map[kinx]struct{}`
- `edgesTo map[kinx]map[origin]edge`

but remove or ignore:

- `prunedEdges`
- `RemoveEdge(...)`
- `To(...)`
- `Cost(...)`
- `StartOfEdge(...)`

What stays:

- exact edge lookup by `(from, to, line)`
- predecessor lookup by scanning `edgesTo[to]` for the right line
- current tests can be updated with minimal disruption

Advantages:

- lowest implementation risk
- almost no algorithm changes required
- preserves the current debug story
- keeps the structure familiar while dead features are removed

Disadvantages:

- still stores the data in a more “graph-like” shape than the algorithm needs
- still keeps a second level map keyed by `origin`
- still conceptually preserves more structure than is actually consumed

Best use:

- if we want to simplify without changing algorithm structure at all

### Candidate B: Predecessor Table Keyed By `(to, line)`

Representation:

- `feasBP map[kinx]struct{}`
- `pred map[state]transition`

with something like:

```go
type state struct {
    to   kinx
    line lineNo
}

type transition struct {
    from  kinx
    cost  merits
    total merits
}
```

Operational reading:

- each reachable `(to, line)` state stores only its best predecessor
- backward reconstruction becomes direct table lookup

Advantages:

- matches the current runtime access pattern best
- removes one level of mapping and the fiction of general edge storage
- makes the “one surviving predecessor per `(to, line)`” invariant explicit
- aligns naturally with `collectOptimalBreakpoints(...)`

Disadvantages:

- `Edge(from, to, line)` no longer has a natural meaning unless reintroduced as a helper
- tests would need to stop thinking in terms of generic edges and start thinking in terms of DP states
- if we later need multiple predecessors per `(to, line)`, this representation would need to grow

Important observation:

The current algorithm already behaves as if this were the real state store:

- `kp.paths[origin{to, line}]` keeps the best path state
- `collectOptimalBreakpoints(...)` only needs the best predecessor for `(to, line)`

So Candidate B is the most honest representation of the present algorithm.

### Candidate C: Slice-Backed Per-Breakpoint State Table

Representation:

- `feasBP []bool` or `map[kinx]struct{}`
- `predTo []map[lineNo]transition`

or, if line counts become predictable enough:

- `predTo [][]transition`
- plus presence bits or sentinel entries

Operational reading:

- index directly by breakpoint `to`
- then by line number

Advantages:

- avoids hashing on `to`
- more cache-friendly than nested maps
- fits the fact that `kinx` is already a dense integer index into the paragraph

Disadvantages:

- line counts are sparse, so fully dense `[][]transition` may waste memory
- more bookkeeping is needed for absent states
- harder to keep elegant while the design is still moving

Best use:

- if the algorithm stabilizes and we later want to reduce allocation and map overhead
- probably premature before the semantics settle

## Tradeoff Summary

### Lowest-risk simplification

Candidate A.

Reason:

- it removes obviously dead parts without changing the mental model of the code

### Best match to current algorithm semantics

Candidate B.

Reason:

- the algorithm is already state-centric, not graph-centric
- the winning-path reconstruction wants `(to, line) -> predecessor`

### Most performance-oriented option

Candidate C.

Reason:

- it takes advantage of dense integer knot indices
- but it is the least justified before the algorithm and API stop moving

## Provisional Recommendation

If we decide to change representation at all, the most sensible sequence looks like this:

1. First, trim the current graph API down to the operations the runtime really uses.
2. Then evaluate whether the remaining shape still wants to be called a graph.
3. If not, migrate toward Candidate B: a predecessor table keyed by `(to, line)`.
4. Only consider Candidate C after the algorithm is semantically stable and parity-tested.

In other words:

- Candidate A is the safest cleanup
- Candidate B is the most coherent redesign
- Candidate C is the possible later optimization

One constraint now needs to be stated explicitly:

- any replacement must support overwriting the predecessor stored for `(to, line)` when a cheaper segment is found

That is the minimal form of “delete” that the old implementation relied on.

## A Concrete Litmus Test

Before changing the representation, a useful question is:

- if `collectOptimalBreakpoints(...)` were the only consumer, what information would it ask for?

The answer today is:

- the final reachable `(end, line)` states
- for each chosen state, the predecessor `from`
- optionally the local and total cost for debugging

That answer is much closer to Candidate B than to a generic graph API.
