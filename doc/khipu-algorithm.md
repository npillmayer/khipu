# Overall Algorithm to Shape Text into Type

The streaming/glyph sections below describe the intended architecture. The
implemented SOA encoder uses the synchronous `Shaper` metric interface; see
[MIGRATION.md](MIGRATION.md) for its byte-offset and cluster contract. It preserves
original source bytes. Any normalization must precede construction of styled
source ranges. UAX#14 is evaluated across the whole paragraph before intersecting
its boundaries with shaped runs.

We are dealing with sequences of runes as input, forming a styled paragraph of text. Styles are nested, i.e. form a hierarchy. Examples of styles may be "*boldface*", a font- or color-change, etc. Styles are projected onto the runes out-of-band. In previous stages an HTML-parser will have converted HTML and CSS into a styled paragraph, where the style hierarchy is held in a tree-like structure. For every rune in the input sequence, the style hierarchy is able to tell the exact style valid at this rune.

Non-discardable HTML-items (e.g. inline-blocks, images, etc.) are already taken care of at this stage, resulting in fixed-size anonymous boxes, unaffected by the current style.

The first phase of our algorithm will convert the rune sequence into a sequence of glyphs. Glyphs are selected from a font. Each glyph will carry positioning information, sizing information, a cluster-ID, and other meta-information. In general, we will be translating from the realm of characters (input paragraph) to the realm of glyphs (khipu).

The sequence of runes will be broken into runs of runes (i.e., of text). Runs are the input for text shaping (with some tool like HarfBuzz). All runes of a single run share the same style information.


### Param-Environments

Each run `R` is associated with a "Param-Environment" `P`, which determines properties for each rune `c` in the run. That means: `P(c) = P(R)` for every `c` in `R`. Properties of a `P` are:

- selected `font`
- current script `script`
- current language `lang`
- current `bidi` direction (LTR or RTL)
- treatment of inter-word spaces: `spacing` = either `elastic` or `fixed`

The top-level paragraph has `P(¶)`, which will provide default values for all properties.

Example: For a character / rune `c`, `P(c).font` would denote the font associated with `c` through the Param-Environment `P`, which is active for the run containing `c`.

Param-Environments result from the paragraph's style hierarchy. From a logical point of view, the hierarchy gets flattened into a sequence of property-sets. This does not restrict space optimization for the actual memory layout, but from a conceptual angle every run is associated with a full set of properties (relevant for paragraph layout).

Styles will always be translated to concrete properties, e.g. "*bold*" will be translated to a bold font face. `P(R)` will be uniform for all properties which influence shaping.


### Bidi Intervals

Paragraphs may contain character sequences in mixed LTR and RTL scripts. These sub-sequences will be described (in terms of bidi) as bidi-intervals, denoted as `B`. Bidi intervals are independent (orthogonal) to rune runs. Each `B` has a property `bidi`, either of value `LTR` or `RTL`. The `uax/bidi` package resolves the paragraph into `*ResolvedLevels`, which we retain for line-local ordering after line-breaking. We do not store a numeric embedding level in each run's Param-Environment; direction is still required for shaping and later glyph placement.

If a paragraph is split at a bidi interval boundary, the style hierarchy is split appropriately.

Example: A paragraph of English text may have two Hebrew words contained in the middle. The second Hebrew word and the first English word after it are spanned in a "boldface" style. With a bidi interval split after the 2nd Hebrew word, this word will still be in bold style, and the following English word will still be in bold, too. The style span gets split in two.


### Clusters

Every output glyph `g` is considered "stemming from" one or more characters `c` of `R`. This relation is denoted by cluster-IDs.

We need to keep bidi-interval `B` boundaries and `P(R)` information associated with cluster-IDs for later stages.


### Unicode Annexes

Segmentation of rune sequences is done with Unicode segmentation rules in mind, i.e. UAX annex rules. Examples of this would be UAX#9 (bidi), UAX#14 (line segmenting) or UAX#29 (word breaking). We have a Go package available that translates UAX rules into penalty values, i.e. `UAX#29(c)` will give a numerical value (or `∞`, i.e. infinity, for disallowing a breakpoint). Mandatory breakpoints give `-∞`.

