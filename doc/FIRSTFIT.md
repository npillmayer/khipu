# Firstfit

Current contract, reviewed November 9, 2026. `linebreak/firstfit` is a greedy,
single-pass breaker over the SOA `Khipu`. It chooses line endpoints and selected
discretionary variants; it neither shapes ordinary input nor assigns final widths.

## API And Ownership

```go
func BreakParagraph(khp *khipu.Khipu, layout khipu.Params, params *Parameters) ([]int, error)
```

Resolve geometry once with `firstfit.DefaultLayout(shape)`, apply overrides, and
reuse that `khipu.Params` for freezing. The profile has zero left skip and a
zero-natural-width, zero-shrink, `fil` physical right skip. Explicit zero overrides
are not replaced with defaults. Physical right remains right for RTL paragraphs.

`NewDefaultParameters()` returns independent algorithm settings: zero
`MinHyphenGain` and no `DiscretionaryProvider`. Nil algorithm parameters use that
same profile. Geometry is not part of these settings.

Results are zero-based indices of line-ending knots, including the final trailer.
The encoder supplies a non-discardable, forced paragraph-fill trailer. Selected
discretionaries retain their source split and shaped fragment box IDs; the base
knots and width planes are not rewritten.

Selections are staged locally and replace `Khipu.SelectedDiscretionaries` only
after success. Validation or provider errors return no breakpoints and preserve
earlier selections, including a nil map. A successful ordinary-only run clears
stale selections. Candidate caches may populate during a failed call and remain
available for retry. Providers must not modify the selection map themselves.
This is failure atomicity, not a concurrent-access guarantee for Khipu.

## Algorithm And Fitting

The breaker scans knots in logical order, remembering the latest feasible
checkpoint. When a later candidate overflows, it commits that checkpoint and
restores the already-scanned post-break material from a carry snapshot. The current
candidate is reconsidered against the next line's target, rather than lost.
Forced endpoints always end a line; they bypass discretionary rescue. When no
fitting choice exists, the breaker can return an unavoidable overfull line.

Fit means **effective MinW <= target**, after edge trimming and including side
skips. The same criterion applies to ordinary candidates, carry, forced-boundary
overflow checks and discretionary pre-break candidates. Natural-width overflow
alone is not a reason to break earlier when the supplied knots can shrink enough.

The ragged-right profile does not mean "never compress":

- Positive slack goes to the highest infinite stretch order present. The `fil`
  right skip leaves finite spaces at natural width; final paragraph `fill` wins
  over that skip.
- Negative slack uses finite shrink capacity. The default right skip supplies
  none, so content can shrink only within its explicit bounds.
- Fixed-width content (`MinW = W = MaxW`) automatically uses natural-width
  fitting. Clients can set `SpaceStretchPercent` and `SpaceShrinkPercent` to zero
  at encoding. A `fil` skip may still absorb unused space without changing content.

There is no separate fitting-mode parameter. Breaking chooses endpoints;
freezing assigns widths. The internal freezer still rejects insufficient
capacity: the full planned overfull/underfull result contract remains future work
(see [FREEZE.md](FREEZE.md)).

### Trimming

`Khipu.Flags`, not knot kind, decides discardability. `KFDiscardable` knots trim
at line edges; unflagged knots remain. Interior glue and kern count toward width.
The segment state tracks raw width, leading/trailing trim, whether visible content
has appeared, and a complete carried snapshot. Reset restores that whole state,
so leading trimming works on subsequent lines too. The final fill must remain
non-discardable; there is no terminal exemption from flags.

The encoder's default flag assignment is still simple and requires future
script-/shaping-aware refinement. This policy belongs upstream, not in firstfit.

## Discretionary Rescue

Configure the encoding `Khipukamayuq` as `DiscretionaryProvider` to enable lazy
streaming discovery. Without a provider, existing cached candidates are consulted.
Only an overflowing textbox is considered. Automatic discovery requires an
earlier ordinary checkpoint; a retained explicit marker (such as U+00AD) may
also rescue a first word, using a zero checkpoint-width baseline. Structural
single-word/context eligibility is enforced by the owner.

Ordinary endpoint prohibition does not prohibit an internal discretionary. Both
permitted and prohibited textbox endpoints use one rescue path; mandatory
endpoints do not request candidates. Candidate selection is deliberately local:

1. Reject `PreBreak.Penalty >= InfinityDemerits`.
2. Require the candidate line's effective minimum width to fit.
3. Require natural-width gain over the ordinary checkpoint to reach
   `MinHyphenGain`, inclusively. Default zero permits zero improvement.
4. Prefer the widest natural-width line. Lower finite penalty breaks width ties;
   exact width/penalty ties retain candidate order.

The selected `PostBreak` seeds the next line. If nothing qualifies, ordinary
overflow handling continues. Finite penalties do not compete with width and
`MinHyphenGain` is not a hyphen-frequency guarantee.

Deliberately absent: tight-line hyphenation, automatic first-word rescue without
a checkpoint, multi-box/repeated same-box splitting, consecutive-hyphen limits,
path-dependent penalties, badness/tolerance scoring, global path optimization,
and multiple passes. These are scope decisions, not incomplete migration work.

## Performance And Package Structure

Ordinary traversal is linear in knot count with constant-size segment/carry
bookkeeping, plus output storage. Discretionary work adds candidate enumeration
and provider costs, including source preparation and shaping. Linear traversal
does not imply constant-cost hyphenation. There is no predecessor graph or replay
of the whole paragraph. Debug arguments are constructed only when debug logging
is enabled; the trace is selected once per breaker call.

`linebreak` owns shared `WSS` arithmetic and paragraph shapes (`ParShape` aliases
`khipu.LineShape`). Firstfit owns its parameters, greedy scan and segment state;
Knuth-Plass owns its multipass optimization and scoring. The legacy cursor/AOS
implementations are retired. Further extraction of trimming helpers is optional,
not required for this cleanup.

`BenchmarkFirstfit` covers 16/128/512 words with no provider, a declining provider,
and productive synthetic hyphenation. It times the full public breaking call on
fresh owners/caches, including lazy shaping and selection publication, but excludes
encoding/setup and verification. Counts and warm-run stability are tested outside
the benchmark. See [TESTING.md](TESTING.md) for measurements and reproduction.

## Regression Coverage And Remaining Work

- Checkpoint/carry, forced breaks and fallback, including explicit kern fixtures
  for flagged/unflagged boundaries, interior width and leading carry trimming.
- Discretionary prohibition, inclusive gain boundaries, deterministic ranking,
  both ordinary endpoint classes, and forced-textbox exclusion from rescue.
- Failure-atomic selection publication, cache retention/retry and independent defaults.
- Ordinary breaking through freezing at natural/minimum-width boundaries,
  fixed-space input, finite shrink, fill precedence and LTR/RTL physical skips.
- Disabled logging guards and unchanged results across error/info/debug levels.
- Cold provider workloads and warm cache reuse, with deterministic work counts.

The firstfit cleanup is complete. Future performance work should be driven by
profiles of real workloads, especially font-backed discovery and larger variant
sets. Script-sensitive upstream flags and the remaining freezing result contract
are separate work. No additional firstfit policy or compatibility API is required.
