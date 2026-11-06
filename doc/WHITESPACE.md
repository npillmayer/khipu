# Whitespace

## Unicode space characters and typographic behavior

| Unicode | Name | Typical width | Stretch/shrink in justification? | Line break? |
|---|---|---|---|---|
| `U+0020` | SPACE | Font-defined | **Yes** | Yes |
| `U+00A0` | NO-BREAK SPACE | Normal space | Depends on layout policy | No |
| `U+2002` | EN SPACE | ½ em | Normally no | Yes |
| `U+2003` | EM SPACE | 1 em | Normally no | Yes |
| `U+2004` | THREE-PER-EM SPACE | ⅓ em | Normally no | Yes |
| `U+2005` | FOUR-PER-EM SPACE | ¼ em | Normally no | Yes |
| `U+2006` | SIX-PER-EM SPACE | ⅙ em | Normally no | Yes |
| `U+2007` | FIGURE SPACE | Width of a digit | Normally no | No |
| `U+2008` | PUNCTUATION SPACE | Width of punctuation | Normally no | Yes |
| `U+2009` | THIN SPACE | Approximately ⅕ em | Normally no | Yes |
| `U+200A` | HAIR SPACE | Very narrow | Normally no | Yes |
| `U+202F` | NARROW NO-BREAK SPACE | Narrow, font-dependent | Normally no | No |
| `U+205F` | MEDIUM MATHEMATICAL SPACE | Approximately ⁴⁄₁₈ em | Normally no | Yes |
| `U+3000` | IDEOGRAPHIC SPACE | Typically 1 em | Normally no | Yes |

**Note**: The justification behavior describes conventional typographic defaults, not mandatory Unicode rules. Actual behavior depends on the layout engine, font and formatting properties.

## Candidate-Knot Policy

The internal candidate helpers classify original source through readers; they
do not infer whitespace from glyph IDs or identify words by whitespace.
Only exclusively U+0020 source is eligible for elasticity under `SpacingElastic`:

```
MinW = naturalWidth * (1 - SpaceShrinkPercent / 100)
W    = naturalWidth
MaxW = naturalWidth * (1 + SpaceStretchPercent / 100)
```

`Params` defaults to 50% stretch and 33.3% shrink (the latter is the decimal
factor 0.333, not exact one-third). Explicit zero disables that adjustment.
Percentages must be finite and nonnegative; shrink cannot exceed 100%.
Validation takes place when candidate widths are finalized.

`SpacingFixed`, NBSP, other Unicode whitespace, and inseparable mixtures of
space kinds remain fixed by default. Mixed material must not receive U+0020
elasticity across its entire width. Future accumulation can separate space kinds
where shaped-cluster boundaries permit; it must not split an inseparable cluster.

Natural width comes from scaled horizontal glyph advances, not the approximate
widths in the table. Offsets do not add width. Fractional DU are accumulated
before rounding to nearest DU, ties away from zero. Bounds are calculated from
the unrounded natural width. Invalid conversions, negative final widths, and
results reaching reserved fill/infinity dimensions are rejected.

Knot kind and discardability still follow the existing policy: whitespace-only
source is glue and receives the current glue flags, independently of elasticity.
This is not a new Unicode-wide trimming policy. Empty source creates no candidate.
Line-break penalties remain independent of classification and elasticity.

These helpers are used by the candidate accumulator in EncodeParagraph.
The former metric encoder and its KnotStyler hook have been removed.

## Unicode Characters relevant for Hyphenation

| Code point | Name | Behavior | TeX equivalent |
|---|---|---|---|
| `U+00AD` | SOFT HYPHEN | Optional line break with a hyphen when used | `\-` |
| `U+200B` | ZERO WIDTH SPACE | Optional line break without a visible hyphen | `\discretionary{}{}{}` (approximately) |
| `U+2010` | HYPHEN | Visible hyphen that normally permits a break afterward | Explicit `-` |
| `U+2011` | NON-BREAKING HYPHEN | Visible hyphen that prohibits a break | Hyphen in an unbreakable box |
| `U+2060` | WORD JOINER | Prohibits a line break at its position | `\nobreak` (approximately) |
