# Khipu: From Text to Typesetting

Package khipu is about encoding text into typesetting items.

## Metaphor

> “Khipu were recording devices fashioned
> from strings historically used by a number of cultures in the region of
> Andean South America.
> Khipu is the word for »knot« in Cusco Quechua.
> A khipu usually consisted of cotton or camelid fiber strings. The Inca
> people used them for collecting data and keeping records, monitoring tax
> obligations, properly collecting census records, calendrical information,
> and for military organization. The cords stored numeric and other values
> encoded as knots, often in a base ten positional system. A khipu could
> have only a few or thousands of cords.”
> ––Excerpt from a Wikipedia article about khipus

The Khipukamayuq (Quechua for “knot-makers”) were the scribes of those
times, tasked with encoding tax figures and other administrative
information in knots.
We will use this analogy to call typesetting items “khipus” or “knots,”
and objects which produce khipus will be “Khipukamayuq”s (I admit this is a tough one).
Knots implement items for typesetting paragraphs.

## Create Khipus from Text

A Khipukamayuq is part of a typsetting pipeline and will transform text into khipus.
Khipus are the input for linebreakers. The overall process of creating
them and the interaction with line breaking is fairly complicated, especially if
one considers international scripts.

We will use a box-and-glue model and the various knot types more or less
implement the corresponding node types from the TeX typesetting system.
The diagram below depicts an approximation of the overall task flow for typesetting
a single paragraph.

<div style="width:480px;padding:5px;padding-bottom:10px">
<img alt="typeset a single paragraph" src="http://npillmayer.github.io/TySE/images/khipukamayuq.svg" width="480px">
</div>

## Configure SOA Encoding

Use `NewKhipukamayuq` with the public `Config` type. Given a styled paragraph
`para` and caller-supplied implementations of `Shaper` and `Hyphenator`:

```go
kq := khipu.NewKhipukamayuq(khipu.Config{
    Shaper:     shaper,
    Hyphenator: hyphenator,
})
khp, err := kq.EncodeParagraph(para, nil)
```

`khp` is the SOA `*khipu.Khipu`. Omitting `Params` selects the existing defaults;
a supplied `*Params` is copied in full, including zero fields. Omitting
`Hyphenator` disables lazy discovery while preserving access to cached candidates.
The collaborators themselves are retained, so a stateful shaper must not be used
concurrently with its owner.

`Config{}` selects a reusable fixed-width shaper and automatically appends the
non-discardable paragraph fill and forced break expected by both modern breakers.
Supply `Config.ParFillSkip` to override the fill dimensions. Encoding preserves
the original UTF-8 bytes and source positions; normalize text before constructing
styled ranges if needed. Custom shapers report run-relative byte offsets in
logical order, including explicit cluster starts with zero width.

The fixed-width shaper is a simple metric provider, not a font shaping backend.
Freezing, glyph provenance, and script-sensitive spacing remain future work.

SOA is the chosen paragraph model. Legacy APIs remain during migration; see
[the API migration guide](doc/MIGRATION.md) for the constructor change and planned
replacements.
