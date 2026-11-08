# Lazy Discretionaries: Implementation And Ownership

Reviewed against the code on November 4, 2026.

## Streaming Bridge: Design And Next Steps

The streaming encoder now produces the same width/penalty/flag planes consumed
by both linebreakers, plus textbox shaping storage. Ordinary breaking requires
no glyph-aware algorithm. Narrow-width integration tests cover actual wrapping,
trimming, forced breaks and unchanged base knots. Lazy streaming lookup supports
explicit U+00AD and automatic alternatives for structurally eligible source
segments in one retained textbox/context. Configure the Khipukamayuq as the
breaker's `DiscretionaryProvider`. Ineligible boxes return no alternatives;
streaming lookup never falls back to metric shaping.
`ErrStreamingDiscretionariesUnsupported` guards missing streaming configuration
when fragment shaping is needed.

### Initial Request Scope

The private `Khipu.automaticHyphenationEligible` predicate now implements the
structural prerequisite for automatic streaming discovery. It requires
an ordinary, non-discardable textbox with retained original shaping storage,
consistent byte/scalar source coordinates, and a source range contained in one
retained context. Both endpoints must coincide with original UAX#29 word hints
(paragraph edges count), with no internal word hint hidden by a shaped cluster.
It never uses the line-break penalty or `breakAfter` as a word-boundary proxy.

A style or bidi boundary inside a word is not a word boundary. Both partial
textboxes are declined, even when their fonts happen to agree; no neighbors are
concatenated. A style boundary at an actual word boundary is acceptable.
Glyphless source gaps must not truncate the source segment. Boxes containing
U+00AD are reserved for explicit discovery, avoiding competing variant namespaces.
Manually constructed nodes without provenance and generated replacement boxes are not eligible here.

True means one complete default word *segment*, not proven linguistic
hyphenatability. Punctuation and numbers may also form complete segments, and
default UAX#29 is not dictionary segmentation for all scripts. The downstream
hyphenator must decide whether any alternatives exist; no Western letter test
or minimum word length is imposed by this metadata-only predicate. Feature
mapping and fragment validity are separate checks after structural eligibility.

The predicate allocates no text, invokes no collaborators and changes no state.
The provider now applies it before automatic requests. Explicit markers take
precedence and never fall back to automatic discovery, even when declined.

Automatic requests read only the bounded original textbox and pass a local
parameter copy using its retained Language, Script and BidiDir. The policy must
return complete replacement strings with known interior original-byte splits at
UTF-8 scalar boundaries. Unknown, endpoint, out-of-range or malformed splits
fail the batch; replacement lengths cannot supply a missing split. Invalid UTF-8
and recursive soft-hyphen material also fail. Empty replacement halves are
skipped after validation. No new
Western character-count minimum is imposed; script-specific validity remains
with the policy.

Nonzero policy variant IDs are retained and duplicates rejected. Zero IDs receive
the lowest unused positive IDs in response order, reserving all supplied IDs
first. Exhausting the uint16 identity space is an error. All response metadata
is validated before any shaping; all shaped alternatives are staged before
publication. Failures leave arena/cache unchanged and can be retried.

Extending a request across boxes requires a separate design: the current breaker
substitutes one knot with PreBreak/PostBreak. A multi-box replacement would need
source-range identity and corresponding width/bookkeeping changes in both
breakers, not merely a larger string passed to the hyphenator.

### Source Mapping Must Be Explicit

`HyphenationFragment.SourceSplit` now carries an optional original split offset.
Its `ByteOffset` uses UTF-8 bytes relative to the requested text; `Valid`
distinguishes unknown from a known zero. `DiscretionaryCandidate.SourceSplit`
uses original paragraph-relative UTF-8 bytes instead. Automatic policies must supply a valid split; replacement lengths are never a fallback.

The private `prepareDiscretionaryInput` foundation validates UTF-8, original
byte/scalar ranges and the explicit split at a scalar boundary. It separates
replacement strings and local scalar counts from original paragraph provenance.
The two provenance intervals partition the request, not the replacement input.
Tests cover changed spellings, multibyte input, a removed soft-hyphen marker,
known zero, empty fragments, invalid metadata and large coordinates. Endpoint
splits are valid provenance, not automatically eligible hyphenation opportunities.
The explicit provider now connects this helper to shaping and publication.

Keep replacement text and original provenance separate. The fragment shaper's
scalar clusters describe its replacement input, not the original paragraph.
For range-limited OpenType features, arbitrary replacement text also needs a
mapping or explicit feature-resolution policy. A split offset alone is not
enough to map all internal feature positions. Whole-fragment settings can be
supported first; ambiguous mappings must be rejected rather than guessed.

### Shape, Cache And Select

The private `shapeDiscretionaryInput` stage is now implemented independently of
the owner provider. It accepts validated replacement input and explicit pre/post
run parameters, creates a fresh stream for each side, and returns an owned pair
only after both streams and width conversion succeed. Producers run in
goroutines; the caller consumes glyphs and cancels/joins on failure. Empty and
glyphless fragments retain their input and provenance with zero width.

