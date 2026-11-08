# Overall Algorithm to Shape Text into Type

This document describes how a styled paragraph of text is turned into lines of
positioned glyphs. It describes the intended design. How far the code has got,
known deviations from the design and limitations are collected in
[Implementation Status](#implementation-status) at the end.

The process has five phases:

1. **Knotting** converts the styled paragraph from characters into glyphs and
   groups the glyphs into an elastic, logically ordered sequence of knots, the
   *khipu* `K`.
2. **Line breaking** chooses a sequence of breakpoints in `K`.
3. **Line-local bidi ordering** computes the visual order of each chosen line.
4. **Freezing** distributes the remaining width of each line among its elastic
   knots and produces final, fixed-width lines.
5. **Visual representation** (box construction, glyph placement, rendering)
   happens later and is outside the scope of this module.


## Input Model

The input is a sequence of runes forming a styled paragraph of text. Styles are
nested, i.e. they form a hierarchy. Examples of styles are "*boldface*", a font
change or a color change. Styles are projected onto the runes out-of-band. In a
previous stage an HTML parser will have converted HTML and CSS into a styled
paragraph, where the style hierarchy is held in a tree-like structure. For every
rune in the input sequence, the style hierarchy can tell the exact style that
applies to it.

The paragraph carries a bidi resolution (UAX#9) computed for the whole
paragraph. Original source bytes are preserved throughout all phases: every
position used by this algorithm is a half-open UTF-8 byte offset (or a scalar
index) into the original paragraph. If text is to be normalized, normalization
must happen before styled ranges are established.

Non-discardable inline items (e.g. inline-blocks or images) are intended to
enter the algorithm as fixed-size boxes, unaffected by the current style. Their
representation in the input and in the khipu is still an
[open question](#inline-objects).

The first phase converts the rune sequence into a sequence of glyphs. Glyphs are
selected from a font. Each glyph carries positioning information, sizing
information, a cluster ID and other metadata. In general, we translate from the
realm of characters (input paragraph) to the realm of glyphs (khipu).

The rune sequence is broken into runs. Runs are the input for text shaping
(with a tool like HarfBuzz). All runes of a single run share the same shaping
parameters.


### Param-Environments

Each run `R` is associated with a Param-Environment `P`, which determines
properties for each rune `c` in the run: `P(c) = P(R)` for every `c` in `R`.
The properties of `P` are:

- selected `font` and its `emSize`
- the font `features` to apply (OpenType feature settings, possibly limited to
  sub-ranges of the run)
- current script `script`
- current language `lang`
- current `bidi` direction (LTR or RTL)
- treatment of inter-word spaces: `spacing`, either `elastic` or `fixed`

The top-level paragraph has `P(¶)`, which provides default values for all
properties. Defaults are applied independently to every style span; they are
never inherited from the preceding span.

Example: for a rune `c`, `P(c).font` denotes the font associated with `c`
through the Param-Environment active for the run containing `c`.

Param-Environments result from the paragraph's style hierarchy. Logically, the
hierarchy is flattened into a sequence of property sets. This does not restrict
the memory layout, but conceptually every run is associated with a full set of
properties relevant for paragraph layout.

Styles are always translated into concrete properties by a style resolver, e.g.
"*bold*" is translated into a bold font face. `P(R)` is uniform for all
properties that influence shaping.

Script changes are expressed as style spans. Run preparation does not detect
scripts automatically or subdivide runs for them; the style resolver supplies
script and language.


### Bidi Intervals

Paragraphs may contain character sequences in mixed LTR and RTL scripts. These
sub-sequences are described as bidi intervals, denoted `B`. Bidi intervals are
independent of (orthogonal to) style spans. Each `B` has a direction, either
`LTR` or `RTL`. Isolates form intervals of their own, even if style and
direction agree with their neighbors.

The paragraph's resolved embedding levels are retained for line-local ordering
after line breaking. Runs do not store a numeric embedding level; they only
store the direction needed for shaping and later glyph placement.

Shaping runs are the intersections of bidi intervals with style spans. When a
style span crosses a bidi interval boundary, the span is split.

Example: a paragraph of English text contains two Hebrew words. The second
Hebrew word and the first English word after it are set in boldface. The bidi
interval boundary after the second Hebrew word splits the bold span in two: the
Hebrew word is still bold, and so is the following English word, but they are
shaped as two separate runs.

Bidi control characters are not passed to the shaper. They still occupy
positions in the source, so runs may be separated by gaps.


### Clusters

Every output glyph `g` "stems from" one or more characters `c` of `R`. This
relation is expressed through cluster IDs. A cluster ID is the original scalar
index of the first character of the cluster, so glyphs can always be related
back to the original source, across all runs.

A cluster is the smallest unit that may be associated with a source range.
Knots never split a cluster, and no line break may occur inside one.

Bidi interval boundaries and `P(R)` remain associated with every cluster for
later stages (reshaping for hyphenation, glyph placement).


### Unicode Annexes

Segmentation of the rune sequence follows the Unicode annexes. Three are
relevant:

- **UAX#14 (line breaking)** yields a penalty after every character. `U(c)`
  denotes the line-break penalty after input character `c`. A prohibited break
  has `U(c) = ∞`, a mandatory break has `U(c) = -∞`. Hard-break classes (BK,
  LF, NL, CR) are always mandatory, even where the additive rule contributions
  would cancel each other out; CR LF counts as one separator.
- **UAX#29 (word boundaries)** yields boolean word-boundary hints. These are
  kept separate from line-break penalties: a word boundary does not permit a
  line break, and a line-break opportunity need not coincide with a word
  boundary. Word hints only partition knots; they never influence demerits,
  glue classification, elasticity or discardability.
- **UAX#9 (bidi)** is used for run construction (see above) and again for
  line-local ordering after line breaking.

Interpreting the annexes belongs to the character realm. The analysis runs over
the complete, intact paragraph before shaping starts; style and bidi boundaries
never reset it. The result is a **source-boundary table** with one entry after
every original scalar, containing the byte and scalar end positions, the final
line-break penalty, the word-boundary flag and a soft-hyphen flag. Characters
not passed to the shaper (bidi controls) keep their entries. The end of the
paragraph has no entry of its own; it is represented by the final fill knot.

The table is metadata. A finite penalty does not permit a break inside a shaped
cluster.


## Pseudo-Code Notation

- "[ … ]" denotes pseudo-code lines or blocks
- ":" introduces a sub-block
- "//" denotes code comments
- ":=" details a block into sub-blocks

The algorithm relies on a streaming shaper: it reads runes on the input end and
pushes glyph and positioning information into a buffered channel on the output
end. This decouples the character realm from the glyph realm and makes the
responsibilities of both sides easier to reason about. The shaper always
produces glyphs in logical order, also for RTL runs.


## Phase 1: Khipu-Knotting

Knotting is all-or-nothing: either the complete paragraph is published as a
khipu, or an error is returned and nothing is published.

### Character Input End

```
[ Analyze the complete original paragraph into the source-boundary table ]
[ Suppress ordinary breaks at soft hyphens (U+00AD); keep their positions ]
[ Intersect bidi intervals with style spans ]
[ Resolve each contributing style span once; prepare run parameters P(R) ]
[ Publish the immutable run and boundary tables before starting the producer ]
[ In the producer goroutine, for every prepared run `R` in logical order ] :
    [ Check cancellation and cluster-ID bounds ]
    [ Set the shaper's initial cluster to R's original scalar start ]
    [ Call Shape with P(R) and a reader over R's original text, omitting U+00AD ]
    // The shaper assigns cluster anchors; source penalties are already final
[ Finish the stream; report producer completion separately from glyph output ]
```

with

```
[ Intersect bidi intervals with style spans ] :=
    [ Use the paragraph's retained bidi resolution, without modifying it ]
    [ Obtain flattened style spans in original source-byte coordinates ]
    [ For each logical bidi segment B, including nested isolates ] :
        [ Visit style spans S overlapping B ]
        [ Compute from = max(B.From, S.From), to = min(B.To, S.To) ]
        [ For each nonempty intersection [from, to) ] :
            [ Record source byte/scalar ranges, originating style span and B.Dir ]
    [ Merge an intersection holding only soft hyphens into its neighbor ]
        // only within the same style span and direction, with contiguous source
    [ Retain the bidi resolution for splitting at the chosen line boundaries ]
```

Notes:

- Traversal uses the logical bidi segments. Splitting the bidi resolution is
  reserved for the line boundaries chosen by the line breaker.
- Feature ranges refer to scalar positions in the original style span. They are
  clipped and rebased for each shaping run, and once more when soft hyphens are
  removed from the shaper input. Empty overlaps are omitted.
- Soft hyphens are removed before shaping, not after, so that ligatures and
  joining across the marker are not disturbed. Glyph anchors returned by the
  shaper are mapped back to original scalar positions before cluster assembly.
- Setting the initial cluster for each run (rather than only for the first)
  keeps cluster IDs in original coordinates across omitted controls.

### Glyph Output End

The consumer runs on the calling execution path. It observes glyph output and
producer completion independently: channel closure does not imply success, and
producer success does not mean that buffered glyphs have been consumed.

```
[ Initialize cluster assembler and knot accumulator ]
[ Observe glyph output and producer completion ] :
    [ Map glyph anchors back to original scalar positions ]
    [ Validate glyph anchors and assemble complete clusters ]
    [ Associate each cluster with its run and original source span ]
    [ Reject mandatory breaks inside inseparable clusters ]
    [ Separate knots at line opportunities, word hints, run/class changes or gaps ]
    [ Preserve the source penalty and exact break position ]
    [ Accumulate fractional widths and retain owned glyph records ]
[ On any failure, cancel and join the producer; discard everything ]
[ After output is drained and producer and shaper succeeded ] :
    [ Complete the last cluster and the last knot ]
[ Prohibit the break after the final content knot ]
[ Append one non-discardable ParFillSkip knot carrying a mandatory break ]
[ Publish knot planes and the textbox shaping arena into a new khipu `K` ]
```

**Cluster assembly.** Consecutive glyphs sharing an anchor form a cluster.
Anchors must not decrease. A cluster ends at the next anchor in its run, or at
the end of its run; it never extends into the next run or across omitted
controls between runs. Glyph order and signed font-unit positions are preserved.
Each cluster records its run and the source penalty at its end.

Missing anchors cannot tell a ligature from characters the shaper omitted.
Interior and trailing omissions are therefore included in the preceding
cluster's span. Leading omissions and completely silent runs remain glyphless
*source gaps*; no glyph clusters are invented for them.

**Knot formation.** Completed clusters are grouped into knots. A new knot starts
at:

- a permitted line break (finite penalty) or a mandatory break,
- a word-boundary hint,
- a change of run,
- a change of whitespace class (see below),
- a glyphless source gap.

Cluster interiors are never split. For a glyphless source gap, the strongest
penalty within the gap is attached to the preceding knot (the earliest position
on ties), and its exact source position is recorded separately from the knot's
glyph-bearing span. No placeholder knot is created, so optional alternatives
with identical geometry collapse into one.

**Knot classification.** Each knot's original source range (ignoring soft
hyphens) determines its kind and elasticity:

```
[ Create a knot `k` for the collected clusters ] :=
    [ If the source range consists only of whitespace ] :
        [ Create a discardable glue knot `k` with its shaped natural width ]
        [ If it consists only of U+0020 and P(R).spacing is elastic ] :
            [ Derive stretch and shrink as percentages of the natural width ]
        [ Else ] :  // NBSP, other or mixed whitespace
            [ Keep the width fixed ]
    [ Else ] :  // letters, punctuation, or clusters mixing text and space
        [ Create a non-discardable textbox `k` retaining its glyphs ]
```

Widths are accumulated from glyph advances in fractional design units and
rounded once per knot. Elastic bounds are computed from the unrounded natural
width. Glyph offsets position ink and do not contribute to width. See
[WHITESPACE.md](WHITESPACE.md) for the space-elasticity policy. Kerning is part
of the shaped advances, so the encoder produces no separate kern knots.

Source spaces are retained as knots. Line breakers trim discardable knots at
line edges.

**Mandatory breaks.** A mandatory break at a cluster end is kept, including
zero-width newline glyphs. Optional breaks inside a cluster are suppressed. A
mandatory break that cannot be represented, because it lies inside a cluster or
because a glyphless gap would require a blank line, makes encoding fail with an
explicit error. Mandatory breaks are never weakened to optional or prohibited
ones. The end of the paragraph is represented once, by the final fill knot.
See [Open Design Questions](#blank-lines-and-glyphless-mandatory-breaks).

### Khipu Storage

The khipu stores knots in aligned struct-of-arrays planes: natural, minimum and
maximum width, penalty, source position and length, kind and flags. Line
breakers only read these planes. Everything needed by later phases is kept
out-of-band, in an arena owned by the khipu:

- flat glyph records, cluster-to-source mappings and box ranges for every
  textbox, in font units,
- shared run contexts (`P(R)` with its source range), so that a box can be
  reshaped later (e.g. for hyphenation); feature ranges remain relative to the
  original run and must be clipped and rebased for reshaping,
- source provenance and exact break position for every knot, including glue,
- the source-boundary table.

Arena entries are referenced by owner-local indices, not by pointers into
growable slices. Glue and the paragraph fill have no glyphs. Clusters mixing
text and space remain textboxes and keep all their glyphs. The original styled
paragraph and immutable font services are shared, not copied, and must stay
unchanged while the khipu is in use.

Glyphs are knotted in logical order.


## Phase 2: Line Breaking

**Prerequisites**:
- Khipu `K` holds the information used for line breaking in-band as knots.
  Information for later stages is held out-of-band.
- Knots are in logical order.
- UAX rules have been translated into knot penalties.
- Paragraph shape (target width per line number) and left/right skips are
  supplied as shared paragraph parameters. Line breaking and freezing must use
  the same parameters.

```
[ Find line breaks for `K` ] :=
    [ Clear previously selected discretionary decisions ]
    [ Obtain target line widths from the paragraph shape ]
    [ Run the chosen line-breaking algorithm over `K` in logical order ] :
        [ Consider permitted knot boundaries as candidate breakpoints ]
        [ Honor forced and prohibited breaks ]
        [ For each candidate line ] :
            [ Include any PostBreak fragment carried from its starting breakpoint ]
            [ Exclude discardable material at the line edges ]
            [ Include left/right skip dimensions ]
            [ Calculate natural width and available stretch/shrink ]
            [ Evaluate the candidate according to the chosen algorithm ]
            [ If the algorithm permits and requests discretionary lookup ] :
                [ Identify the textbox to consider ]
                [ Request candidates from the Khipukamayuq ]
                [ Evaluate each candidate using PreBreak instead of the full textbox ]
                [ Associate PostBreak with the continuation after that break ]
        [ Choose the breakpoint sequence according to the algorithm ]
        [ If no feasible sequence exists, fall back to over-/underfull lines ]
    [ Store only the applied discretionary choices in `K` ]
    [ Return the ordered breakpoint sequence ]
```

Both first-fit and Knuth-Plass follow this outline.

### Width Model and Badness

A knot's elasticity is the triple (`MinW`, `W`, `MaxW`). Its stretch is
`MaxW - W`, its shrink `W - MinW`. Infinite stretch of order fil, fill and filll
is encoded by sentinel values of `MaxW`. Shrink is always finite.

For a candidate line with natural width `w`, target width `t`, total finite
stretch `Y` and total shrink `Z`, the adjustment ratio is, as in TeX:

- `r = (t - w) / Y` if the line is short (`w < t`),
- `r = (t - w) / Z` if the line is long (`w > t`),
- `r = 0` if `w = t`.

If an infinite stretch order is present, a short line has `r = 0` for the
finite material. Badness is `100·|r|³`, capped at 10000 (`inf_bad`). A short
line without any stretch has badness `inf_bad`.

Following TeX:

- A line with `r < -1` (it cannot shrink enough) is **infeasible**. When a
  candidate line from an active breakpoint becomes infeasible, that breakpoint
  is **deactivated**; it cannot reach any later breakpoint either.
- A line with `r > 1` is feasible, merely bad. Whether it is acceptable depends
  on the tolerance of the current pass.

### Knuth-Plass

```
[ Find line breaks using Knuth-Plass ] :=
    [ Run a first pass without discretionaries, using PreTolerance ]
    [ If no acceptable complete path was found ] :
        [ Run a fresh second pass with discretionaries enabled, using Tolerance ]
            // final pass if EmergencyStretch = 0
    [ If still no acceptable path was found and EmergencyStretch > 0 ] :
        [ Run a fresh emergency pass like pass 2, adding EmergencyStretch ]
        [ to every line's stretch for badness calculation ]
            // always the final pass
    [ Choose the complete path with the lowest total demerits ]
    [ Collect its breakpoints backwards and reverse the sequence ]
```

Only the **final pass** may fall back to over-/underfull lines (see the pass
structure below). Earlier passes fail without a fallback, which triggers the
next pass. This guarantees a result whenever the input is valid.

**Emergency pass.** Its purpose is to prefer loose lines over overfull ones:
text protruding into the margin is considered worse than wide spaces. The pass
relaxes exactly one constraint, the apparent stretchability of a line during
badness calculation. For a short line with shortfall `s` and real finite
stretch `Y`, badness becomes `100·(s/(Y+E))³` instead of `100·(s/Y)³`, where
`E` is `EmergencyStretch`. Consequently, loose lines whose badness exceeded the
tolerance may become acceptable, lines without any stretch get a finite
badness, and since `E` is the same for every line, looseness tends to be spread
evenly across the paragraph. Nothing else changes: shrink is not relaxed
(`r < -1` remains infeasible, so overfull lines cannot be rescued), and
tolerance, penalties, forced/prohibited breaks, demerits and discretionary
lookup are the same as in pass 2.

`E` only influences the choice of breakpoints. It is not added to any knot, and
freezing sets each line with its real glue only. Lines chosen thanks to `E`
therefore usually have `r > 1` and are set and reported as underfull: the
emergency pass decides *which* underfull lines are produced, instead of
overfull ones.

Each pass has the following structure:

```
[ Run a Knuth-Plass pass ] :=
    [ Initialize the root state before the first knot ]
    [ Initialize the set of active breakpoints with the root ]
    [ Scan knots in logical order ] :
        [ Extend the line-segment bookkeeping of active states ]
        [ At each permitted or forced breakpoint `E` ] :
            [ For each active predecessor state `S` ] :
                [ Evaluate the candidate line from `S` to `E` ]
                [ If r < -1 ] :
                    [ Deactivate `S` ]
                    [ In the final pass, remember it as a fallback if it is the last one ]
                [ During pass 2, if the line's badness warrants discretionary lookup ] :
                    [ Request and evaluate discretionary alternatives ]
                [ For each acceptable ordinary or discretionary candidate ] :
                    [ Calculate line demerits and accumulated path cost ]
                    [ Keep the cheapest predecessor for this breakpoint and line count ]
            [ If `E` is forced, it must be reached ]
                // in the final pass, using the fallback if necessary;
                // otherwise the pass fails
            [ Activate newly discovered feasible states ]
            [ If no active state remains ] :
                [ In the final pass, activate `E` from the remembered fallback, as an overfull line ]
                [ Otherwise the pass fails ]
    [ Compare complete states reaching the paragraph end ]
    [ Report the best path and its quality, or failure ]
```

Demerits follow TeX: `(LinePenalty + b)² ± p²` for line badness `b` and break
penalty `p`, plus extra demerits for hyphenated lines. See
[DEMERITS.md](DEMERITS.md) and [KP.md](KP.md).

Discretionaries are discovered lazily. Shaping hyphenation alternatives is
costly, so pass 2 only requests them where a line's badness suggests that
hyphenation could help: for a loose line the next textbox that did not fit, for
a tight line the last textbox on the line. This is a deliberate deviation from
TeX, which hyphenates every word in pass 2; it may miss solutions that
hyphenate a different word. Explicit soft hyphens are requested whenever the
scan reaches a textbox that contains one. See [HYPHENATION.md](HYPHENATION.md).

### First-Fit

First-fit scans knots once and greedily commits the last feasible breakpoint
when the current line no longer fits. If a single textbox overflows, it tries
to split it with a discretionary alternative. If nothing fits, the line is set
overfull. See [FIRSTFIT.md](FIRSTFIT.md).

### Over- and Underfull Lines

Line breakers do not fail because a paragraph cannot be set well. As in TeX,
they fall back to lines that are overfull (too long even at maximum shrink) or
underfull (badness above the tolerance), and freezing sets those lines as
described [below](#over--and-underfull-lines-1). Line breakers only fail on
invalid input, for example if a forced break cannot be reached at all.


## Phase 3: Line-Local Bidi Ordering

Line breaking operates entirely in logical order. Only after the final path has
been chosen is the retained bidi resolution split at the same source
boundaries. We split the resolved levels, not an already computed ordering, and
then compute an ordering separately for each line.

```
[ Map the chosen breakpoints to source byte positions `lineEnds`; stop on error ]
    // ordinary breaks via the source-boundary table,
    // selected discretionaries via their original source split position
[ Materialize logical lines, recording each line's trailing source-glue interval before trimming ]
[ Let `remaining` be a deep copy of the paragraph's resolved bidi levels ]
[ For every `lineEndByte` in `lineEnds`, in logical paragraph order ] :
    [ If this is not the final line ] :
        [ `lineLevels, remaining = remaining.Split(lineEndByte)` ]
    [ Else ] :
        [ `lineLevels = remaining` ]
    [ If this line has a trailing source-glue interval `[from, to)` ] :
        [ Reset its levels to the paragraph level (rule L1); stop on error ]
    [ Store the reordering of `lineLevels` as this line's `Ordering` ]
```

Then the knots of every line are arranged in physical left-to-right order,
independently of the paragraph's base direction:

```
[ Arrange the line's knots using its `Ordering` ] :=
    [ For every `run` in `Ordering.Runs` ] :
        [ For every source `segment` of `run`, in physical left-to-right order ] :
            [ Locate the knots associated with `[segment.From, segment.To)` ]
            [ Arrange them in the segment's directional traversal order ]
            [ Preserve the glyph sequence within each box and its direction ]
    [ Place the final ParFillSkip at the paragraph's trailing physical edge ]
        // right for LTR paragraphs, left for RTL paragraphs
    [ Reject unsupported source-less material rather than guessing its position ]
```

Design rules:

- All bidi positions are half-open UTF-8 byte offsets into the original
  paragraph, never knot indices or cluster IDs. Splitting preserves these
  coordinates across lines. A run can consist of noncontiguous source
  intervals.
- A selected discretionary needs its original source split position and the
  provenance of its fragments. The length of the shaped PreBreak text is no
  substitute: substitutions or inserted glyphs may change it. The final
  ParFillSkip maps to the paragraph end, not to an additional character.
- Source coverage and the L1 glue interval are recorded before trimming:
  discarded spaces still take part in bidi line-boundary calculation even
  though their knots are not emitted.
- Boxes must not cross directional segment boundaries. A segment may cover
  several boxes, which are ordered directionally, but glyph arrays inside boxes
  are not reversed here. Later placement must honor their direction, because
  the shaper emits glyphs in logical order.
- Selected fragments with original provenance, even those without glyphs, are
  placed as indivisible boxes.
- The paragraph direction used for placing the final fill comes from the
  paragraph's bidi resolution, not from a run or a configured default.
- The paragraph's bidi resolution and the logical lines are not modified, so
  ordering can be retried.


## Phase 4: Freezing the Khipu

**Prerequisites**:
- A sequence of breakpoints into `K`, as returned by a line breaker.
- The selected discretionaries of that same line-breaking run.
- A line-local bidi `Ordering` for each line.
- The same paragraph shape and left/right skips used during line breaking.

```
[ Make the first knot of `K` the start `S` of the current line ]
[ For every breakpoint `E` ] :
    // The sub-khipu from `S` to `E` is the current line
    [ Remove discardable knots at both line edges ]
    [ Replace a selected discretionary textbox, if present ]
    [ Arrange the line's knots using its `Ordering` ]
    [ Place the left/right skips in explicit physical edge slots ]
    [ Snapshot the target width `LineLength(lineNumber)`, line numbers starting at 1 ]
    [ Calculate delta = target width - natural width, both including the skips ]
    [ Distribute delta among the elastic knots of the line ]
    [ Record the line's adjustment ratio, badness and over-/underfull status ]
    [ Package a frozen line ]
    [ Make `E+1` the new `S` ]
```

with

```
[ Replace a selected discretionary textbox, if present ] :=
    [ Substitute PreBreak for the last box on the broken line ]
    [ Prepend PostBreak to the following line ]
```

and

```
[ Distribute delta among the elastic knots of the line ] :=
    [ If delta > 0 ] :  // stretch
        [ If knots with infinite stretch are present ] :
            [ Distribute delta evenly among the knots of the highest order present ]
                // filll before fill before fil
            [ All other knots keep their natural width ]
        [ Else if the total finite stretch Y > 0 ] :
            [ Distribute delta proportionally to each knot's stretch MaxW - W ]
                // r = delta / Y may exceed 1: knots then grow beyond MaxW
            [ If r > 1, mark the line underfull ]
        [ Else ] :
            [ Keep all knots at natural width; mark the line underfull ]
            [ The unfilled remainder stays at the line's trailing edge ]
    [ If delta < 0 ] :  // shrink
        [ If the total shrink Z >= |delta| ] :
            [ Distribute delta proportionally to each knot's shrink W - MinW ]
        [ Else ] :
            [ Set every knot to its MinW ]
            [ Mark the line overfull by |delta| - Z ]
            [ The excess protrudes at the line's trailing edge ]
```

Shares are calculated with exact integer arithmetic: proportional shares are
rounded down and the remaining design units are assigned by largest remainder,
ties in physical left-to-right order. The widths of a line always sum exactly
to the target, except for overfull lines and underfull lines without any
stretch.

The trailing edge is the right edge for LTR paragraphs and the left edge for
RTL paragraphs, matching the placement of the final ParFillSkip.

Elastic knots are normally glue. Elastic textboxes (e.g. kashida justification)
fit the same allocation scheme, but an allocated textbox width is only a
proposal until a shaping service has produced matching glyphs for it.

### Over- and Underfull Lines

As in TeX, freezing does not fail because a line cannot meet its target width.
Such a line is set as well as possible and reported:

- **Underfull:** stretchable knots are stretched beyond their maximum in
  proportion to their stretch (TeX's glue set ratio greater than 1). Lines
  without any stretch keep their natural width and leave the remainder at the
  trailing edge.
- **Overfull:** every knot is shrunk to its minimum and the line protrudes
  beyond the target width at its trailing edge.

Each frozen line records its adjustment ratio, its badness and, for overfull
lines, the excess width, so that a report can be produced. Whether a line is
reported should depend on thresholds similar to TeX's `\hbadness` and
`\hfuzz`. How reports are delivered is an
[open question](#reporting-over--and-underfull-lines).

The frozen result is final layout data: it has fixed widths per knot, retains
glyphs and provenance, and has no elastic widths or penalties. There is no
unfreeze operation; the elastic khipu remains available for freezing another
break decision.


## Phase 5: Visual Representation

Box creation, directional glyph placement and rendering are outside the
responsibility of this module. A frozen line already has its knots arranged for
physical traversal. This does not reverse glyph sequences within boxes or remove
the need for direction-aware placement later; paragraph bidi ordering must not
be applied a second time.


## Open Design Questions

### Reporting Over- and Underfull Lines

Lines that cannot meet their target are set and reported, but the reporting
channel is undecided. Options:

- per-line status in the frozen result (adjustment ratio, badness, excess),
  which clients query,
- path quality returned by the line breaker (worst badness, number of
  fallback lines),
- a diagnostics callback or logger configured on the Khipukamayuq.

The thresholds that decide what is worth reporting (analogous to `\hbadness`,
`\hfuzz`) also belong to this decision.

### Inline Objects

Images and inline-blocks are meant to enter as fixed-size boxes. Open points:
how they are represented in the styled paragraph (e.g. U+FFFC with an attached
object), which knot kind they map to, how they take part in bidi ordering and
line-breaking penalties, and how their dimensions bypass the shaper.

### Blank Lines and Glyphless Mandatory Breaks

Paragraphs whose mandatory breaks fall into glyphless source gaps (for example
consecutive line feeds when the shaper emits no glyph for them) need a way to
represent empty lines.

### Tabs

A tab is currently treated as ordinary whitespace glue with its shaped advance.
There is no concept of tab stops.

### Storage of Break Decisions

Line breakers store their selected discretionaries in the khipu. Consumers
cannot detect whether these selections belong to the breakpoints they are
holding. Returning selections together with the breakpoints would make break
results self-contained.

### Infinite Shrink and Fill Magnitudes

The width model has no infinite shrink, and all knots of the same fill order
receive equal shares (there is no "2fil"). Both are simplifications compared
to TeX.


## Implementation Status

### Overview

| Phase | Status |
|---|---|
| Knotting | Implemented: `Khipukamayuq.EncodeParagraph`. |
| Line breaking | Implemented: `linebreak/firstfit` and `linebreak/knuthplass`, with lazy explicit and automatic discretionaries. Known deviations listed below. |
| Bidi ordering | Implemented internally; no public API. |
| Freezing | Internal glue-only implementation; no public API. Over-/underfull lines are rejected. |
| Visual representation | Out of scope. |

`Config{}` provides a synthetic streaming backend for unstyled text; this is
deterministic test/simple-layout shaping, not real font shaping. Applications
supply a complete `Config.Streaming` with shaper factory, style resolver and
defaults. There is no metric-only encoding path. See
[MIGRATION.md](MIGRATION.md).

### Code Map

| Design step | Code |
|---|---|
| Source-boundary table | `prepareParagraphBoundaries` (`boundary_preparation.go`) |
| Bidi/style intersection | `intersectParagraphRuns` (`run_intersection.go`) |
| Run parameters `P(R)` | `prepareParagraphRunsForInput`, `preparedRun` (`run_preparation.go`, `prepared_run.go`) |
| Soft-hyphen handling | `coalesceMarkerIntersections`, `prepareNoBreakRuns`, `restoreNoBreakAnchor` (`stream_nobreak.go`) |
| Producer | `produceNoBreakRuns` (`run_producer.go`) |
| Cluster assembly | `glyphClusterAssembler` (`glyph_clusters.go`) |
| Knot formation and classification | `candidateKnotAccumulator`, `classifyCandidateSource`, `candidateWidth` (`candidate_knots.go`, `candidate_metrics.go`) |
| Connected pipeline | `streamCandidateParagraph` (`stream_candidates.go`) |
| Khipu storage | `publishCandidateParagraph`, `shapingArena` (`shaping_storage.go`) |
| Lazy discretionaries | `Khipukamayuq.DiscretionaryCandidates` (`discretionary*.go`) |
| Breakpoint positions | `Khipu.BreakpointPositions` (`breakpoint_positions.go`) |
| Logical lines | `materializeLogicalLines` (`logical_lines.go`) |
| Bidi ordering | `lineOrderings`, `trailingGlueL1` (`line_ordering.go`), `mapOrdering` (`knot_ordering.go`), `placeGenerated` (`generated_placement.go`), `materializeVisualLines` (`visual_lines.go`) |
| Line geometry | `materializeLineGeometry`, `prepareGeometry` (`line_geometry.go`) |
| Width distribution | `planWidths`, `allocateWidths` (`width_allocation.go`) |
| Freezing | `freezeParagraph`, `frozenParagraph` (`frozen.go`) |

### Knotting

- No dictionary-based or language-specific word segmentation (e.g. Thai
  tailoring); word hints use default UAX#29 rules.
- Source NUL is classified as U+FFFD for UAX#14, without changing its source
  coordinates.
- An omitted U+200B whose boundary is already a cluster end uses the normal
  source-penalty path.
- Leading, repeated or trailing glyphless mandatory breaks and mandatory breaks
  inside clusters return `ErrUnrepresentableMandatoryBreak`.
- A knot's source length is stored in 16 bits, so a run of more than 64 KiB
  without a break opportunity or word boundary cannot be encoded.
- Preparation checks cancellation between stages, not within them. A shaper
  blocked in a source read is subject to the streaming contract's cancellation
  limits.
- Arena compaction and renderer access to glyph data are not implemented.
- Inline objects and tab stops are not supported.

### Line Breaking

Known deviations from this design in `linebreak/knuthplass`:

- **Infeasible lines are not deactivated.** `calcAdjustmentRatio` accepts
  ratios below −1; the badness is capped at 10000 and the active breakpoint is
  kept. With a tolerance of 10000 or more, overfull lines are accepted as merely
  bad.
- **Lines without stretch are infeasible** rather than having badness
  `inf_bad`.
- **The second pass is only triggered** when the first pass's best path is an
  overfull fallback.
- **No emergency pass.** `EmergencyStretch` is retained for the final pass
  designed above, but is still unused. Its
  defaults also differ: 20 bp in `NewKPDefaultParameters`, 50 bp in
  `DefaultParameters` (plain TeX uses 0, i.e. off).
- **Fallbacks are not restricted to the final pass.** Every pass may produce
  overfull fallback lines.
- The unused `DoubleHyphenDemerits` and `HyphenPenalty` fields and their defaults
  have been removed from `Parameters`. Consecutive-discretionary demerits remain
  deferred. The effective hyphen penalty comes from a nonzero penalty on the
  hyphenator's fragment or from `khipu.Params.Hyphenpenalty` (default 10), with no
  competing breaker default.
- **Performance.** Working states are indexed by breakpoint and line number;
  width updates and ordinary/discretionary scoring visit only the requested
  breakpoint's bucket. Ordinary/discretionary evaluation results and sorted
  horizon snapshots reuse separate per-breaker scratch buffers. Retired working
  states are still retained, and active-set growth remains an optimization target.
  Expensive debug arguments and final info dumps are now guarded
  before formatting; full debug tracing remains intentionally expensive.
  Separate encoding/breaking/first-pass benchmarks and initial measurements
  are documented in [TESTING.md](TESTING.md#knuth-plass-performance-baseline).
- Complete paths tied on total demerits prefer fewer lines. Equal-cost
  predecessors for the same state prefer the lower knot index, then variant;
  these tie decisions no longer depend on map iteration or arrival order.

Both breakers fall back to overfull lines when nothing fits, but the frozen
result cannot yet represent them (see below).

### Bidi Ordering

- `BreakpointPositions` maps ordinary breaks through the source-boundary table
  and selected discretionaries through `SourceSplit`. Metric-only candidates
  without a known source split are rejected.
- Rule L1 is approximated by resetting the trailing source-glue interval of
  each line. This is not full UAX#9 L1: NBSP glue or suppressed soft hyphens
  can be over-reset; omitted controls, whitespace embedded in text clusters and
  segment separators (tabs) can be missed. See
  [FREEZE.md](FREEZE.md#accepted-l1-approximation).
- Genuinely zero-source fragments fail until an attachment contract exists.
- Successive splits inside isolates are tested together with the sibling
  `uax/bidi` package. This does not establish full L1 conformance.

### Freezing

- Freezing is internal (`freezeParagraph`); the public `Freeze` API and
  `FrozenKhipu` are proposed in [FROZEN-API.md](FROZEN-API.md).
- Only glue may change width. Non-glue knots must have `MinW == W == MaxW`;
  elastic textboxes are rejected until a target-width shaping service exists.
- **Over- and underfull lines are rejected.** `allocateWidths` returns
  `errInsufficientCapacity` when a line needs more shrink or more finite
  stretch than available, instead of setting the line as described above. No
  adjustment ratio, badness or over-/underfull status is recorded per line.
- Fill precedence, finite stretch/shrink and largest-remainder rounding are
  implemented as designed. See [FREEZE.md](FREEZE.md#frozen-publication).

### Parameter Cleanup

The unused `Baselineskip`, `Lineskip`, `Lineskiplimit`, `Hyphenchar` (formerly
`Hypenchar`) and `Minhyphenlength` fields and defaults have been removed from
`khipu.Params`. Vertical line spacing belongs to future line-placement work;
replacement material belongs to the script-sensitive discretionary provider.
A future minimum-length policy needs to define both the constrained fragment
and the unit of measurement before exposing a parameter.

`EmergencyStretch` is deliberately retained as the exception: the planned
emergency pass above defines its purpose. This cleanup does not implement that
pass, change its existing defaults, or add over-/underfull freezing support.
