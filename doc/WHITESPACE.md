# Whitespace

This document defines khipu's current candidate-knot whitespace policy.
Elasticity, edge discardability and line-break eligibility are separate decisions;
none should be inferred from either of the others.

## Implemented Candidate-Knot Policy

The internal candidate helpers classify original source through readers; they
do not infer whitespace from glyph IDs or identify words by whitespace.
Only exclusively U+0020 source is eligible for elasticity under `SpacingElastic`:

```text
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
widths in the reference table below. Offsets do not add width. Fractional DU are accumulated
before rounding to nearest DU, ties away from zero. Bounds are calculated from
the unrounded natural width. Invalid conversions, negative final widths, and
results reaching reserved fill/infinity dimensions are rejected.

Knot kind and discardability still follow the existing policy: whitespace-only
source is glue and receives the current glue flags, independently of elasticity.
This is not a new Unicode-wide trimming policy. Empty source creates no candidate.
Line-break penalties remain independent of classification and elasticity.

For line-edge trimming, non-discardable textboxes and `KTAtomic` items count as
content. Zero or negative atomic advances still shelter intervening spaces.
Atoms are never discarded. Neutral retained items, such as paragraph fill, do
not shelter adjacent spaces. Both linebreakers and freezing use this rule.

These helpers are used by the candidate accumulator in `EncodeParagraph`.
Linebreakers consume the resulting width bounds and discardable flags. Freezing
allocates final widths; allowing glue to shrink also permits a linebreaker to
accept a line wider than its target at natural width, provided its minimum width
fits. See [firstfit's fitting policy](FIRSTFIT.md#algorithm-and-fitting) and
[freezing](FREEZE.md).

## Typographic Reference

The following table gives conventional width and justification guidance, not
mandatory Unicode rules or khipu configuration. In particular, khipu's NBSP policy
is fixed-width, as specified above. Actual widths come from shaping.

"Break tendency" is only a shorthand: surrounding context and the configured
Unicode boundary processing determine actual opportunities. Do not use this table
as a per-character penalty lookup or as a discardability table.

| Unicode | Name | Typical width | Conventional elasticity | Break tendency |
|---|---|---|---|---|
| `U+0020` | SPACE | Font-defined | Yes | Breakable |
| `U+00A0` | NO-BREAK SPACE | Normal space | Depends on layout policy | Nonbreaking |
| `U+2002` | EN SPACE | ½ em | Normally no | Breakable |
| `U+2003` | EM SPACE | 1 em | Normally no | Breakable |
| `U+2004` | THREE-PER-EM SPACE | ⅓ em | Normally no | Breakable |
| `U+2005` | FOUR-PER-EM SPACE | ¼ em | Normally no | Breakable |
| `U+2006` | SIX-PER-EM SPACE | ⅙ em | Normally no | Breakable |
| `U+2007` | FIGURE SPACE | Width of a digit | Normally no | Nonbreaking |
| `U+2008` | PUNCTUATION SPACE | Width of punctuation | Normally no | Breakable |
| `U+2009` | THIN SPACE | Approximately ⅕ em | Normally no | Breakable |
| `U+200A` | HAIR SPACE | Very narrow | Normally no | Breakable |
| `U+202F` | NARROW NO-BREAK SPACE | Narrow, font-dependent | Normally no | Nonbreaking |
| `U+205F` | MEDIUM MATHEMATICAL SPACE | Approximately ⁴⁄₁₈ em | Normally no | Breakable |
| `U+3000` | IDEOGRAPHIC SPACE | Typically 1 em | Normally no | Breakable |

## Break-Control Reference

These are conceptual descriptions, not a complete boundary algorithm. TeX
equivalents are mnemonic only, not khipu's representation of discretionaries.
For lazy discovery, shaped replacements, source provenance and selected variants,
see [the hyphenation contract](HYPHENATION.md). In particular, replacement glyphs
are supplied through shaping, not inserted by the linebreaker.

| Code point | Name | Behavior | TeX equivalent |
|---|---|---|---|
| `U+00AD` | SOFT HYPHEN | Optional line break with a hyphen when used | `\-` |
| `U+200B` | ZERO WIDTH SPACE | Optional line break without a visible hyphen | `\discretionary{}{}{}` (approximately) |
| `U+2010` | HYPHEN | Visible hyphen that normally permits a break afterward | Explicit `-` |
| `U+2011` | NON-BREAKING HYPHEN | Visible hyphen that prohibits a break | Hyphen in an unbreakable box |
| `U+2060` | WORD JOINER | Prohibits a line break at its position | `\nobreak` (approximately) |
