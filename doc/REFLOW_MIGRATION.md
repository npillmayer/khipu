# Reflow PR 1: immutable material and independent trials

Implemented in khipu, 2026-11-19. Typography adoption (PR 2) and tytui/tycli
adoption (PR 3) remain deferred. This is a source API migration, without a new
replay format or backend serialization. The upstream bidi limitation is unchanged.

| Former interface/state | Current contract |
| --- | --- |
| `PrepareParagraph` → `PreparedParagraph` | Returns immutable SoA `Khipu` and matching synchronized `Discovery` |
| `EncodeParagraph` → mutable Khipu | Returns the same two handles for text-only input |
| Public SoA planes, mutators and selected maps | Detached `KnotTemplate`/bound `KnotCore` queries; no public mutation |
| Owner `BindParagraph` copying base knots | Package `BindParagraph(k, advances)` overlays canonical copied atomic advances |
| Owner/provider candidate cache on a working Khipu | Complete immutable alternative batches on `Discovery`; failed batches retry |
| Breaker returns indices and replaces shared selections | Breaker accepts `BreakRequest` and returns independent immutable `Tikray` |
| Owner `Freeze(k, breaks, params)` | `Freeze(plan)` uses that plan's captured geometry/replacements |
| Owner intrinsic measurement on preparation | `MeasureIntrinsic(k, discovery, request)` with the same bounded policy gates |
| Owner checkpoint from an endpoint list | `Checkpoint(plan, acceptedLines)` certifies a complete validated Tikray |
| `PrefixFor` | `SeedFor(binding)` returns detached scratch and validates consumed advances |
| First-fit continuation with separate input/path/geometry | `ContinueParagraph(BreakRequest{Prefix: checkpoint, ...}, params)` |
| Breaker `ParFillSkip` / `DiscretionaryProvider` options | Fill is preparation material (`Config.ParFillSkip`); discovery belongs to the request |
| Khipu breakpoint/source query | `Tikray.BreakpointPositions()` |

First-fit and KP retain their algorithms, tie policies, signed geometry and
hyphenation behavior. Width callbacks are memoized per queried line and released
on return. KP's built-in rectangular optimization remains enabled through that
memoization. Plans retain effective policy/diagnostics, with detached accessors.
First-fit continuation accepts a KP prefix but still requires NoHyphenation.
Full assembly/freezing remains permitted; no page solver is included.

All public typesetting calls are synchronous and context-free. Internal streaming
coordination retains cancellation, draining, terminal errors and producer joining.
Bindings/checkpoints reject foreign incarnations; consumed atomic advances cannot
change under a locked prefix. Frozen results retain stable source/glyph storage,
not material, discovery, bindings, plans, checkpoints or their construction executor.

The existing geometry, shaping, feature, cleanup, hyphenation and algorithm tests
remain active. Mutable metric/corruption fixtures now live in test-only support;
they cannot masquerade as production Khipus or manufacture freeze provenance.
Direct public regressions cover trial isolation across success/failure/rejection,
captured geometry, detached accessors, normalized IDs, concurrent cache publication,
prefix/atomic compatibility, policy attribution and collection of working handles
while frozen content survives. New public examples use the immutable API.

## Verification and dependency identity

Validated with `go test ./...`, `go test -race ./...`, `go build ./...`,
`go vet ./...`, and `go mod tidy -diff`. Tests used the sibling replacements
configured by this module, with these baselines:

- khipu before PR 1: `c3ef1300f6ed3821f306f9d5594614b79ca1b9c0`.
- cords: `58b65a5bc6e7dfe74c338f8f197276c5c9c94867`; its existing uncommitted
  `go.mod` adds `replace github.com/npillmayer/schuko => ../schuko`.
- uax: `c92d5c4593263cec69aeffd7ecc0257fe77c2d8f`, clean checkout.
- typography before documentation updates: `b7f5811f4ee5c20e0574656f8b2d1d8e939131f5`.

The first validation ran in a temporary source copy with those same sibling
replacements; the required checks were also run after applying the patch to
`/Users/npi/prg/go/khipu`. Typography and tytui `go build ./...` both report the
expected unmigrated `glyphing/paragraph` API errors (removed PreparedParagraph,
owner methods/provider options and changed breaker signatures). No dependency
changes or generated proof/replay controls were made.
No new production cost measurement or performance release claim is made by PR 1;
older cost reports describe the preceding pipeline.

## Expected downstream breakage

Typography's paragraph preparation, binding, breakpoint execution, freezing,
intrinsic measurement and continuation still call removed signatures/types.
Its tests and khipu benchmark helpers also use the former mutable planes and
selection/provider seams. They need PR 2. Tytui/tycli build on that typography
pipeline and need PR 3, including replay/proof integration verification. Neither
module is claimed to pass against PR 1. Temporary compilation failure is permitted
by the agreed module sequence; CSS geometry, replay v5 and the geometry-only
equal-width path must be preserved when those consumers migrate.