Interpretation of UAX lives in the character realm. As a general rule, breaking-information derived from UAX rules will be translated to penalties in the glyph-realm. We will use a function `U` to calculate the accumulated penalties for (the position after) a given input character `c` from the UAX rules: `U(c) = UAX#29(c) + UAX#14(c) + …`. In case of a disallowed break, `U(c) = ∞` (infinity).


## Pseudo-Code

Notation:
- "[ … ]" denotes pseudo-code lines or blocks
- ":" introduces a sub-block
- "//" denotes code comments
- ":=" details a block into sub-blocks

For sketching out the algorithm we will rely on a Shaper version which offers a streaming interface with a buffered channel. It is easier to reason about the two realms and their responsibilities with a decoupling queue between them. The streaming Shaper reads runes on the input end and pushes glyph and positioning information into a channel on the output end. The shaper always produces glyphs in logical order.


## Khipu-Knotting

### Character Input End

```
[ Intersect bidi intervals with style spans ]
[ For every interval `B` in `ResolvedLevels.LogicalSegments()` ] :
    // Runs are split off at style boundaries
    [ For every run `R` split off from `B` ] :
        [ `P(R).bidi = B.bidi` ]
        [ Configure the Shaper with `P(R)` ] // Set font, script, etc.
        [ Point the Shaper to `R` ]
        // The Shaper reads from a RuneSource,
        // i.e. it is "pulling" characters
        [ For every rune pulled into the Shaper ] :
            [ Generate a cluster-ID for `c` ]
            [ Calculate `U(c)` ]
        [ Associate the Cluster-ID of the first rune of `R` with `P(R)` ]
[ Signal EOF to the Shaper channel ]
```

Here, splitting off shaping intervals means traversing the paragraph in logical
order, not consuming `ResolvedLevels.Split`. The latter is reserved for the
chosen line boundaries. `LogicalSegments()` includes nested isolates without
modifying the resolution. Intersect its source-byte intervals with
`Paragraph.StyleRanges()`, preserving isolate boundaries and gaps for omitted
controls. The internal `intersectParagraphRuns` stage performs this intersection
without flattening the cord: a forward reader scan records original scalar
positions, including skipped controls. Results retain source ranges, style and
direction and the original style-span range. `prepareParagraphRuns` then resolves
each contributing style span once through `contracts.StyleResolver`, acquires
its font scale, and creates prepared runs with owned feature storage. Feature
ranges refer to original style-span scalar positions, including omitted controls;
they are clipped and rebased for each shaping run. Empty overlaps are omitted.
No numeric bidi level is stored. Neither preparation nor streaming-shaper
submission is wired into the active encoder yet.

```
[ Intersect bidi intervals with style spans ] :=
    [ Resolve bidi for the whole paragraph ]
    [ Obtain flattened style spans in original source-byte coordinates ]
    [ For each segment B in levels.LogicalSegments() ] :
        [ Visit style spans S overlapping B ]
        [ Compute from = max(B.From, S.From) ]
        [ Compute to = min(B.To, S.To) ]
        [ For each nonempty intersection [from, to) ] :
            [ Resolve style-controlled shaping parameters from S ]
            [ Set shaping direction to B.Dir ]
            [ Subdivide further where script/language requires it ]
            [ Record original byte and scalar ranges in preparedRun ]
            [ Submit original logical text with these parameters to the Shaper ]
    [ Retain levels for splitting at the finally selected line boundaries ]
```

### Glyph Output End

```
[ Initialize an empty khipu `K` ]
[ Repeat until Shaper output channel is drained ] :
    [ Collect glyph info `g` as long as `U(c) = ∞`, where `g` stems from `c` ]
    // The last `c` has `U(c) < ∞` or the channel is drained
    [ Create a knot `k` for the collected glyph infos ]
    [ Set a penalty for `k` to `U(c)` ]
    [ Append knot `k` to `K` ]
[ Retain source spaces; breakers trim flagged discardables at line edges ]
[ Prohibit the break on the final source knot ]
[ Append one non-discardable parfillskip knot carrying the mandatory penalty ]
```

where


