# Lazy Discretionaries: Implementation And Ownership

Reviewed against the code on November 4, 2026.

## Streaming Bridge: Design And Next Steps

The streaming encoder now produces the same width/penalty/flag planes consumed
by both linebreakers, plus textbox shaping storage. Ordinary breaking requires
no glyph-aware algorithm. Narrow-width integration tests cover actual wrapping,
trimming, forced breaks and unchanged base knots. Lazy streaming discretionaries
are still guarded by `ErrStreamingDiscretionariesUnsupported`: the metric
fragment shaper must not measure alternatives for streaming text.

### Initial Request Scope

Keep the existing knot-index provider interfaces initially. Accept a request
only when the textbox contains a complete eligible source word in one retained
shaping context. Use the original source and independent word-boundary metadata,
not whitespace or the mere existence of a textbox boundary. A style or bidi
boundary inside a word is not a word boundary. Decline such multi-box words
without silently hyphenating only one fragment. This eligibility policy is
proposed, not implemented; language-specific segmentation remains upstream.

Extending a request across boxes requires a separate design: the current breaker
substitutes one knot with PreBreak/PostBreak. A multi-box replacement would need
source-range identity and corresponding width/bookkeeping changes in both
breakers, not merely a larger string passed to the hyphenator.

### Source Mapping Must Be Explicit

`HyphenationFragment.SourceSplit` now carries an optional original split offset.
Its `ByteOffset` uses UTF-8 bytes relative to the requested text; `Valid`
distinguishes unknown from a known zero. `DiscretionaryCandidate.SourceSplit`
uses original paragraph-relative UTF-8 bytes instead. Legacy metric callers may
omit these fields; their existing shaping path does not yet propagate the split.
PreBreak length cannot supply this offset when text is inserted, removed or changed.

The private `prepareDiscretionaryInput` foundation validates UTF-8, original
byte/scalar ranges and the explicit split at a scalar boundary. It separates
replacement strings and local scalar counts from original paragraph provenance.
The two provenance intervals partition the request, not the replacement input.
Tests cover changed spellings, multibyte input, a removed soft-hyphen marker,
known zero, empty fragments, invalid metadata and large coordinates. Endpoint
splits are valid provenance, not automatically eligible hyphenation opportunities.
This helper is not yet wired into discovery, shaping or candidate publication.

Keep replacement text and original provenance separate. The fragment shaper's
scalar clusters describe its replacement input, not the original paragraph.
For range-limited OpenType features, arbitrary replacement text also needs a
mapping or explicit feature-resolution policy. A split offset alone is not
enough to map all internal feature positions. Whole-fragment settings can be
supported first; ambiguous mappings must be rejected rather than guessed.

### Shape, Cache And Select

1. Recover original text, word bounds and the retained font/scale/direction/
   script/language context from the requested box. Do not reconstruct it from
   the owner's unrelated metric defaults.
2. Query the hyphenator lazily, validate source split coordinates and replacement
   strings, and apply upstream fragment filtering and penalty policy.
3. Create fresh streaming shapers through the configured factory. Shape the
   replacement fragments directly; do not run whole-paragraph knotting or append
   paragraph-fill knots. Reuse the producer lifecycle and width-conversion rules.
4. Build fragment metrics and owned replacement-input/glyph/context records
   privately. Publish a complete batch to the arena/cache only after every
   required shaping operation succeeds. Keep IDs stable and immutable.
5. Extend candidate storage with optional pre-/post-break box references while
   preserving the metric PreBreak/PostBreak payload expected by the breakers.
   A selected `(Source, Variant)` must resolve both metrics and stored fragments.
6. Remove the streaming guard only after end-to-end tests demonstrate equivalent
   measured and retained fragment output. Preserve the legacy metric path.

Ordinary break results remain knot indices. Downstream line extraction and bidi
splitting must consult retained exact break provenance, not `Pos + Len`;
discretionary choices must use their explicit original split position. No
renderer helpers are needed for this bridge.

### Suggested PR Sequence

- Source split/replacement-input contracts and validation tests are implemented.
  Feature mapping and single-word/single-context eligibility remain to be wired.
- Pursue explicit U+00AD opportunities before automatic streaming hyphenation,
  initially in one context. Automatic discovery must not cross structural/style
  boundaries or hyphenate their partial-word boxes. Cross-style manual breaks
  require a later multi-box replacement design.
- Implement standalone streaming fragment shaping and atomic arena publication;
  test backend failures, glyph omission, changed spellings and owned storage.
- Wire the owner provider, then test both breakers' choices and retained selected
  fragments. Add cache lifetime/invalidation rules before supporting mutable input.

Apart from the source-input foundation described above, this section is a
proposal. It does not enable streaming hyphenation or imply
that default Unicode word segmentation alone handles Thai or other scripts
requiring language-specific analysis.

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
    SourceSplit SourceSplit // request-relative original UTF-8 bytes
}

type Hyphenator interface {
    Discretionaries(word string, params *Params) ([]HyphenationFragment, error)
}

type DiscretionaryCandidate struct {
    Variant   uint16
    PreBreak  KnotCore
    PostBreak KnotCore
    SourceSplit SourceSplit // paragraph-relative original UTF-8 bytes
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
