# contracts

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
| `Shaper` | `RuneReader`, `dimen.DU`, `styled.BidiStyleChange` | Active boundary for paragraph encoding and discretionary measurement through `khipu.Config.Shaper` |
| `FontLoader`, `FontFace`, `FontScale` | `FontMetrics` | Draft backend boundary; a synthetic face is available in `helpers` |
| `StyleResolver` | `ResolvedRunStyle`, `SpacingMode` | Draft run-resolution boundary; a default implementation is available in `helpers` |
| `StreamingShaper` | `ShapeParams`, `ShapeFeature`, `ShapeBufferOptions`, `ShapeFlushBoundary`, `ShapedGlyph` | Draft streaming boundary; a standalone synthetic implementation is available in `helpers` |

The font, resolver, and streaming APIs are not yet integrated into paragraph
encoding. `StreamingShaper` cannot be supplied to `Config.Shaper`; these are
different interfaces with different units and lifecycles.

Paragraph-specific `KnotStyler` and `Hyphenator` remain in the root `khipu`
package. Paragraph shapes and discretionary-provider interfaces remain with
their linebreaker consumers.

## Metric Shaping

[`Shaper`](shaping.go) supplies metrics in logical source order:

- `SetSource` starts a run and resets run-relative byte offsets.
- `SetStyle` supplies the run's style and bidi information.
- `ReadClusterMetrics` returns a cluster anchor, a width in `dimen.DU`, an
  informational source length, and an error.

Anchors identify the first UTF-8 byte of each cluster and must not decrease.
Repeated anchors accumulate width. Zero-width metrics still mark cluster starts.
EOF carries no metric; other errors abort encoding. The encoder determines
source spans from anchors rather than the returned informational length.

Instances used by `Config` are retained, not cloned. Do not use a mutable shaper
concurrently through an owner, another owner, or direct calls.

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
are errors. Explicit bounds beyond the span are clipped. The internal
preparation stage is available, but the encoder does not call it yet.

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
| `Shaper` cluster anchor | Run-relative UTF-8 byte offset |
| `Shaper` width | `dimen.DU` |
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

Root names such as `khipu.FontFace` and `khipu.ShapeParams` remain type aliases.
`khipu.ErrIllegalArguments` retains the same sentinel identity. Prefer the
canonical `contracts` names in new adapters. See
[compatibility declarations](../compat.go) and the [migration guide](../doc/MIGRATION.md).

For concrete usage, see the [helper guide](../helpers/ReadMe.md), the
[encoding example](../helpers_example_test.go), and the
[testing strategy](../doc/TESTING.md).