```
[ Create a knot `k` for the collected glyph infos ] :=
    [ If the collected glyphs represent space only ] :
        // Stretchability of glue drop depends on `P(c).spacing` (`elastic` or `fixed`)
        [ Create a glue knot `k` ]
    [ Else ] :  // Deal with a sequence of letter glyphs
        [ Create a box knot `k` from the collected glyph infos ]
```

Glyphs are knotted now in logical order.


## Line Breaking

**Prerequisites**:
- Khipu `K` holds information used for line-breaking in-band as knots. Information used in later stages is held out-of-band (associations).
- Glyphs are knotted in logical order.
- UAX rules have been translated to penalties in the glyph realm.
- Linewidth and parshape are provided as input parameters to the line-breaker.

```
[ Find line breaks for `K` ] :=
    [ Clear previously selected discretionary decisions ]
    [ Obtain target line widths from parshape ]
    [ Run the chosen line-breaking algorithm over `K` in logical order ] :
        [ Consider permitted knot boundaries as candidate breakpoints ]
        [ Honor forced breaks and prohibited breaks ]
        [ For each candidate line ] :
            [ Include any PostBreak fragment carried from its starting breakpoint ]
            [ Exclude discardable material at the line edges ]
            [ Include left/right skip dimensions ]
            [ Calculate natural width and available stretch/shrink ]
            [ Evaluate the candidate according to the chosen algorithm ]
            [ If the algorithm permits and requests discretionary lookup ] :
                [ Identify the textbox to consider ]
                [ Request candidates from Khipukamayuq ]
                [ Evaluate each candidate using PreBreak instead of the full textbox ]
                [ Associate PostBreak with continuation after that break ]
        [ Choose the breakpoint sequence according to the algorithm ]
    [ Store only the applied discretionary choices in `K` ]
    [ Return the ordered breakpoint sequence ]
```

The shared outline allows either firstfit or Knuth-Plass. For Knuth-Plass:

```
[ Find line breaks using Knuth-Plass ] :=
    [ Run a first pass without discretionaries, using PreTolerance ]
    [ If no acceptable complete path was found ] :
        [ Run a fresh second pass with discretionaries enabled, using Tolerance ]
    [ Choose the complete path with the lowest total demerits ]
    [ Collect its breakpoints backwards and reverse the sequence ]
```

Each Knuth-Plass pass has the following structure:

```
[ Run a Knuth-Plass pass ] :=
    [ Initialize the root state before the first knot ]
    [ Initialize the horizon of active breakpoints with the root ]
    [ Scan knots in logical order ] :
        [ Extend the line-segment bookkeeping of active states ]
        [ At each permitted or forced breakpoint `E` ] :
            [ For each active predecessor state `S` ] :
                [ Evaluate the candidate line from `S` to `E` ]
                [ During pass 2, if local badness warrants discretionary lookup ] :
                    [ Request and evaluate discretionary alternatives ]
                [ For each acceptable ordinary or discretionary candidate ] :
                    [ Calculate line demerits and accumulated path cost ]
                    [ Keep the cheapest predecessor for this breakpoint and line count ]
            [ Retire states that cannot reach further breakpoints ]
            [ Activate newly discovered feasible states ]
    [ Compare complete states reaching the paragraph end ]
    [ Report the best path and its quality, or failure ]
```


## Line-Local Bidi Ordering

Line-breaking operates entirely in logical order. Only after the final path has
been chosen do we split the retained bidi resolution at the same source
boundaries. We split `ResolvedLevels`, not an already computed `Ordering`, and
then compute an `Ordering` separately for each line.

```
[ Map the chosen breakpoint sequence to paragraph-relative UTF-8 byte boundaries ]
[ Let `remaining` be the retained paragraph `ResolvedLevels` ]
[ For every line in logical paragraph order ] :
    [ If this is not the final line ] :
        [ `lineLevels, remaining = remaining.Split(lineEndByte, false)` ]
    [ Else ] :
        [ `lineLevels = remaining` ]
    [ Account for line-boundary whitespace direction rules (UAX#9 L1) ]
    [ Store `lineLevels.Reorder()` as this line's `Ordering` ]
```

The following operation is applied to each materialized line immediately before
freezing. We choose left-to-right physical traversal, independently of the
paragraph's base direction.

