# Performance

Benchmarks for encoding and line breaking: what they measure, how to run them,
and current results. Complexity characteristics of the breakers are described
in [KNUTH+PLASS.md](KNUTH+PLASS.md#performance-characteristics) and
[FIRSTFIT.md](FIRSTFIT.md#performance-characteristics); test coverage in
[TESTING.md](TESTING.md).

All workloads use synthetic shaping (`Config{}`: 5 pt glyph cells at 10 pt, or
the cell size stated) and deterministic test hyphenators. They measure the
algorithms and their bookkeeping, not real font shaping, dictionary
hyphenation or international scripts.


## Benchmarks

### Knuth-Plass (`linebreak/knuthplass`)

| Benchmark | File | Timed scope | Workload |
|---|---|---|---|
| `BenchmarkEncodeLorem` | `benchmark_test.go` | `EncodeParagraph` | lorem corpus, 250/500/1,000 words |
| `BenchmarkBreakLorem` | `benchmark_test.go` | public `BreakParagraph` | lorem corpus, 150 pt, default parameters, no provider |
| `BenchmarkFirstPassLorem` | `benchmark_test.go` | one pass 1, including collection | as above |
| `BenchmarkSecondPassHyphenatingLorem` | `hyphenation_benchmark_test.go` | one final pass 2 with cold discovery | lorem corpus, 50/100/250 words, midpoint hyphenator |
| `BenchmarkRetirementWorkloads` | `workload_benchmark_test.go` | one final pass | matrix: 50/250/1,000 words × 5 geometries × 3 provider modes |
| `BenchmarkUniformCompaction` | `uniform_test.go` | one pass, compaction on/off | lorem corpus, 250/1,000 words, ordinary/productive |
| `BenchmarkUniformPublic` | `uniform_public_test.go` | public `BreakParagraph`, built-in vs. opaque rectangle | first-pass, second-pass and emergency fixtures |
| `BenchmarkEmergencyWorkloads` | `emergency_benchmark_test.go` | public call, or isolated pass 3 | fixtures that require pass 3, 16/128/512 words |

### First-Fit (`linebreak/firstfit`)

| Benchmark | File | Timed scope | Workload |
|---|---|---|---|
| `BenchmarkFirstfit` | `benchmark_test.go` | public `BreakParagraph` | 16/128/512 words, no/declining/productive provider |

### Workloads

- **Lorem corpus.** A fixed 19-word ASCII text repeated to the requested length,
  with a trailing space; `2·words + 1` knots. Unless stated otherwise:
  `DefaultLayout` at 150 pt and `NewDefaultParameters()`.
- **Midpoint hyphenator.** Offers one split in the middle of every ASCII word
  of at least four characters and declines shorter words. A reproducible
  workload, not a linguistic policy.
- **Provider modes** (`BenchmarkRetirementWorkloads`): *ordinary* (no
  discovery), *empty* (the hyphenator declines everything) and *productive*
  (midpoint hyphenator).
- **Geometries** (`BenchmarkRetirementWorkloads`): *narrow* 100 pt, *standard*
  150 pt, *wide* 300 pt, *alternating* 150/100 pt (a custom shape, so no
  uniform-shape compaction), *forced* 150 pt with every twentieth interword
  space turned into a mandatory break. All cases run one final pass with
  tolerance 200; hyphenating cases keep the default discovery trigger (100).
- **Emergency fixtures** (`BenchmarkEmergencyWorkloads`): 17 pt lines, zero side
  skips, emergency stretch 6 pt.

  | Fixture | Input and provider | Outcome |
  |---|---|---|
  | `loose-none` | repeated `ab `, no provider | loose lines accepted thanks to emergency stretch |
  | `loose-declining` | same, provider declines | same, with discovery requests |
  | `split-productive` | repeated `abcd `, midpoint split | every word hyphenated, no fallback |
  | `overfull-declining` | repeated `abcd `, provider declines | every word overfull, fallback lines |

  *Public* times the whole three-pass call; *isolated* times pass 3 alone,
  with passes 1 and 2 run untimed beforehand on the same paragraph.
- **First-fit workload.** Repeated `abcd` words, 40 pt lines; alternatives
  split after two letters and add a hyphen to the first fragment.

Encoding, owner construction and result checks are outside the timed region,
except in `BenchmarkEncodeLorem`. Every iteration starts with a fresh owner and
paragraph, so discovery is cold. Lazy fragment shaping during breaking is timed.


## Running

Benchmarks refuse to run with info or debug tracing enabled. Use `-run '^$'`
so that no test changes the trace level first.

```sh
go test ./linebreak/knuthplass -run '^$' -bench . -benchtime=3x -count=3 -benchmem -timeout=30m
go test ./linebreak/firstfit   -run '^$' -bench . -benchtime=3x -count=3 -benchmem
```

Select single cases with anchored sub-benchmark patterns, so that `50` does not
also match `250`:

```sh
go test ./linebreak/knuthplass -run '^$' -bench '^BenchmarkRetirementWorkloads$/^1000$/^standard$/^productive$' -benchtime=3x -benchmem
```

The workload tests (`TestEmergencyBenchmarkWorkloads`,
`TestFirstfitBenchmarkWorkloads` and others) run as ordinary tests and pin the
expected work counts without timing.

### Profiling

Keep the test binary next to the profiles, outside the checkout:

```sh
go test ./linebreak/knuthplass -run '^$' \
  -bench '^BenchmarkRetirementWorkloads$/^1000$/^standard$/^productive$' -benchtime=3x \
  -cpuprofile=/tmp/khipu.cpu -memprofile=/tmp/khipu.mem -o /tmp/khipu.test
go tool pprof -top -cum /tmp/khipu.test /tmp/khipu.cpu
go tool pprof -top -alloc_space /tmp/khipu.test /tmp/khipu.mem
```

Profiles are process-wide and include the untimed encoding and setup; use them
for attribution, not for absolute costs. If `go tool pprof` is unavailable,
build it with `go build -o /tmp/khipu-pprof cmd/pprof`.

### Work Counters

Several benchmarks report counters from a private, optional instrumentation of
one breaker pass (`metrics.go`). Public calls run without it. Counts describe
one call, not totals over all iterations.

| Counter | Meaning |
|---|---|
| `peak-active-states` | maximum number of live working states `(BreakRef, line)` during a pass |
| `ordinary-evals/op` | line classifications from ordinary states |
| `ordinary-reused/op` | classifications avoided by reusing results within a boundary |
| `discretionary-evals/op` | classifications of discretionary alternatives |
| `state-evals/op` | sum of ordinary and discretionary classifications |
| `provider-requests/op` | calls of the breaker's `DiscretionaryProvider` (including cache hits) |
| `hyphenator-calls/op`, `policy-calls/op` | calls reaching the `Hyphenator` policy |
| `predecessors` | predecessor entries kept at the end of the pass |
| `retained-states` | working states left at the end (not a peak) |

In `BenchmarkEmergencyWorkloads`, public counters are summed over the passes,
except the peak, which is the maximum.


## Current Results

Measured on 2026-11-09, Apple M1, Go 1.25.6, darwin/arm64, with
`-benchtime=3x -count=3`; times are medians of the three samples. They are
indications for comparing workloads and changes, not latency guarantees.

### Encoding and Ordinary Breaking

| Words | Encode | Break (public) | Encode allocations | Break allocations |
|---:|---:|---:|---:|---:|
| 250 | 10.3 ms | 0.60 ms | 10.1 MB, 74,481 | 0.30 MB, 552 |
| 500 | 20.9 ms | 1.29 ms | 20.4 MB, 149,277 | 0.61 MB, 1,058 |
| 1,000 | 46.9 ms | 2.57 ms | 41.3 MB, 298,673 | 1.22 MB, 2,073 |

Both scale linearly. For ordinary text, encoding (with synthetic shaping) costs
about 18 times as much as breaking, with roughly 300 allocations per word.

### Knuth-Plass with Hyphenation

`BenchmarkSecondPassHyphenatingLorem` (one cold pass 2, 150 pt):

| Words | Time | Peak states | Classifications | Provider requests | Hyphenator calls |
|---:|---:|---:|---:|---:|---:|
| 50 | 1.12 ms | 37 | 1,569 | 485 | 49 |
| 100 | 3.07 ms | 73 | 5,504 | 1,709 | 99 |
| 250 | 14.7 ms | 196 | 32,090 | 10,108 | 249 |

`BenchmarkRetirementWorkloads`, 1,000 words (one final pass):

| Geometry | Ordinary | Empty | Productive | Peak states (ordinary / productive) |
|---|---:|---:|---:|---:|
| narrow (100 pt) | 2.27 ms | 5.25 ms | 1,206 ms | 7 / 2,361 |
| standard (150 pt) | 2.68 ms | 6.11 ms | 244 ms | 8 / 836 |
| wide (300 pt) | 4.66 ms | 8.54 ms | 102 ms | 14 / 303 |
| alternating (custom) | 103 ms | 124 ms | 1,061 ms | 1,025 / 2,600 |
| forced (150 pt) | 2.04 ms | 4.99 ms | 18.6 ms | 8 / 27 |

The same matrix at 250 words: standard productive 14.8 ms (peak 196), narrow
productive 44.1 ms (594), alternating ordinary 6.43 ms (251).

### Uniform-Shape Compaction

`BenchmarkUniformCompaction` (one pass, 150 pt):

| Words | Mode | Without | With | Peak states without / with |
|---:|---|---:|---:|---:|
| 250 | ordinary | 2.35 ms | 0.66 ms | 93 / 8 |
| 250 | productive | 36.5 ms | 14.8 ms | 492 / 196 |
| 1,000 | ordinary | 32.5 ms | 2.57 ms | 369 / 8 |
| 1,000 | productive | 840 ms | 244 ms | 2,172 / 836 |

`BenchmarkUniformPublic` shows the same effect on public calls (1,000 words:
31.7 → 2.65 ms for the first-pass fixture, 1,026 → 278 ms for the second-pass
fixture). Emergency fixtures keep only two to four states and are unaffected.

### Emergency Pass

`BenchmarkEmergencyWorkloads`, 512 words:

| Fixture | Public | Isolated pass 3 | Peak states | Fallback lines |
|---|---:|---:|---:|---:|
| `loose-none` | 0.82 ms | 0.77 ms | 3 | 0 |
| `loose-declining` | 1.71 ms | 1.66 ms | 3 | 0 |
| `split-productive` | 7.21 ms | 7.22 ms | 4 | 0 |
| `overfull-declining` | 1.66 ms | 1.56 ms | 2 | 512 |

Passes 1 and 2 fail after a few evaluations on these fixtures, so pass 3
dominates. All four scale linearly from 16 to 512 words.

### First-Fit

`BenchmarkFirstfit`:

| Words | No provider | Declining | Productive | Selections (productive) |
|---:|---:|---:|---:|---:|
| 16 | 0.006 ms | 0.044 ms | 0.156 ms | 5 |
| 128 | 0.018 ms | 0.227 ms | 0.811 ms | 43 |
| 512 | 0.061 ms | 1.08 ms | 3.37 ms | 171 |

Ordinary breaking allocates almost nothing (7 allocations at 512 words);
discovery and fragment shaping dominate the provider cases.


## Observations

- **Ordinary breaking is linear and cheap** for rectangular shapes, in both
  breakers. Encoding is the larger cost for ordinary paragraphs.
- **Custom line shapes are superlinear.** Without uniform-shape compaction, the
  number of line counts per breakpoint grows with the paragraph (alternating
  shape: 1,025 peak states at 1,000 words).
- **Productive hyphenation is superlinear** in Knuth-Plass: discretionary
  breakpoints keep all their line counts, and narrow lines widen the set of
  live states (2,361 peak states at 100 pt).
- **Provider requests far exceed hyphenator calls** (about 160 per word for
  1,000 words, standard width). Repeated requests are answered from the cache,
  but each still costs a call.
- Paragraphs that need pass 2 or 3 pay for every earlier pass as well.
