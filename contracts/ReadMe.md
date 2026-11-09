# contracts

Streaming shaping and style resolution are integrated into paragraph encoding
and discretionary reshaping. The metric-only `Shaper` has been removed;
`RuneReader` remains the streaming input contract. See
[the integration guide](../README.md#integration-and-ownership).

`github.com/npillmayer/khipu/contracts` defines the font, shaping, and style
services used by khipu. Implement an adapter against this package when providing
a font backend or application-specific style resolution.

```go
import "github.com/npillmayer/khipu/contracts"
```

The package contains interfaces, their parameter and result types, constants,
and a shared argument-error sentinel. Concrete defaults are in
[`helpers`](../helpers/ReadMe.md).

## Scope and Integration Status

| Service | Associated types | Current use |
| --- | --- | --- |
| `FontLoader`, `FontFace`, `FontScale` | `FontMetrics` | Font metrics/scaling used by encoding; loading remains the adapter's responsibility |
| `StyleResolver` | `ResolvedRunStyle`, `SpacingMode` | Active run resolution; a default implementation is available in `helpers` |
| `StreamingShaper` | `ShapeParams`, `ShapeFeature`, `ShapeBufferOptions`, `ShapeFlushBoundary`, `ShapedGlyph` | Active shaping boundary; a synthetic implementation is available in `helpers` |

The font, resolver, and streaming APIs are used by `EncodeParagraph(ctx, para)`
through `Config.Streaming`, with synthetic defaults for `Config{}`.
Paragraph-specific `Hyphenator` remains in the root package. Paragraph shapes
and discretionary-provider interfaces remain with their linebreaker consumers.

## Fonts and Style Resolution

[`FontLoader`](font_api.go) loads a face from an `io.Reader`; the adapter retains
the backend data and any required backing storage. Font discovery, family
matching, and fallback selection are outside this interface.

`FontFace` provides immutable metrics and creates a scale for a positive em size.
`FontMetrics` uses signed, unscaled font units. `FontScale.ToDU` converts to
fractional DU without changing signs or axis orientation. Accumulate fractions
before rounding; the caller checks range before converting to `dimen.DU`.
Scales must be immutable and safe for concurrent reads. Shaping adapters must
reject faces from backends they do not support.

[`StyleResolver`](style_resolver.go) returns a complete `ResolvedRunStyle`, not a
property delta. Nil style means no overrides of the supplied defaults. Successful
results require a non-nil font and positive em size. Fixed spacing and an empty
feature list are valid explicit settings.

Resolvers must not mutate style or defaults, and returned feature storage must
remain stable. Supply inherited defaults independently for each run. Font scaling
follows resolution; bidi analysis, script-based run subdivision, and font
fallback are separate responsibilities. `SpacingElastic` describes a policy,
not numerical stretch/shrink capacities.

Features in `ResolvedRunStyle` and its supplied defaults refer to original
scalar positions within the originating style span, including omitted bidi
controls. Preparation resolves each contributing span once, clips features to
each bidi intersection, and rebases them for `ShapeParams`. Nonpositive bounds
mean beginning/end; empty overlaps are omitted and positive reversed ranges
are errors. Explicit bounds beyond the span are clipped. The encoder performs
this preparation before starting its shaping producer.

## Streaming Shaping

[`StreamingShaper`](shaping_stream.go) accepts uniform runs and publishes glyph
records on one output channel. `Shape` is synchronous and may block until the
consumer drains output. Run the producer and consumer concurrently.

| Operation or outcome | Contract |
| --- | --- |
| Successful `Shape` | Leaves output open for subsequent runs |
| `Finish` | Closes output after the final run |
| Accepted shaping failure or cancellation | Closes output with terminal `Err`; previously emitted glyphs remain drainable |
| `Finish` or `SetInitialCluster` during `Shape` | Reports an error without disrupting the active call |
| Consumer stops draining | Consumer cancels the producer context; an executing source read need not be interrupted |

Concurrent `Shape` calls are unsupported. `Glyphs` and `Err` may be queried
concurrently. A finished instance cannot be reused for later discretionary
shaping. Commit accumulated paragraph output only after success.

`ShapedGlyph` records use logical shaping order, including RTL runs. Glyph IDs
are meaningful only with their run's face. Advances and offsets are signed font
units, converted using the run's `FontScale`. Advances move the pen; offsets move
the glyph relative to that pen. The channel has no run/cluster completion markers.

## Coordinates and Units

| Value | Coordinate or unit |
| --- | --- |
| `ShapedGlyph.Cluster` | Continuing original Unicode scalar ID, with an optional initial-base override |
| Streaming advances and offsets | Signed font units |
| `ShapeFeature.Start` / `End` | Half-open run-relative original scalar range; nonpositive start/end mean beginning/end |
| Buffer sizes and watermarks | Input scalar counts, independent of output channel capacity |

For `aéz`, one-scalar clusters have byte anchors `0, 1, 3`, but scalar anchors
`0, 1, 2` with initial base zero. Streaming bases continue by input scalar count,
including scalars producing no output. Cluster boundaries alone do not authorize
line breaks; source spans and penalties remain layout metadata.

## Dependencies, Errors, and Compatibility

`contracts` imports neither the root engine nor `helpers`. It uses `dimen` and
the accepted text/style dependencies, allowing adapters to depend on service
definitions without importing paragraph storage or default implementations.

`ErrIllegalArguments` identifies invalid arguments. Implementations may wrap it;
use `errors.Is(err, contracts.ErrIllegalArguments)` when checking it. Other
failures follow the relevant service contract and implementation documentation.

Import service types and constants directly from `contracts`; the root package
does not re-export them. `khipu.ErrIllegalArguments` retains the same sentinel
identity. See the [integration guide](../README.md#integration-and-ownership).

For concrete usage, see the [helper guide](../helpers/ReadMe.md), the
[encoding example](../helpers_example_test.go), and the
[testing strategy](../doc/TESTING.md).