```
[ Arrange the line's knots using its `Ordering` ] :=
    [ For every `run` in `Ordering.Runs` ] :
        [ For every `segment` in `run.Segments(run.IsOpposite(bidi.LeftToRight))` ] :
            [ Locate the knot/box groups associated with `[segment.From, segment.To)` ]
            [ Arrange those groups in the segment's directional traversal order ]
            [ Preserve the glyph sequence within each box and its direction metadata ]
    [ Place generated material according to its associated fragment or line edge ]
```

**Implementation remarks**:
- All bidi positions are half-open UTF-8 byte offsets into the original paragraph, not knot indices or cluster IDs. `Split(..., false)` preserves those coordinates across lines. A run can contain noncontiguous source intervals; use `Segments`, not `run.Length`, to locate them.
- A selected discretionary needs its original source split position and fragment provenance. The length of the shaped PreBreak text is not a substitute: substitutions or inserted glyphs may change it. The terminal parfillskip maps to paragraph end, not an additional source character.
- Source coverage is partitioned before trimming. Discarded source spaces still participate in the bidi line boundary calculation, even though their knots will not be emitted. L1 handling remains a client responsibility and needs refinement independently of width trimming.
- Boxes must be compatible with directional segment boundaries. A segment may cover multiple boxes, which need directional ordering, but glyph arrays inside boxes are not reversed here. Later placement must still honor their direction because the streaming shaper emits logical shaping order.
- Generated fragments and zero-source glue need explicit attachment rules. Left/right skips are attached to physical line edges after ordering; source-less material cannot be located by byte intervals alone.
- `Split` and `Reorder` mutate backing state. Do not consume the retained resolution during candidate-path exploration; copying the structs does not create independent snapshots.
- The current `uax/bidi` implementation does not preserve the paragraph embedding-direction field when splitting. This and its unfinished nested-isolate splitting paths need attention before enabling general multiline bidi layout. The sketch describes the intended pipeline, not a claim that these upstream paths are ready.


## Freezing the Khipu


**Prerequisites**:
- A set of breakpoints is available, pointing to knots in `K`.
- Glyphs are knotted in logical order.
- A line-local bidi `Ordering` is available for each chosen line.
- Selected discretionaries (SelectedDiscretionaries)
- Paragraph shape (ParShape) or explicit target line lengths
- The same left/right skip dimensions used during linebreaking


```
[ Make the first knot of `K` the start `S` of the current line to freeze ]
[ For every breakpoint `E` ] :
    // Consider the sub-khipu from `S` to `E` as the current line
    [ Remove trimmed discardables at both line edges ]
    [ Replace selected discretionary textbox, if present ]
    [ Arrange the line's knots using its `Ordering` ]
    [ Pre-/append left/right skip dimension glue to the current line ]
    [ Calculate delta = target_line_width - natural_line_width ]
    [ Distribute delta between elastic knots present in the line ]
    [ Package a frozen khipu data-structure for the line ]
    [ Make `E+1` the new `S` ]
```

with 

```
[ Replace selected discretionary textbox, if present ] :=
    [ Substitute PreBreak for the last box on the broken line ]
    [ Prepend PostBreak on the following line ]
```

and

```
[ Distribute delta between elastic knots present in the line ] :=
    // Elastic knots are either glue drops or elastic boxes (e.g., kashida)
    [ Do this for shrink case and for stretch case ] :
        [ If "(-)filll" drops are present in the line ] :
            [ Distribute delta evenly and completely between the filll-drops ]
        [ Else if "(-)fill" drops are present in the line ] :
            [ Distribute delta evenly and completely between the fill-drops ]
        [ Else if "(-)fil" drops are present in the line ] :
            [ Distribute delta evenly and completely between the fil-drops ]
    [ If delta has been distributed ] :
        [ Set all other stretches/shrinks to 0 ]
    [ Else ] :
        // Only "normal" glue drops and/or elastic boxes present in the line
        [ For each elastic knot: calculate the elasticity ratio `s` ]
        [ Distribute delta between knots proportional to their `s` ]
```


## Create Visual Representation

Box creation, directional glyph placement, and rendering are outside the
responsibility of this phase. The frozen line already has its knot/box groups
arranged for physical traversal. This does not reverse glyph sequences within
boxes or remove the need for direction-aware placement later; paragraph bidi
ordering must not be applied a second time.
