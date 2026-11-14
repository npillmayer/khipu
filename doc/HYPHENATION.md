# Discretionaries and Hyphenation

Reference for discretionary breaks: alternatives that split one textbox into a
`PreBreak` and a `PostBreak` fragment. It covers the data model, how the
paragraph owner discovers and shapes alternatives, how the line breakers use
them, and how selected breaks map back to the source. Breaker-specific details
are in [KNUTH+PLASS.md](KNUTH+PLASS.md#lazy-discretionaries) and
[FIRSTFIT.md](FIRSTFIT.md#discretionary-rescue).


## Overview

A discretionary is a replacement pair for an original textbox. The line breaker
never inserts a hyphen glyph itself: fragment text, glyphs and metrics are
prepared upstream by the paragraph owner (`Khipukamayuq`). This keeps the model
open beyond Western hyphenation, for example to spelling changes at a break.

Alternatives are discovered **lazily**: only when a breaker asks for a
particular textbox. Two kinds of request exist:
- **explicit**: the textbox contains a soft hyphen (U+00AD);
- **automatic**: the textbox is a complete word eligible for hyphenation by a
  `Hyphenator` policy.


## Data Model

```go
type HyphenationFragment struct {   // returned by a Hyphenator
    Variant     uint16
    PreBreak    string
    PostBreak   string
    Penalty     Penalty
    SourceSplit SourceSplit         // request-relative original UTF-8 bytes
}

type Hyphenator interface {
    Discretionaries(word string, params *Params) ([]HyphenationFragment, error)
}

type DiscretionaryCandidate struct { // stored on the Khipu, read by breakers
    Variant     uint16
    PreBreak    KnotCore
    PostBreak   KnotCore
    SourceSplit SourceSplit         // paragraph-relative original UTF-8 bytes
}

type DiscretionarySelection struct {
    Source  int                     // knot index of the split textbox
    Variant uint16
}
```

`SourceSplit` is an explicitly optional original-text position: `Valid=false`
means unknown, and `ByteOffset=0` with `Valid=true` is a known split at the
start. It is provenance, never derived from the length of replacement text.

The `Khipu` holds two sparse maps keyed by knot index:
- `DiscretionaryCandidates`: discovered alternatives (a cache);
- `SelectedDiscretionaries`: the choices applied by the last successful
  line-breaking run.

Owner-produced candidates also carry private references to shaped fragment
boxes in the paragraph's arena: replacement text, glyphs, shaping context and
original provenance. Candidates added by hand (`AddDiscretionaryCandidate`)
carry only metrics and are usable by breaker tests, but cannot be mapped back to
the source.


## Discovery

Both breakers call a `DiscretionaryProvider`:

```go
type DiscretionaryProvider interface {
    DiscretionaryCandidates(khp *khipu.Khipu, at int) ([]khipu.DiscretionaryCandidate, error)
}
```

`(*Khipukamayuq).DiscretionaryCandidates` implements it. The breakers know
neither the `Hyphenator` nor the shaper. Without a provider, they use only
candidates already cached on the `Khipu`.

For a request, the owner
1. validates the paragraph, the knot index and its own ownership of the `Khipu`;
2. returns cached candidates, or nothing if the knot is known to have none;
3. decides the kind of request: explicit if the textbox contains a soft
   hyphen, otherwise automatic if the textbox is eligible, otherwise none;
4. asks the `Hyphenator`, validates the answer, shapes all fragments, and
   publishes the complete batch.

### Explicit Requests

For a textbox containing soft hyphens, the owner calls
`Hyphenator.Discretionaries("­", params)` once. This special request asks
for **break material**, not for word segmentation: the returned `PreBreak` is
appended to the text left of a marker, `PostBreak` is prepended to the text
right of it. With material `"-"`/`""`, `co­operate` yields `co-` and
`operate`. Empty material is valid; returning no alternatives declines the
request. No ASCII hyphen is assumed.

Each marker with text on both sides yields one alternative per returned
material; all markers are removed from the replacement strings. The owner
assigns the variants (by source position, then material order) and the source
splits (after the marker); `Variant` and `SourceSplit` from the policy are
ignored. Several markers in one textbox offer alternative single splits, not
multiple splits of the same box.

Explicit requests take precedence: a textbox with a marker never falls back to
automatic discovery, even if the explicit request is declined.

### Automatic Requests

A textbox is **eligible** for automatic discovery if it is a complete default
word segment within one shaping context:
- an ordinary, non-discardable textbox with retained shaping data and
  consistent source coordinates;
- both ends coincide with UAX#29 word boundaries (paragraph edges count), and
  no word boundary is hidden inside one of its clusters;
- its source lies within one retained shaping context; a style or bidi change
  inside a word makes both parts ineligible;
- no soft hyphen (those boxes are explicit).

Eligibility means "one complete word segment", not linguistic hyphenatability:
numbers and punctuation can be segments, and default UAX#29 is not dictionary
segmentation (for example for Thai). The `Hyphenator` decides whether
alternatives exist; no minimum word length or letter test is imposed.

The policy receives the textbox's original text and a copy of the owner's
`Params` whose `Language`, `Script` and `BidiDir` come from the textbox's
shaping context. It must return complete replacement strings with a valid
interior `SourceSplit` at a scalar boundary. Unknown, endpoint, out-of-range or
malformed splits, invalid UTF-8 and soft hyphens in the replacements fail the
whole request. Empty replacement halves are skipped after validation.

Nonzero variant IDs are kept and must be unique; zero IDs get the lowest unused
positive IDs in response order.

### Penalties

A nonzero fragment penalty is used as given. Zero means "unspecified" and falls
back to `Params.Hyphenpenalty`; an explicit zero penalty can therefore not be
expressed. The breakers read the result as `PreBreak.Penalty`.

### Shaping and Publication

Fragments are shaped as complete strings with a fresh streaming shaper each,
using the textbox's retained font, size, direction, script and language. No
whole-paragraph knotting happens, and no paragraph fill is appended.

- Glyph cluster anchors refer to the replacement text, not to the original.
- Widths are accumulated as for ordinary textboxes and rounded once per
  fragment. Fragments currently have fixed widths (`MinW == W == MaxW`).
- Font features covering the whole original textbox apply to both fragments.
  Features limited to part of the textbox are rejected, since their positions
  cannot be mapped onto changed text.
- Fragment `KnotCore.Pos/Len` describe the original source span, not the
  replacement spelling.

All alternatives of one request are validated and shaped before anything is
stored. A failure leaves arena and cache unchanged, so the request can be
retried. Arena IDs stay stable as the arena grows.

### Caching

Working results are cached per bound paragraph and knot:
- non-empty results in `DiscretionaryCandidates`;
- empty results (declined, ineligible, or all alternatives filtered out) in a
  private set, so "asked, none" is distinguished from "not asked yet".

Errors are never cached. Requests without discovery (no `Hyphenator`, not a
textbox, or a `Khipu` without owner) leave no cache entry. A manually added
candidate takes precedence over an empty entry. Releasing a `Khipu` clears the
caches. Caches survive breaker passes and repeated breaking runs.

The cache assumes stable source text, knots, shaping contexts and policy for the
lifetime of the paragraph. To change the policy, create a new owner and encode
again; there is no invalidation for mutable input.

`ExplicitHyphenation` preparations additionally own a lazy cache of complete
immutable shaped replacement pairs, keyed by original textbox index. Measurement
and layout share these pairs, including successful empty responses. Binding does
not populate candidate maps: a binding imports a batch only when its provider is
asked, reconstructing independent glyph/cluster/feature storage and arena IDs.
Manual candidates and selected decisions stay local. A binding created before
measurement can import newly cached pairs later. Releasing a binding clears its
preparation association without affecting the cache or other bindings.

A failed or cancelled batch publishes nothing; earlier completed batches remain
reusable. Automatic-mode preparations and legacy encoding keep their original
binding-local caches. Provider methods still take no context: layout fragment
shaping uses `context.Background()` with internal cancellation on failure, and
cannot be cancelled by the caller. Intrinsic measurement passes its request context
through explicit source scanning and fragment shaping and checks around the
synchronous, non-interruptible material-provider call.

### Intrinsic Measurement of Explicit Breaks

`MeasureIntrinsic` supports ordinary wrapping and bounded `ExplicitHyphenation`.
Count eligible source positions before calling the material provider. More than
one position in a textbox (including consecutive interior markers) is unsupported,
even if the provider would decline. Leading/trailing, marker-only and unmarked
text require no provider call. At one eligible position, validate the complete
response; zero effective break-permitting variants means ordinary measurement,
one permits the split, and two or more return `ErrUnsupportedPolicy`. A variant
permits a break only when its effective penalty is below `MaxPenalty`; zero uses
the owner's default. Empty inserted material is a valid silent break, distinct
from an empty response. No default hyphen is invented.

Min-content combines the complete pre-break fragment with preceding unbreakable
material, closes that span, and carries the post-break fragment into the next span.
The original after-textbox break policy is retained. Max-content uses original
no-break advances. Both keep signed atomic contributions, content-aware trimming
and paragraph-fill exclusion; widths are normalized only after completed spans.

Layout may cache multiple positions/variants that measurement rejects. Its full
batch stays available to layout and prior layout cannot change measurement's
answer. Successive splits of a single original textbox, automatic intrinsic
hyphenation and CSS manual/per-span policy mapping remain deferred. Typography
exposes this same contract; tytui's CSS callback continues using `NoHyphenation`.
Glyph coverage remains the caller's responsibility, with no new validation.


## Line Breaking

Both breakers treat a discretionary as a break inside textbox `At`, identified
internally by `BreakRef{At, Variant}`. The line ending at it contains the
material up to the textbox plus `PreBreak`; the next line starts with
`PostBreak`. The breakers read only `KnotCore` metrics; glyph access is not
part of their interface.

- **Knuth-Plass** requests alternatives in passes 2 and 3: for every textbox
  with a soft hyphen when the scan reaches it, and for one textbox per bad or
  overfull line (the next box for a loose line, the last box on the line for a
  tight one). Costs use `PreBreak.Penalty` plus `FirstHyphenDemerits` and
  `FinalHyphenDemerits`. See [KNUTH+PLASS.md](KNUTH+PLASS.md#lazy-discretionaries).
- **First-fit** requests alternatives for a textbox that makes the line
  overflow, and picks the widest fitting alternative. See
  [FIRSTFIT.md](FIRSTFIT.md#discretionary-rescue).

Neither breaker uses an ordinary break at a soft hyphen: the encoder prohibits
breaks there, and the marker is only reachable as a discretionary.

Fragments returned lazily to Knuth-Plass must have nonnegative minimum widths
within a safe range; see [KNUTH+PLASS.md](KNUTH+PLASS.md#retirement).


## Selection and Source Mapping

A successful breaker run replaces `SelectedDiscretionaries` with the choices of
its result; a failed run leaves the previous choices. The candidate cache is
not pruned to the chosen alternatives.

`Khipu.BreakpointPositions(breakpoints)` maps a breaker result to exclusive
UTF-8 byte positions in the original paragraph (paragraph-relative, without
`paragraph.Offset`). Call it before another breaking run replaces the
selections.
- Ordinary breaks map through the retained source-boundary provenance,
  including glyphless gaps; source whitespace stays included regardless of
  trimming.
- Selected discretionaries map through their `SourceSplit`, never through the
  length of the replacement text.
- The paragraph fill maps to the paragraph end.

The mapper accepts increasing knot indices and does not check that the path is
feasible. Missing or ambiguous candidates, unknown splits or decreasing
positions return an error and no partial result. Knots without retained
provenance (hand-built knots) cannot be mapped; `Pos`/`Len` are not used as a
fallback.

Later stages (logical lines, bidi ordering, freezing) consume these positions;
see [FREEZE.md](FREEZE.md).


## Soft Hyphens During Encoding

The encoder removes U+00AD from the text before shaping, so markers do not
disturb ligatures or joining, and maps glyph anchors back to original
positions. The source-boundary table keeps the marker positions but prohibits
ordinary breaks there and suppresses their word-boundary hints. Ordinary spaces
and mandatory breaks right before a marker keep their own break positions.
Marker-only input produces no glyphs. Encoding never discovers alternatives.


## Design Decisions

**Lazy discovery through an interface.** Hyphenating every word up front would
shape many alternatives that are never used, and tied the package to a
concrete hyphenation library. Breakers request alternatives only where they
need them; the policy is an interface.

**Storage in the `Khipu`.** Candidates and selections are sparse tables on the
paragraph, not an external cache or a breaker-specific view: choices must
survive into freezing, and discovered alternatives are reused across passes
and breakers.

**Provenance separate from spelling.** Replacement text may differ from the
original (spelling changes, inserted hyphens). Source positions are therefore
carried explicitly (`SourceSplit`) instead of being inferred from fragment
lengths.

**Whole-batch publication.** All alternatives of a request are published
together or not at all, so a failed request leaves no partial state and can be
retried.


## Limits

- One split per textbox; no replacements spanning several textboxes (for
  example across a style change). Both would need a multi-box replacement model
  in the breakers.
- Fragments have fixed widths; no script-specific adjustment capacity (such as
  kashida) is obtained from the shaper.
- No complete script-sensitive validity policy for fragments. Breakers must not
  impose Western character-count rules; such policies belong to the
  `Hyphenator`.
- Features limited to part of a textbox prevent its alternatives.
- An explicit zero fragment penalty cannot be expressed.
- No minimum fragment length. A future policy must first define which fragment
  it constrains and how length is measured (characters, clusters or width).
- No cancellation by the caller during discovery.
- No consecutive-hyphen demerits or limits (see
  [KNUTH+PLASS.md](KNUTH+PLASS.md#known-limits-and-deviations-from-tex)).