Each result retains replacement text, original-source provenance, shaping
context, glyph IDs, advances and offsets. Cluster anchors are local replacement
scalar positions, nondecreasing and in range; repeated anchors retain cluster
groups. No per-character mapping back to original text is inferred. Horizontal
advances are accumulated in fractional DU and rounded once per fragment using
the ordinary candidate-width rules. Current fragment widths are fixed
(`W == MinW == MaxW`); offsets do not contribute to advance.

Only features with both bounds nonpositive (whole-input settings) are accepted.
Positive bounds are conservatively rejected before factory invocation, even if
they happen to cover the original run. Prepared-run feature extraction/rebasing
is not implemented here. Each result owns its feature slice; font/scale services
remain shared immutable objects. The function inserts no hyphen itself.

Shaping itself does not write to the arena or candidate cache. The separate
private `Khipu.publishDiscretionary` stage now stores a successful pair and its
candidate atomically with respect to returned errors. It validates both sides,
their source partition, metrics and target textbox before appending anything.
Duplicate variant IDs are rejected rather than replacing existing records.
Publication is single-owner, not concurrent-safe; allocation failures are not
recoverable transaction errors. The provider stages every alternative privately
before committing a complete batch. A failure in a later fragment leaves both
arena and cache unchanged, allowing a clean retry.

Candidates retain optional private `preBox`/`postBox` arena IDs. Zero means a
metric-only candidate. Ordinary knot associations and metrics remain unchanged.
Replacement boxes retain generated input separately from original provenance;
their glyphs, cluster ranges and context ranges use replacement-local coordinates.
Empty replacement input is distinct from an ordinary box and may have no glyphs.
Glyph/feature slices are copied; IDs remain stable across append-only arena growth.
Streaming fragment `KnotCore.Pos/Len` describe original source spans (Pos includes
the paragraph offset), not replacement spelling. The selected `(Source, Variant)`
still identifies the candidate carrying both metrics and stored box references.

These stages themselves do not discover markers; the explicit provider now
connects marker preparation, shaping and batch publication. Renderer-facing
access remains future work.

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
5. Candidate storage now has optional pre-/post-break box references while
   preserving the metric PreBreak/PostBreak payload expected by the breakers.
   A selected `(Source, Variant)` must resolve both metrics and stored fragments.
6. Explicit and automatic streaming lookup are enabled and covered end to end.

Ordinary break results remain knot indices. Downstream line extraction and bidi
splitting must consult retained exact break provenance, not `Pos + Len`;
discretionary choices must use their explicit original split position. No
renderer helpers are needed for this bridge.

### Suggested PR Sequence

Explicit fragment preparation is now available privately through
`Khipukamayuq.prepareExplicitDiscretionaries`. For one caller-established
textbox/context it requests `Hyphenator.Discretionaries("\u00ad", params)` once.
This special request supplies break material rather than discovering word
boundaries: PreBreak is appended to the left text, PostBreak prepended to the
right. For example, material `"-"`/`""` produces `"co-"`/`"operate"`, which are
shaped as complete fragments by the existing common stage. There is no implicit
ASCII-hyphen default. Nil policy or no returned alternatives declines preparation;
one alternative with empty material still represents an explicit opportunity.

All soft-hyphen markers are removed from replacement strings. Original source
splits remain after their selected marker, including it in pre-break provenance.
Markers with no remaining text on either side are ignored. Returned material
must be valid UTF-8 and contain no recursive soft-hyphen markers. Variants are
numbered by source position then material order, independently of policy variant
IDs; they must not yet be merged with a separate automatic-variant namespace.
Penalties follow the existing owner policy and default fallback. This simple
attachment mechanism does not implement contextual spelling changes.

Public streaming encoding now implements the no-break side of U+00AD. A reader
filters markers before shaping; it does not discard glyphs afterward. Compatible
contiguous bidi intersections separated only by markers are coalesced within an
original style span and direction, so a marker does not itself interrupt shaping.
Style changes, direction changes and source gaps remain separate. Transient
scalar maps restore backend anchors to original paragraph coordinates before
cluster assembly, including clusters spanning markers. Feature ranges are mapped
to surviving input scalars for submission; retained contexts keep original ranges.

The retained source-boundary table flags explicit markers even inside clusters,
but suppresses their ordinary penalties and knot-partition hints. Original byte
and scalar endpoints remain unchanged. Ordinary spaces and mandatory breaks
immediately preceding markers retain their own break positions. Marker-only
input produces no text glyphs. This changes only streaming encoding, not the
standalone synthetic shaper's character policy.

Provider wiring is now active. K&P requests retained explicit opportunities when
pass 2 reaches their textbox, before evaluating its cached variants. Firstfit
requests them on overflow; an explicit marker may salvage a first word without
an earlier plain checkpoint. Its existing fit and MinHyphenGain rules still
apply. K&P retains its pass-1 acceptance and demerit policies, so an explicit
marker is available, not mandatory. Neither breaker uses an ordinary penalty-only
break at a marker. Selected variants retain their source split and arena IDs.

