# Enterprise readiness

## Required gates

- Conformance: Λ.1–Λ.7 trace matrix and GRE-1001 stage tests.
- Chaos (Dishamory): injected halt, drift, ledger break, provider timeout, and operator interrupt; expected result is halt-and-surface.
- Isolation: tenant state namespaces never cross; lane mismatch is denied by USL.
- Trust bundle parity: compare immutable bundle manifest digests before promotion.
- Audit package: export ledger entries plus one CER per execution and deterministic replay markers.

Current status: scaffold conformance is partial. Docker images for USL, GRE, ledger, and drift are referenced but not built in this repository; promotion is blocked until those images and the CI gates are supplied.
