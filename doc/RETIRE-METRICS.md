# Retiring Metric-Only Shaping

Status: all three changes implemented. Metric-only shaping has been removed.
This follows the earlier AOS-to-SOA migration; SOA metrics and manually supplied
breaker fixtures remain supported.

## Decisions And API Boundary

- The sole encoding signature is
  `EncodeParagraph(ctx context.Context, para *styled.Paragraph) (*Khipu, error)`.
  The former streaming-suffixed name has no forwarding alias.
- `KnotStyler` and arbitrary `AdaptKnot` width customization have been removed,
  without a replacement hook. Existing streaming spacing parameters remain.
- `Config{}` now enables synthetic streaming encoding. Nil `Config.Streaming`
  installs a factory returning a fresh unbuffered synthetic stream for each
  paragraph or fragment, `DefaultStyleResolver`, a synthetic font at 10 pt,
  and elastic U+0020 spacing. Ordinary synthetic cells are 5 pt. Language/script
  remain unspecified in run defaults; they are not inferred from legacy Params.
- Non-nil `StreamingConfig` values remain complete configurations, not deltas.
  Missing services, fonts or invalid sizes fail; no real backend is silently
  replaced with a synthetic backend. Defaults/features are copied as before.
- The default resolver rejects non-nil application styles. Clients needing
  those must supply a resolver. Synthetic defaults are not real font shaping.
- `Params`, `Hyphenator` and `ParFillSkip` retain their ownership and explicit-zero
  semantics. No automatic discovery occurs without a hyphenator/provider.
- The old `EncodeParagraph(para, knotAdapt)`, `Config.Shaper`, `contracts.Shaper`,
  `KnotStyler`, and fixed-width metric helper APIs have been deleted.

`StreamingConfig` remains nested. Flattening it is not necessary for retirement.

## Historical Production Inventory

The removal column records code retired in change 3; preserved items remain
supported. Notes below describe the migration inventory, not additional work.

| Area | Removed | Preserved |
| --- | --- | --- |
| `encoding.go` | Metric `EncodeParagraph` and `spaceBoundary` | Streaming encoder, candidate accumulation and its trailer rules |
| `encoding_metrics.go` | `clusterMetrics`, `getMetrics`, metric width accumulation | Streaming glyph-cluster assembly and candidate metrics |
| `encoding_boundaries.go` | String-based `paragraphBoundaries` | Reader-based `prepareParagraphBoundaries` |
| `contracts/shaping.go` | `Shaper` | `RuneReader`, used by streaming contracts |
| `config.go`, `environment.go`, `khipu.go` | `Config.Shaper`, `typEnv.shaper`, fixed-width metric default construction | Streaming configuration/factory, parameter snapshots, ownership and paragraph fill |
| `khipu.go` | `KnotStyler`, `sanitizeKnot`, `kindFromText`, `wrapAsRuneReader` once their test dependencies migrate | SOA storage, `KnotCore`, width/flag planes, append and discard operations |
| `discretionary.go` | Metric branch of `DiscretionaryCandidates`, `textboxSource`, `styleChangeForKnot`, `shapeDiscretionaryFragment` | Candidate/selection types and maps, cached lookup, flags, `discretionaryPenalty`, streaming dispatch |
| `helpers/fixed_width_shaper.go`, `compat.go` | Metric helper, constructor, metric-specific aliases/forwarder | Synthetic streaming helper, font/style helpers and unrelated aliases |
| `breakpoint_positions.go` | Metric span fallback and its UTF-8 byte probe once only streaming-encoded paragraphs remain | Boundary provenance and selected `SourceSplit` mapping |

Important shared dependencies:

- `defaultFlagsForKind` is used by streaming candidate metrics, despite living
  in `discretionary.go`. Do not delete it with metric fragment shaping.
- `discretionaryPenalty` is used by both explicit and automatic streaming
  discovery. Candidate metric fields and penalties remain necessary.
- `normPenalty` in `penalties.go` is used by reader-based boundary preparation.
  Its tests now live in `penalties_test.go`, independently of legacy metrics.
- `syntheticZeroWidth` moved from the deleted metric helper to
  `helpers/synthetic_streaming_shaper.go`; character-policy tests remain active.
- `strings` in `khipu.go` remains needed for `Khipu.String`; removing the legacy
  text classifier does not justify deleting that import indiscriminately.
- Manually constructed metric-only knots remain legitimate independent breaker
  fixtures. Retiring a shaper does not require glyphs on every `Khipu` value.
  Such values do not gain original-source mapping without retained provenance.

Neither linebreaker calls the metric shaper or encoder in production. They
consume `KnotCore` widths and request alternatives through the provider interface;
their algorithms and `linebreak.WSS` do not need to change.

## Caller And Coverage Migration

The local source search found no imports of `github.com/npillmayer/khipu` in
the other searched sibling projects under `/Users/npi/prg/go` (excluding this
repository and the GOPATH dependency cache). The sibling `tyse` has its own older
package named `khipu`, not a caller of this module. This is a local inventory,
not proof that published or untracked external clients do not exist.

All test and example encoder callers now use streaming. There are no test calls
to the old `EncodeParagraph`, `AdaptKnot`, `paragraphBoundaries`, `kindFromText`,
or `env.shaper`. The following records coverage disposition:

