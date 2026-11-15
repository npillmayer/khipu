# knuthplass: paragraph cost optimization

`github.com/npillmayer/khipu/linebreak/knuthplass` compares complete break paths by
accumulated demerits. Use it when balancing line quality across a paragraph matters.
It chooses the best path among candidates explored in the first successful pass;
it does not enumerate every possible hyphenation or implement all TeX policies.
For a simpler greedy policy, see [firstfit](../firstfit/ReadMe.md).

## Break and freeze

This helper takes an encoded or bound paragraph. See the root
[executable examples](../../example_test.go) for source construction and providers.

```go
package example

import (
    "context"
    "github.com/npillmayer/khipu"
    "github.com/npillmayer/khipu/dimen"
    "github.com/npillmayer/khipu/linebreak"
    "github.com/npillmayer/khipu/linebreak/knuthplass"
)

func Layout(ctx context.Context, owner *khipu.Khipukamayuq, input *khipu.Khipu) (*khipu.FrozenKhipu, error) {
    layout := knuthplass.DefaultLayout(linebreak.RectangularParShape(100 * dimen.BP))
    params := knuthplass.NewDefaultParameters()
    breaks, err := knuthplass.BreakParagraph(input, layout, params)
    if err != nil { return nil, err }
    return owner.Freeze(ctx, input, breaks, layout)
}
```

Geometry and algorithm parameters are separate. `DefaultLayout` supplies zero
left skip and a right skip with 50 BP finite stretch, rather than firstfit's fil
skip. Reuse the resolved layout unchanged when freezing. The input must include
the generated non-discardable forced-break trailer, which encoding/binding supplies.

## Choose a profile

| Profile | PreTolerance | Tolerance | EmergencyStretch |
| --- | ---: | ---: | ---: |
| `NewDefaultParameters()` (also nil parameters) | 100 | 200 | 0 |
| `NewTolerantParameters()` | 100 | 5000 | 50 BP |

Both return independent values with no discretionary provider. The tolerant
profile also reduces final-hyphen demerits from 10000 to 50. For lazy alternatives,
configure the owner's `Hyphenator`, then set `params.DiscretionaryProvider = owner`.
This is required for explicit marker material as well as automatic discovery.
Cached candidates remain usable without a provider.

Pass 1 uses pretolerance without discovery. If it fails, pass 2 enables discovery
and uses tolerance. Positive finite emergency stretch enables pass 3. The first
complete pass wins, and only the final configured pass may use fallback
transitions. Provider errors abort the call instead of triggering another pass.
Emergency stretch changes scoring, not physical widths; freezing can still
report insufficient capacity.

## Configure typed values

The five numeric score fields use the public `Merits` type (underlying `int64`).
Convert values from application configuration explicitly:

```go
configuredTolerance := int64(500) // for example, from a flag or configuration file
params := knuthplass.NewDefaultParameters()
params.Tolerance = knuthplass.Merits(configuredTolerance)
```

The same conversion applies to `PreTolerance`, `LinePenalty`,
`FirstHyphenDemerits`, and `FinalHyphenDemerits`; see the
[executable example](public_api_test.go). `Merits` aliases the original defined
type, so existing literals, field assignments and inferred values remain compatible.
`EmergencyStretch` remains a physical `dimen.DU` value, and knot/discretionary
penalties remain `khipu.Penalty` values.

Scores are dimensionless. Computed badness ranges from 0 to 10000, so that is the
useful tolerance range. Negative tolerances screen out even zero badness; values
above 10000 add no further quality allowance. Final-pass fallback can still
complete a paragraph after screening fails. `LinePenalty` is added to badness
before squaring; first/final-hyphen settings are additive costs. Costs are signed,
not clamped or overflow-checked: keep squared line costs and accumulated totals
within `int64`. Start from a built-in profile and use modest overrides.

## Results and limits

Returned indices are zero-based line endings, including the trailer. A successful
call replaces `input.SelectedDiscretionaries`; failure preserves prior selections
but may leave newly discovered candidates cached. Freeze before another successful
run replaces those choices. Serialize owner/input operations.

Search and provider calls have no context argument. Encoding/freezing cancellation
does not cancel them. `ErrNoParShape` identifies a nil shape and `ErrNoBreakpoints`
identifies empty input or an exhausted search; other argument/provider errors
are returned separately. Capacity shortfalls can still produce successful frozen
results with fit diagnostics.

Fitness classes, consecutive-hyphen demerits, and exact symbolic fill arithmetic
in search are not implemented. Built-in rectangular shapes permit state compaction;
custom shapes retain line-count alternatives and may cost more.

## Compatibility names

The following exports remain available for existing source code, with their
original types and values. They are deprecated implementation details:

| Legacy name | Use in new client code |
| --- | --- |
| `BreakRef`, `PlainBreak`, `BreakRef.Kinx` | Use `BreakParagraph`'s integer indices and the input's `SelectedDiscretionaries`; freeze for final queries. |
| `NoBreakRef` | Use the returned error to detect failure. Legacy code needing a sentinel can keep its own `BreakRef{At: -1}`. |
| `LICContent`, `LICTrimDiscardable`, `LICRetainedNeutral` (in both breakers) | Inspect `KnotCore.IsContent()` and `KnotCore.IsDiscardable()`. Discardability takes precedence for trimming. |
| `khipu.DefaultInitialKhipuCapa` | Let owner encoding/preparation manage storage; capacity is not a paragraph size limit. |

The breaker never reads `NoBreakRef`; changing that variable does not configure
search. `BreakRef` helpers remain usable by legacy clients, but the public breaker
does not accept or return search records. The penalty thresholds
`InfinityMerits`/`InfinityDemerits` remain supported, and `AwfulDemerits` documents
the base fallback cost rather than a maximum paragraph score.

Read the [algorithm](../../doc/KNUTH+PLASS.md), [scoring notes](../../doc/KNUTH_PLASS_SCORING.md),
[performance guide](../../doc/PERFORMANCE.md), and [hyphenation contract](../../doc/HYPHENATION.md)
for the detailed policies and tradeoffs.

For source reading, start with [the entry point](knuthplass.go) and
[pass orchestration](passes.go), then follow [the scan](search.go) and
[boundary transactions](boundary.go). [Geometry](geometry.go),
[demerit scoring](scoring.go), and [result collection](results.go) have separate
homes. The [source map](../../doc/KNUTH+PLASS.md#code-reading-guide) lists the remaining topics.
