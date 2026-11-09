# Documentation Guide

The AOS-to-SOA migration is complete. This index separates implemented contracts
from historical investigations and future designs. Reviewed November 6, 2026.

## Implemented Contracts

| Document | Scope |
| --- | --- |
| [Project README](../README.md) | Runnable encoding/linebreaking example, configuration, and local setup |
| [Service contracts](../contracts/ReadMe.md) and [default helpers](../helpers/ReadMe.md) | Package ownership, dependency direction, and reusable deterministic implementations |
| [Migration guide](MIGRATION.md) | Removed APIs, current source/cluster/trailer contracts, coverage map, and validation limits |
| [Knuth–Plass](KNUTH+PLASS.md) | Contract, algorithm, passes, retirement, emergency pass, uniform-shape compaction, parameters and limits |
| [First-fit](FIRSTFIT.md) | Contract, greedy algorithm, fitting, trimming, discretionary rescue and limits |
| [Hyphenation](HYPHENATION.md) | Provider ownership, fragment shaping, cache/selection behavior, and provenance limits |
| [Demerits](DEMERITS.md) | Implemented cost formulas and differences from TeX; its fitness-class section is a proposal |
| [Performance](PERFORMANCE.md) | Benchmarks, workloads, how to run and profile them, and current measurements |

[Public Go examples](../example_test.go) are compiled and executed by `go test`.
The two breakers share paragraph shapes and elastic-width summaries, but have
separate parameters and policies. Neither materializes or renders final lines.

## Design History And Future Work

- [Testing strategy and defaults](TESTING.md): current guarantees with source and
  test evidence, synthetic default services, and proposed reusable fixtures and
  conformance suites.
- [Overall algorithm](KHIPU_ALGORITHM.md): architectural pseudocode, including
  implemented streaming shaping and future freezing and rendering.
- [Freezing](FREEZE.md): an unimplemented design for line materialization and
  final width allocation. No `FrozenKhipu` or freeze entry point exists.
- [Knuth–Plass paper](knuthplass.pdf): reference material used by the cost-design
  notes, not an implementation or a claim of full TeX parity.

`StreamingShaper` and font/style interfaces are integrated into encoding and
discretionary reshaping. Synthetic helpers do not supply real font shaping.
The metric-only `Shaper` interface has been removed; see the [migration guide](MIGRATION.md).
