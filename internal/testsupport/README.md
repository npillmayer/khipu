# testsupport

Test-only support for existing metric and algorithm regression fixtures.
Only tests import this package. Mutable fixture planes are detached from actual
Khipus; they cannot manufacture production shaping/freezing provenance.
`FromBinding` snapshots real immutable input, while hand-built metric fixtures
exercise private arithmetic/search components, including malformed cases.
Public-contract integration tests use the immutable APIs directly.
