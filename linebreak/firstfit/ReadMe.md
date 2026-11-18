# firstfit: greedy paragraph breaking

`github.com/npillmayer/khipu/linebreak/firstfit` remembers the latest fitting
checkpoint and commits it when later material overflows. Use it for a simple,
local policy, especially ragged-right layouts. For paragraph-wide cost comparison,
see [knuthplass](../knuthplass/ReadMe.md).

## Break and freeze

This helper consumes an already encoded or bound paragraph. The root
[executable example](../../example_test.go) shows construction from styled text.

```go
package example

import (
    "context"
    "github.com/npillmayer/khipu"
    "github.com/npillmayer/khipu/dimen"
    "github.com/npillmayer/khipu/linebreak"
    "github.com/npillmayer/khipu/linebreak/firstfit"
)

func Layout(ctx context.Context, owner *khipu.Khipukamayuq, input *khipu.Khipu) (*khipu.FrozenKhipu, error) {
    layout := firstfit.DefaultLayout(linebreak.RectangularParShape(100 * dimen.BP))
    breaks, err := firstfit.BreakParagraph(input, layout, nil)
    if err != nil { return nil, err }
    return owner.Freeze(ctx, input, breaks, layout)
}
```

`DefaultLayout` gives the physical right skip `fil` stretch. Positive slack
normally goes there, leaving ordinary interword spaces at natural width. The
higher-order default paragraph fill wins on the final line. Fit uses trimmed
**minimum** width including skips, so supplied shrink can still be used. To keep
material rigid, supply equal minimum, natural and maximum widths.

Breaks are zero-based line-ending knot indices, including the generated trailer.
They do not contain final widths or all discretionary information. Freeze with
the same layout before another successful breaker run replaces selections.

## Hyphenation and errors

For lazy discovery, configure `Config.Hyphenator` on the owner, then pass:

```go
params := firstfit.NewDefaultParameters()
params.DiscretionaryProvider = owner
// Use params as BreakParagraph's third argument.
```

Both explicit soft-hyphen material and automatic discovery need a policy;
`Config{}` installs none. Without a provider, the breaker can use cached
candidates. Rescue is local to an overflowing textbox: automatic discovery needs
an earlier ordinary checkpoint, while explicit markers can rescue the first word.
`MinHyphenGain` is an inclusive natural-width gain threshold; default zero permits
zero improvement. It is not a hyphen-frequency limit.

Nil algorithm parameters select independent defaults. Geometry is always supplied
explicitly. Failures return an error and preserve prior selections, although
provider discovery may populate caches. Overfull fallback can be a successful
break result; inspect frozen fit diagnostics. Serialize use of the input/owner.

Ordinary `BreakParagraph` and provider calls are synchronous and have no context argument. The
context in the example controls freezing; it does not cancel the preceding
search or lazy shaping. See the [algorithm and limits](../../doc/FIRSTFIT.md),
[hyphenation contract](../../doc/HYPHENATION.md), and [freeze guide](../../doc/FREEZE.md).

## Checkpoint continuation

`ContinueParagraph(ctx, binding, geometry, checkpoint)` searches only the suffix
after an opaque `khipu.BreakCheckpoint`. Obtain the checkpoint with
`owner.Checkpoint(ctx, prepared, completeBreakPath, acceptedLines)` after a
successful ordinary layout. It certifies the complete legal path and copies the
accepted prefix. A KP prefix may be retained even when first-fit would have
chosen different boundaries. No KP optimization state is resumed.

Bind the same `NoHyphenation` preparation; nil/zero checkpoints and bindings from
another preparation are rejected. The seeded search has no discretionary provider
or replacement carry and performs no base shaping or font acceptance. Prefix
knots are not searched. Zero accepted lines starts from the beginning; accepting
all lines returns the copied path without a search. Ordinary `BreakParagraph`
behavior is unchanged.

Use the original widths for every accepted line and the new width for the suffix
in a stable one-based `LineShape`, with unchanged skips. Freeze the entire result
using that same geometry. A new checkpoint can lock more lines on the next call.
Errors/cancellation publish no partial path and preserve previous selections.
Cancellation is checked between search knots, after width callbacks and before
publication; one synchronous callback is not interruptible.

Checkpoints retain preparation while flow/retry remains possible. Drop working
handles when each paragraph finishes; frozen content stays usable. See the
[preparation and lifecycle guide](../../doc/PREPARED_PARAGRAPHS.md#checkpoint-continuation).

For source reading, start with [the sequential scan](break.go), then follow
[segment and carry bookkeeping](segment.go) or [discretionary rescue](discretionary.go).
[Parameters and layout defaults](parameters.go) are separate from the scan.

The old `LICContent`, `LICTrimDiscardable`, and `LICRetainedNeutral` constants
remain for source compatibility but are deprecated. For knot inspection use
`KnotCore.IsContent()` and `KnotCore.IsDiscardable()`; discardability takes
precedence for line-edge trimming. These constants do not configure the breaker.