The provider reads only the target textbox's bounded source, reuses its retained
font/scale/direction/script/language, and normalizes features covering the whole
box to whole-fragment settings. Interior feature boundaries are rejected rather
than guessed. Provider methods still lack a context parameter; internal shaping
uses Background with cancellation/joining on failure, not caller cancellation.
Successful nonempty results are cached. Empty/declined requests may be retried.
Cross-style/multi-box replacements and repeated breaks within one original box
are not supported. Multiple markers currently supply alternative single splits.

- Source split/replacement-input contracts, structural single-segment/context
  eligibility and automatic provider wiring are implemented and tested. Richer
  feature mapping remains future work.
- Pursue explicit U+00AD opportunities before automatic streaming hyphenation,
  initially in one context. Automatic discovery must not cross structural/style
  boundaries or hyphenate their partial-word boxes. Cross-style manual breaks
  require a later multi-box replacement design.
- Fragment shaping, batch publication, explicit/automatic provider integration
  and both breakers' selection paths are implemented and tested. Cross-style
  explicit replacements remain a separate extension.
- Add cache lifetime/invalidation rules before supporting mutable input.

Apart from the source-input, shaping and storage foundations above, this section
is a proposal. It does not enable streaming hyphenation or imply
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
The linebreaker need not know the `Hyphenator` or `StreamingShaper`.
Without a provider, breakers can consult candidates already stored on `Khipu`.

Widths travel directly in `KnotCore.W/MinW/MaxW`; no extra transport or adapter
type was introduced. The streaming shaper supplies logical-order glyphs with
scalar anchors and font-unit positioning; the owner scales advances to DU and
stores original-source provenance separately from replacement text.

## Lazy Roundtrip

The owner validates paragraph/index/ownership, reuses cached candidates, and
otherwise uses the shared pipeline described above:
bounded source access, retained contexts, full-fragment streaming shaping and
atomic batch publication. Explicit boxes request marker material; eligible
unmarked boxes request complete-word alternatives. The existing breaker triggers
are unchanged for automatic lookup: K&P's bad-line pass-2 requests and firstfit's
loose-line overflow requests after an earlier checkpoint. This does not add an
automatic first-word rescue to firstfit or revise K&P's geometric screening.

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
Both are implemented. Consecutive-discretionary demerits remain deferred; the
unused `DoubleHyphenDemerits` parameter has been removed.

The unused `Params.Hyphenchar` and `Params.Minhyphenlength` fields have also
been removed. Replacement text remains the provider's script-sensitive decision,
not a universal injected character. A future minimum-length policy needs an
explicit definition of which fragments it constrains and how length is measured;
no generic count threshold is currently imposed by these parameters.

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

`Khipu.BreakpointPositions(breakpoints)` now maps these decisions to exclusive
UTF-8 byte endpoints in the original paragraph. Call it before another breaker
run replaces the selections. It resolves the selected `Source`/`Variant` to its
cached `SourceSplit`, not to a replacement length. Ordinary streaming breaks use
retained boundary provenance, including glyphless gaps; the terminal fill maps
to paragraph end. Positions are paragraph-relative, without `paragraph.Offset`.
Source whitespace remains included, independently of line-edge trimming.

The mapper accepts increasing knot indices, including partial or empty lists;
it does not certify path feasibility. Missing/ambiguous candidates, unknown or
invalid splits, or decreasing source endpoints return an error and no partial
result. It neither discovers alternatives nor changes the paragraph. Manually
constructed knots without retained provenance cannot be mapped, even if their
Pos/Len fields appear to describe source spans.

The next consumer is planned line-local bidi ordering and freezing, followed by
box creation and later rendering. No bidi splitting, line materialization, or
renderer-specific helpers are included in this mapper. See [FREEZE.md](FREEZE.md).

## Ownership And Deferred Work

Fragment validity and length filtering belong to `Khipukamayuq`, with
script/shaping knowledge supplied by its collaborators. Storage allows empty/silent
fragments and separates replacement text from original source spans; original
spans must fit the KnotCore length field. Preparation validates UTF-8. Explicit discovery requires nonempty surrounding text, but
permits empty break material. A complete script-sensitive validity policy is
still missing. Breakers should not impose Western character-count rules.

The cache assumes stable base knots, shaping context, and variant identities
within a paragraph instance. Nonempty results are reused; empty results are not
currently negative-cached. Cache invalidation remains future work. Streaming
explicit and automatic population is transactional across the full request batch.

Manually supplied metric-only candidates remain usable by breaker fixtures but
have no implicit source mapping. Owner-produced candidates retain owned glyphs, replacement strings, contexts
and original-source provenance through private arena references. Their explicit
`SourceSplit` is independent of replacement spelling and is exposed for selected
breaks through `BreakpointPositions`. Applying those positions to bidi resolution
and materializing line contents remain downstream integration tasks.

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
