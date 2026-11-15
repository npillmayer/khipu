# Frozen API Delivery Record

Historical implementation and verification notes, archived November 15, 2026.
The steps below are completion records, not work required of API clients.
For current behavior, use the [result contract](../FROZEN-API.md),
[freezing guide](../FREEZE.md) and [test evidence](../TESTING.md).

## Delivery Plan

### Complete Pipeline Verification

The public EncodeParagraph -> BreakParagraph -> Freeze pipeline is exercised
with both firstfit and Knuth-Plass by `pipeline_integration_test.go`, alongside
the public cancellation/retry and varying-shape tests. Checks include mixed bidi
ordering, source coverage, glyph/width consistency, explicit and automatic
replacements, fixed-width spaces, finite shrink, finite overstretch, overfull
words, empty paragraphs and competing fill orders. Frozen output survives later
replacement of the input's selected decisions; freezing does not call shaping
or hyphenation services. An executable Freeze example demonstrates the sequence.

These are synthetic-font contract tests, not typography proofs. They do not
require identical break decisions from the two algorithms. In particular, a fil
skip can make a shorter K&P line preferable to a compressed line; the shared
shrink fixture uses zero side skips to exercise finite shrink with both engines.
Font-backed adapters, broader script corpora and elastic-textbox realization
remain separate work. There were no production changes required by this sweep.

1. **Implemented:** Params is the shared geometry source for both breakers and
   internal freezing. Default profiles resolve before breaking, with firstfit's
   right skip set to fil. Tests cover explicit zero overrides, shared geometry,
   ragged-right interword spacing, finite shrink, fill precedence, RTL physical
   edges, invalid configuration, parameter snapshots and existing break paths.
2. **Implemented:** opaque public result, final line/knot value records and query
   methods. Map existing storage without exporting its planes. Add tests for
   index conventions, empty/silent material, detached values and exact widths.
3. **Implemented:** CopyText and source access. Tests cover snapshot independence, replacement
   coordinates, feature/context preservation, rebased cluster glyph ranges,
   retained lifetime, bounded cord reads and concurrent read-only access.
4. **Implemented:** Freeze consumes resolved Params, promotes the unsupported-adjustment sentinel and threads
   cancellation checks through
   internal stages. Move test-only integration bridges to public API tests for
   both breakers, including failure/retry and input preservation.

Prefer a separate shared-geometry migration followed by the result/API changes.
No renderer helpers, breaker algorithm redesign, arena compaction or elastic-text
realization are prerequisites. Future kashida-like support changes how final glyphs and
widths are produced, not the meaning of this result: successful output must
always contain mutually consistent realized widths and retained shaping data.
