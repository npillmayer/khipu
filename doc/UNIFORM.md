# Uniform-Shape State Compaction

## Status And Decision

Enabled automatically in every K&P pass for the built-in rectangular shape,
including public `BreakParagraph` calls. No `LineShape` extension or public
configuration flag is required. The original experiment and its single-pass
measurements are retained below; production coverage now includes complete
multi-pass calls and selection publication.

The fast path recognizes only the concrete shape returned by
`linebreak.RectangularParShape`. Custom shapes, even constant ones, and
non-uniform shapes retain the baseline algorithm. Recognition checks the type,
not a sample of line widths. This keeps the uniformity guarantee explicit
without exposing a new shape capability. A test-only opaque wrapper around the
same rectangle supplies identical widths through the unoptimized public path.

## Scope And Invariants

At an ordinary breakpoint, different completed-line counts can represent
different paths to the same future problem. With uniform geometry, identical
initial segment state and no remaining first-line distinction, the cheaper
accumulated path dominates. Equal costs prefer fewer completed lines, matching
terminal collection. Actual line numbers remain on the winning state.

The implemented policy is deliberately narrower than general state merging:

- Compact only the newly completed ordinary breakpoint bucket, after all knot
  advancement, discretionary discovery, boundary commit and fallback decisions.
- Keep the minimum-cost state, breaking ties by lower completed-line count.
- Do not compact the root or discretionary variants. All compacted states have
  completed at least one line, so first-line extras have already been charged.
- Remove dominated working bookkeeping and seeds through existing retirement;
  invalidate its evaluation-cache bucket and retain predecessor history.
- Keep retirement tombstones. Ordinary transitions to this boundary are never
  proposed again after its knot, so a later improvement cannot be blocked there.

Why not compact discretionary buckets? They can receive retroactive proposals
after descendants have been created. Their immutable rescan seeds and consumed
suffixes also have different lifetimes. The ordinary-boundary experiment avoids
that problem rather than assuming that a general deletion policy is safe.

The implementation shares the existing classifier, pass orchestration,
discretionary machinery, fallback selection and collector. It branches only the
working-state retention policy; it does not duplicate the linebreaker.
`uniform.go` documents the compaction seam and its restrictions. Shape recognition
runs once during each pass's setup; it does not probe `LineLength`.

## Verification

`uniform_test.go` compares exact `BreakRef` sequences (including
variants), total costs, worst badness and completion status against baseline:

- A 144-case matrix: two rectangular widths, an alternating shape and an
  unadvertised constant shape; ordinary/empty/productive provider modes; all
  three pass configurations; internal forced breaks; neutral or intentionally
  negative ordinary penalties. Each optimized case runs twice from fresh input.
- Separate 1,000-word ordinary and productive comparisons.
- Emergency recovery, exhausted non-final passes and unavoidable fallback
  fixtures from the emergency workload suite.
- Unit checks for canonical ties, retained predecessor history, root exclusion
  and discretionary-bucket exclusion. Shape-recognition checks require unknown
  shapes to remain on the baseline path.

`uniform_public_test.go` compares cold and warm public results, selected variants,
pass identity and path quality against the opaque-wrapper baseline. Fixtures
complete in pass 1 with nil parameters, pass 2 with productive discovery, and
pass 3 with recovery or unavoidable fallback. Existing provider-error tests now
also run under automatic dispatch, preserving error propagation and atomic
selection publication. Cache-work regression counts reflect compacted states.

This is synthetic regression evidence, not exhaustive proof for all possible
providers, fonts or scripts. Future line-number-sensitive scoring requires
revisiting the dominance condition before enabling it in the compacted path.

## Original Single-Pass Measurements

November 8, 2026, Apple M1, darwin/arm64, Go 1.25.6. Post-translation Unicode
penalties; deterministic 5 pt glyph cells; 150 pt rectangular layout with the
default side skips. First/final discretionary extras are 75/125 in both arms.
Ordinary workloads measure pass 1; productive workloads measure a final pass 2.
Input encoding and breaker setup are outside timing; providers and caches start
cold per iteration. Construction, discovery/shaping and collection are timed.
Diagnostic counters are enabled in both arms, so these are instrumented
single-pass comparisons, not public-call throughput measurements.

