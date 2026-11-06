# Migration To Streaming SOA Paragraphs

The AOS-to-SOA migration and subsequent metric-shaping retirement are complete.
The only paragraph encoder is:

```go
func (kq *Khipukamayuq) EncodeParagraph(
    ctx context.Context, para *styled.Paragraph,
) (*Khipu, error)
```

The former `EncodeParagraphStreaming` method was renamed, not wrapped.
[RETIRE-METRICS.md](RETIRE-METRICS.md) records the three-step retirement,
coverage migration and required sibling dependency fixes.

## Public Construction And Configuration

`NewKhipukamayuq(Config{})` supplies synthetic streaming shaping: an unbuffered
fresh stream per encoding/reshaping call, a synthetic font at 10 pt, nil-style
resolution and elastic U+0020 spaces. Ordinary synthetic cells are 5 pt.
This is deterministic test/simple-layout behavior, not real typography.

Supply a complete `Config.Streaming` for an application backend. Its factory
must return a fresh stream for every request; missing services or invalid
defaults fail rather than silently falling back. The resolver maps application
styles to font, size, script, language, features and spacing. Defaults are applied
independently to original style spans, then clipped to bidi intersections.
Numeric width adaptation through KnotStyler has no replacement hook.

Config/default feature storage, Params and ParFillSkip are copied. Collaborator
instances and factories are retained. Serialize owner operations and keep source
text/styles and service behavior stable while paragraphs may be reshaped.
Nil Params selects defaults; a supplied Params value preserves explicit zeros.
Nil ParFillSkip selects zero natural/minimum width and infinite maximum width;
a supplied value preserves explicit zero widths.

## Removed APIs And Migration Targets

| Removed API or implementation | Replacement |
| --- | --- |
| List/AOS paragraphs, Cursor/Mark traversal and concrete node hierarchy | SOA Khipu planes, knot indices and KnotByIndex snapshots |
| Eager HyphenateTextBoxes and concrete hyphenation dependency | Lazy Hyphenator policy and owner DiscretionaryCandidates |
| Metric EncodeParagraph(para, knotAdapt) | EncodeParagraph(ctx, para) |
| Config.Shaper and contracts.Shaper / root Shaper | Config.Streaming with contracts.StreamingShaper |
| KnotStyler / AdaptKnot | Deliberately removed; space elasticity uses Params and resolved spacing |
| FixedWidthShaper and NewFixedWidthShaper, including root aliases | SyntheticStreamingShaper with SyntheticFont and chosen em size |
| String-based metric boundaries and cluster collector | Reader-based boundary preparation, glyph-cluster assembly and candidate widths |
| Metric fragment shaping and Pos/Len source-mapping fallback | Shared streaming fragment pipeline and retained original-source provenance |

The old `linebreak/kp` package was retired earlier. Modern firstfit and
knuthplass retain their own algorithms and provider policies. SOA KnotCore
metrics, manual knot fixtures and cached manual candidates remain supported;
removing a metric-only shaper does not remove linebreaker width arithmetic.

## Package Ownership

| Package | Responsibility |
| --- | --- |
| khipu | Paragraph storage, encoding pipeline, configuration, source provenance and discretionary ownership |
| contracts | StreamingShaper, RuneReader, font/style interfaces, value types and shared argument-error sentinel |
| helpers | SyntheticFont, DefaultStyleResolver and SyntheticStreamingShaper |
| linebreak | Shared paragraph shapes and WSS |
| linebreak/firstfit and linebreak/knuthplass | Breakpoint selection and applied-discretionary decisions |

Root aliases for surviving contracts/helpers preserve type and error identity.
Metric-specific aliases are gone. New adapters should import contracts directly.
Helpers never import the root engine; contracts never import implementations.

## SOA Encoder-To-Breaker Contract

Encoding preserves original UTF-8 source bytes. Normalize before establishing
styled ranges if desired. Boundary preparation evaluates paragraph-wide UAX#14
and UAX#29, intersects logical bidi/style runs and resolves shaping parameters.
A producer feeds those runs while the calling path assembles output clusters.

Shapers return logical-order glyphs with original scalar anchors and signed
font-unit positioning. Duplicate anchors collect glyphs into a cluster. Scaled
advances determine widths; offsets position ink, not pen movement. Fractional
advances accumulate before checked conversion to DU. Cluster boundaries alone
do not authorize line breaks.

Candidate publication retains textbox glyphs, shaping contexts, source spans and
boundary endpoints. Glue keeps metrics and source metadata, not glyphs.
Unsupported mandatory breaks fail explicitly rather than silently changing policy.
Invalid UTF-8, invalid anchors, width overflow or backend failure return no
partial paragraph. Every encoded paragraph, including empty input, ends with a
non-discardable final fill carrying a forced-break penalty.

U+00AD is suppressed before shaping, but its original source position is retained.
No alternative is eagerly shaped. Configure the owner as the breaker's provider
to discover explicit or eligible automatic replacements lazily. Complete fragments
use retained run parameters and publish transactionally after all shaping succeeds.

Breakers return line-ending knot indices and record applied choices in
SelectedDiscretionaries. Rerunning replaces selections. BreakpointPositions maps
a result to original paragraph-relative byte endpoints, including whitespace and
glyphless gaps. Selected variants require SourceSplit, independent of replacement
spelling. Manual knots without retained provenance cannot be mapped; Pos/Len is
not a fallback. Freezing and line-local visual ordering remain downstream work.

## Coverage And Validation Limits

General encoder, corpus and breaker tests now use streaming. Style-dependent
width tests use resolvers and font scaling. Scalar-anchor, zero-advance,
duplicate-anchor, source coverage and overflow tests preserve useful assertions
from the retired collector. Shared penalty normalization and helper character
fixtures survive independently of deleted metric tests. See
[TESTING.md](TESTING.md) for evidence and commands.

The corpus keeps its line-width regression guarantees. Migration exposed and
fixed sibling cords bounded-reader and iterator defects; those fixes must be
retained or released alongside khipu. They are not part of a khipu-only commit.
Full dependency suites have unrelated existing failures documented in
[RETIRE-METRICS.md](RETIRE-METRICS.md).

Validation uses the local sibling replacements selected by go.mod. Published
dependency compatibility, real font adapters, visual proofs, freezing and final
line materialization remain separate release/integration checks.
