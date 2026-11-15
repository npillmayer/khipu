# Implementation History And Proposals

These records preserve rationale, superseded designs and evidence from completed
work. Their historical use of “current” or “implemented” refers to the recorded
stage. They are not client setup instructions or a promise to implement proposals.
Start with the [documentation guide](../README.md) for supported contracts.

| Record | What it preserves | Current reference |
| --- | --- | --- |
| [Pipeline integration](INTEGRATION_HISTORY.md) | Cross-repository ownership, stages 1–5, validation narratives and the original API proposal. | [Pipeline design](../PIPELINE_INTEGRATION.md), [prepared paragraphs](../PREPARED_PARAGRAPHS.md) |
| [Frozen API delivery](FROZEN_API_DELIVERY.md) | Completed geometry migration, result API and pipeline verification steps. | [Result contract](../FROZEN-API.md), [freezing](../FREEZE.md) |
| [Scoring history and proposal](SCORING_DESIGNS.md) | Arithmetic cleanup stages, removed `kp` behavior and an unimplemented fitness-class design. | [Current scoring](../KNUTH_PLASS_SCORING.md) |
| [Project background](BACKGROUND.md) | The name, knot metaphor and original conceptual illustration. | [Quick start](../../README.md#encode-and-break-a-paragraph) |
| [Client-readiness review](../REVIEW.md) | Original review findings with dated follow-up status. Line references belong to the reviewed snapshot. | [Documentation guide](../README.md) |

The integration record's sibling-checkout links require the original developer
workspace. They are retained for local source archaeology, not as portable
dependencies of the client guides. Its cross-repository test results record
specific implementation runs; use each repository's current documentation before
integrating another project. The background illustration is remotely hosted.