| Tests / examples | Change 2 outcome |
| --- | --- |
| `encoder_integration_test.go` | Streaming fixtures preserve trailer, original offsets, empty input, style, forced-break, source-section, error/retry and lazy-discovery assertions. Scripted streams cover repeated scalar anchors, zero-width clusters, invalid anchors, and cluster/knot overflow. Selected breaks now also assert original byte endpoints. |
| `paragraph_regression_test.go` | Corpus, line counts and minimum-width assertions remain. Style resolution/font-size tests replace the intentionally retired adaptation hook. Space percentages reproduce the old corpus elasticity, with explicit checks in later cord chunks. Breaker tolerances and algorithms are unchanged. |
| `example_test.go` | Both examples use streaming; the hyphenator supplies an explicit request-relative SourceSplit. |
| `helpers_example_test.go` | A 14 pt synthetic em gives 7 pt cells, replacing the metric helper configuration example. |
| `config_test.go`, `khipukamayuq_test.go`, `compat_test.go` | Streaming collaborator failure after partial output, fresh defaults/reuse, and surviving font/style/stream aliases and error identities. |
| `discretionary_test.go` | Storage/flags/cache fixtures retained. Discovery now shapes through streaming and asserts retained fragment IDs and SourceSplit as well as widths and penalties. |
| `encoding_metrics_test.go` | Only isolated legacy collector tests remain until deletion. Their useful behaviors also have streaming coverage; shared normalization tests moved to `penalties_test.go`. |
| `boundary_preparation_test.go` | Explicit scalar-end penalty fixtures replace the legacy calculator as oracle, including multibyte input, controls, CJK, hard breaks and one-byte reads. |
| `candidate_metrics_test.go` | Classification now checks explicit knot kinds and flags, not `kindFromText`. |
| `shaping_storage_test.go`, `discretionary_eligibility_test.go`, `breakpoint_positions_test.go` | Manual knot fixtures test absent glyph metadata without invoking legacy encoding; position tests use streaming and still reject unknown or invalid SourceSplit. |
| `encoding_stream_test.go` | Temporary cross-encoder assertions removed; default/configuration, lifecycle and fragment-reshaping guarantees retained. |
| `linebreak/**` | Direct knot/width fixtures and provider stubs unchanged; these are independent algorithm tests, not legacy shaping. |

Change 3 deleted `encoding_metrics_test.go` and
`helpers/fixed_width_shaper_test.go` with their implementations. Shared helper
fixtures moved to `helpers/fixtures_test.go`, preserving streaming character-policy
coverage. Source mapping now rejects absent retained provenance; provider tests
verify that manual knots do not trigger discovery but cached manual candidates
remain available.

### Dependency Fix Required By The Corpus

Migration exposed incorrect `cordext.ByteRangeBounded` indexing in the sibling
`cords` checkout: spans after the first chunk could read empty, making spaces
non-discardable textboxes. This was not a linebreaker tolerance problem.
With approval, `cords` was fixed rather than weakening the corpus assertions:

- Use absolute byte bounds and the starting chunk's cumulative position, including
  exact chunk edges; keep state local to each iterator traversal.
- Report the full number of bytes written by `BoundedReader.Read`; return usable
  EOF readers for empty ranges.
- Propagate `yield(false)` through the underlying btree traversal. New early-stop
  tests exposed this additional iterator contract violation.

Focused tests in `cordext/bounded_reader_test.go` and
`btree/iterate_stop_test.go` cover cross-chunk/multibyte byte ranges, absolute
indices, repeated traversal, early stop, small read buffers and EOF counts.
The migration therefore depends on these sibling changes being retained or
released alongside it; they are not contained in a khipu-only commit.

Legacy behaviors are not an automatic specification for streaming: style
resolution, whitespace elasticity, word boundaries, omitted controls and explicit
soft hyphens already have intentional differences. Record such differences when
migrating assertions rather than weakening the tests until they pass.

## Change Sequence And Checks

1. **Preparation (complete):** record boundaries, enable synthetic streaming
   defaults, deprecate metric APIs and make the primary quickstart use streaming.
   Test independent defaults, repeated calls, retained glyphs, disabled discovery,
   lazy fragment reshaping, incomplete explicit configs and zero parameters/fill.
2. **Caller/coverage migration (complete):** the table above records migrated
   assertions and intentional retirement of KnotStyler coverage. The old
   production implementation remained available at that stage; no linebreaking algorithm changed.
3. **Deletion (complete):** remove the inventoried legacy code, simplify provider and source
   mapping dispatch, move streaming encoding to `EncodeParagraph(ctx, para)`,
   and refresh affected docs/examples. Check references to every removed symbol.

After each change run `go test ./...`, `go test -race ./...`, `go vet ./...`, and
`git diff --check`. The deletion change should also build all packages and check
dependency tidiness. Tests use the sibling dependency checkouts selected by go.mod;
published dependency compatibility remains a separate release check.

No freezing, visual ordering, renderer helpers, negative candidate caching or
new discretionary policies belong in this cleanup.

Change 3 validation: full khipu tests and race tests, vet, build and diff checks
pass. `go mod tidy -diff` reports no dependency changes. Removed API references
remain only in migration/history notes; all executable callers use
`EncodeParagraph(ctx, para)`. Shared manual-knot fixtures and cached candidates
remain supported without metric shaping or guessed source mapping.

Change 2 validation: the full khipu suite, race suite and vet pass. The affected
cords `btree`/`cordext` suites, race checks and vet pass. Full cords testing still
fails in `TestStyleNoOpOnEmptySpan`, `TestConsoleFixedWidth`, and `TestHTMLSimple`.
Each failure was reproduced with a Go overlay restoring the pre-fix reader and
iterator sources, while leaving the existing styled-package changes untouched.
They are outside this migration; no styled code, dependency versions or user
changes were reverted.
