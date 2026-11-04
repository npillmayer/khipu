# Lazy Discretionaries: Implementation And Ownership

Reviewed against the code on November 4, 2026.

## Current Contract

`Khipukamayuq` owns discretionary discovery and shaping. A discretionary is a
replacement pair for an original textbox: `PreBreak` and `PostBreak`.
The linebreaker never inserts a hyphen glyph. Fragment content and metrics are
prepared upstream, allowing the model to evolve beyond Western hyphenation.

`Khipu` owns two sparse maps keyed by knot index:

- `DiscretionaryCandidates` caches discovered alternatives.
- `SelectedDiscretionaries` records only applied choices from linebreaking.

The base SOA includes `Flags` and `KFDiscardable`; this is implemented, not a
proposed extension. Both modern linebreakers consume those flags.

## Implemented Interfaces

The base package declares:

```go
type HyphenationFragment struct {
    Variant   uint16
    PreBreak  string
    PostBreak string
    Penalty   Penalty
}

type Hyphenator interface {
    Discretionaries(word string, params *Params) ([]HyphenationFragment, error)
}

type DiscretionaryCandidate struct {
    Variant   uint16
    PreBreak  KnotCore
    PostBreak KnotCore
}

type DiscretionarySelection struct {
    Source  int
    Variant uint16
}
```

Each modern linebreaker defines its own structurally compatible provider:

```go
type DiscretionaryProvider interface {
    DiscretionaryCandidates(khp *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error)
}
```

`(*Khipukamayuq).DiscretionaryCandidates` implements that method.
The linebreaker need not know the `Hyphenator` or `Shaper`.
Without a provider, breakers can consult candidates already stored on `Khipu`.

Widths travel directly in `KnotCore.W/MinW/MaxW`; no extra transport or adapter
type was introduced. The shaper currently provides cluster metrics through
`SetSource`, `SetStyle`, and `ReadClusterMetrics`. Cluster anchors are run-relative
UTF-8 byte offsets in logical order, reset for each fragment. Repeated anchors
accumulate width; zero-width anchors still mark boundaries; EOF has no metric.

## Lazy Roundtrip

For a textbox requested by a breaker, `Khipukamayuq`:

1. Validates the paragraph/index and ownership, and reuses nonempty cached candidates.
2. Recovers textbox source text and its style from the owning paragraph.
3. Calls `Hyphenator.Discretionaries`.
4. Skips alternatives with an empty pre-break or post-break string.
5. Shapes each fragment under the original style and measures its width.
6. Assigns a nonzero variant ID if the provider returned zero.
7. Sets `PreBreak.Penalty` and caches the resulting pair.

A nonzero fragment penalty is used directly; zero currently means fallback to
`Params.Hyphenpenalty`. Thus an explicit zero override cannot presently be
distinguished from an unspecified penalty.

Currently shaped fragments have fixed widths: `MinW == W == MaxW`.
The roundtrip does not yet obtain script-specific adjustment capacities or
penalty hints from the shaper.

## Knuth-Plass Integration

Pass 1 runs without discretionary lookup. If the first result is unacceptable
or unavailable, pass 2 uses `Tolerance` and permits local lookup.

During pass 2, geometrically feasible line evaluations with
`badness > PreTolerance` can trigger lookup, including candidates which still
pass `Tolerance`. This is a badness threshold, not a demerit threshold.
Loose lines scan forward for a textbox; tight lines scan backward, skipping
flagged discardables and stopping at other retained material.

Internal `BreakRef{At, Variant}` values identify ordinary boundaries (variant
zero) and discretionary alternatives (nonzero variant). Different variants
can coexist at a source knot. Path construction evaluates the pre-break
fragment and seeds the next line with the post-break fragment.

Local cost uses `PreBreak.Penalty`. `FirstHyphenDemerits` adds cost when the
discretionary line starts at the root. `FinalHyphenDemerits` adds cost on the
terminal transition if the break starting the final line is discretionary.
Both are implemented. `DoubleHyphenDemerits` remains reserved and unused.

## Firstfit Integration

Firstfit has a separate, greedy policy with no second pass. When an overflowing
textbox would otherwise force a break at an earlier feasible checkpoint, it
can request candidates for that textbox.

A candidate must fit according to minimum width and increase the line's
natural width over the plain checkpoint by at least `MinHyphenGain`.
Among survivors it chooses the greatest resulting natural width, breaking
ties by lower `PreBreak.Penalty`. The next line starts with `PostBreak`.

Tight-line rescue, consecutive-hyphen limits, and global demerit ranking are
outside its current scope. See [FIRSTFIT.md](FIRSTFIT.md).

## Committed Result

Both modern breakers return knot indices and store applied discretionary
selections separately in `Khipu`. Reruns clear stale choices.
Candidate caches are not pruned to the winning alternatives.

For Knuth-Plass, the winning `[]BreakRef` is authoritative for selection
write-back. Firstfit commits each greedy choice directly. The resulting
`Source` and `Variant` identify the chosen cached candidate.

The next consumer is planned freezing, followed by box creation and later
rendering. No renderer-specific selection helpers or post-processing API
have been implemented. See [FREEZE.md](FREEZE.md).

## Ownership And Deferred Work

Fragment validity and length filtering belong to `Khipukamayuq`, with
script/shaping knowledge supplied by its collaborators. Empty fragments are
filtered; invalid UTF-8 or fragments exceeding the `uint16` byte-length field
return errors during shaping. A complete script-sensitive validity policy is
still missing. Breakers should not impose Western character-count rules.

The cache assumes stable base knots, shaping context, and variant identities
within a paragraph instance. Nonempty results are reused; empty results are not
currently negative-cached. Cache invalidation and transactional population on
shaping errors remain future hardening topics.

The current payload retains metrics and source-like positions, not a durable
glyph sequence or fragment string. In particular, post-break position is
computed from the byte length of the pre-break string. For altered spellings
or inserted glyphs this is not a general source mapping. Freezing/shaping
design must settle fragment provenance before downstream consumers rely on
these fields as faithful original-text ranges.

There is no separate explicit-hyphen cost model in the new path. The producer
supplies the candidate penalty; richer script metadata and shaper feedback
can be added when their concrete requirements are known.

## Why This Design Was Chosen

The removed `HyphenateTextBoxes` path performed eager hyphenation and
imported its concrete hyphenation dependency. The modern path uses an interface
and requests candidates only when a breaker needs them.

Of the original storage options, sparse in-`Khipu` tables were chosen because
opportunities can be discovered lazily while applied decisions remain attached
to the paragraph for subsequent processing. Neither penalties packed with
flag bits nor a separate consumer view was introduced.

Tests cover storage, shaping and cache reuse, provider invocation, multiple
variants, discretionary costs, and selection persistence. Full script-sensitive
freezing and box creation are not established by those tests.