Command (logging disabled):

```sh
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkUniformCompaction$' -benchtime=3x -count=3 -benchmem -timeout=120s
```

Median times from three runs of three iterations each; allocations rounded to
decimal MB. The small sample is noisy, particularly for short runs.

| Words / mode | Baseline ms | Compact ms | Baseline MB/op | Compact MB/op |
| --- | ---: | ---: | ---: | ---: |
| 250 ordinary | 2.37 | 0.65 | 0.81 | 0.30 |
| 1,000 ordinary | 32.39 | 2.73 | 13.24 | 1.22 |
| 250 productive | 36.95 | 14.88 | 7.66 | 5.00 |
| 1,000 productive | 864.45 | 245.34 | 98.76 | 56.78 |

The 1,000-word compact ordinary timings ranged from 2.46 to 4.08 ms;
productive timings ranged from 245.32 to 246.25 ms.

| 1,000-word metric | Ordinary baseline / compact | Productive baseline / compact |
| --- | ---: | ---: |
| Actual state evaluations | 143,799 / 5,420 | 1,690,320 / 497,330 |
| Peak active states | 369 / 8 | 2,172 / 836 |
| Historical predecessor entries | 26,448 / 1,150 | 134,959 / 90,463 |
| Selected lines | 237 / 237 | 212 / 212 |
| Total demerits | 101,081 / 101,081 | 26,855 / 26,855 |

History is not garbage-collected: fewer entries arise because fewer paths are
extended. Ordinary active-state growth is sharply reduced. Productive workloads
still retain multiple line counts at discretionary origins and remain expensive.

## Public-Call Measurements

Production adoption was measured on the same machine with neutral Unicode
penalties, using actual `BreakParagraph` calls. A test-only opaque shape wrapper
is the baseline; its widths are identical to the recognized rectangle. Each
timed call starts with a fresh owner/paragraph and cold candidate caches.
Encoding is excluded; pass selection, pass setup, lazy discovery/shaping,
collection and selected-variant publication are included. Optional `workMetrics`
instrumentation is disabled. Every iteration checks breakpoints and selections
against baseline; untimed diagnostics verify the expected winning pass.

The first-pass case uses nil algorithm parameters. The productive pass-2 case
sets `PreTolerance=0` so the first pass fails and discovery is exercised, while
retaining the standard second-pass defaults. Pass-3 cases use the existing
17 pt/6 pt emergency fixtures. These cases are synthetic workloads, not a
prediction for real-font or international-script performance.

```sh
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkUniformPublic$' -benchtime=3x -count=3 -benchmem -timeout=120s
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkUniformPublic$/^1000$' -benchtime=3x -count=3 -benchmem -timeout=120s
```

The table uses the isolated 1,000-word repeat, without other tests running.
Medians of three runs of three iterations; allocations are decimal MB.

| Public workload | Baseline ms | Fast path ms | Baseline MB/op | Fast path MB/op |
| --- | ---: | ---: | ---: | ---: |
| Pass 1, ordinary | 32.43 | 2.60 | 13.28 | 1.22 |
| Pass 2, productive | 1043.57 | 284.76 | 98.77 | 56.79 |
| Pass 3, productive recovery | 15.23 | 15.26 | 11.86 | 11.86 |
| Pass 3, overfull fallback | 3.82 | 3.73 | 2.68 | 2.68 |

Pass-1 timings ranged from 32.28-32.52 ms / 2.55-2.68 ms; productive pass-2
timings ranged from 1040.72-1044.52 ms / 284.57-288.31 ms. The bounded-state
emergency cases offer essentially no compaction opportunity; their small timing
differences are noise, not evidence of a speedup. Selected line counts and
variants match in every benchmark iteration. The complete-call gain is about
12.5x for ordinary text and 3.7x for the productive fixture in this sample.

Validation for adoption: `go test ./... -count=1 -timeout=90s`, full race tests,
`go vet ./...`, and `git diff --check`.

## Remaining Scope

Unknown shapes remain on the baseline path; a future explicit uniform-shape
capability could broaden eligibility without relying on width sampling. A
separate discretionary-state experiment would need a design for later
improvements and predecessor ownership. Neither is required for the current
ordinary-boundary fast path.
