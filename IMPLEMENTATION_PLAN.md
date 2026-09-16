# Voss Standalone Runtime — implementation plan

## Objective

Deliver a locally runnable, provider-agnostic Voss runtime in which every execution is capability-bounded, operator-interruptible, fail-closed, cryptographically chained, tenant-isolated, and observable.

## Ground rules

1. Preserve existing files and tests unless a change is required to make behavior correct.
2. No provider call may occur before Immune Protocol and USL admission.
3. Every state transition emits an evidence envelope and appends to the ledger.
4. Operator interrupt, correction, and termination have precedence over agents.
5. Unknown, malformed, or unavailable governance state fails closed.
6. Do not claim external standards or proof bundles are verified without a source URL, digest, and test evidence.

## Phase 1 — make the Go runtime testable

- Add request/response types with JSON tags and strict decoding.
- Move runtime construction into testable components; keep `main` as wiring only.
- Add an in-memory or file-backed ledger interface.
- Implement ledger verification, chain-break detection, export, CER generation, and replay markers.
- Add Prometheus counters/histograms for cycle latency, stage timing, USL decisions, drift, halts, interrupts, and ledger integrity.

Acceptance: `go test ./...` passes; a ledger export can be independently verified after the process exits.

## Phase 2 — implement governance behavior

- Define Λ.1–Λ.7 as typed checks with named evidence.
- Implement GRE-1001 as nine explicit stages:
  `input_validation`, `capability_check`, `identity_separation`, `usl_gate`, `drift_check`, `operator_corrigibility`, `audit_trail`, `ledger_write`, `output_validation`.
- Implement stage input/output envelopes and halt-and-surface behavior.
- Implement USL capabilities as `{class, scope, action, risk}` and tenant/lane lattice grants.
- Return admission outcomes `ADMIT`, `DENY`, and `QUARANTINE`, each with reason, rule reference, and decision ID.
- Implement Immune Protocol outcomes `ALLOW`, `CLAMP`, `REROUTE`, `REJECT`, `QUARANTINE` before model calls.

Acceptance: unit tests cover every Λ failure path, every USL outcome, lane mismatch, unknown tenant, missing operator, malformed request, and immune classification.

## Phase 3 — operator supremacy

- Add authenticated APIs for interrupt, correction, and termination.
- Add execution state machine with legal transitions and a terminal `halted` state.
- Ensure checks occur before and during model execution.
- Measure interrupt latency and test that an interrupt wins during an in-flight request.

Acceptance: an operator interrupt prevents provider invocation or terminates an active execution; the result is ledgered.

## Phase 4 — provider and workflow services

- Implement a stateless model-adapter interface with a deterministic fake adapter for tests and an optional Grok-compatible adapter.
- Add read-only bounded Coherence Projection context injection.
- Implement four service workflows: governed intake, OTEM execution, external suggestion admission, governed training/eval.
- Route all workflow transitions through the same USL/GRE/ledger path.

Acceptance: integration tests exercise all four workflows and verify no direct provider call bypasses governance.

## Phase 5 — infrastructure and configuration

- Replace placeholder Compose images with buildable local services or clearly documented external image requirements.
- Add gateway, runtime core, USL, GRE, ledger, drift detector, Prometheus, and Grafana health checks.
- Wire governed secrets through environment or secret files; never commit credentials.
- Validate Prometheus and Grafana provisioning during CI.
- Populate tenant policies with explicit lane, lattice slice, identity namespace, and sovereign operator.

Acceptance: `docker compose config` succeeds and `docker compose up --build` reaches healthy services on a clean machine.

## Phase 6 — evidence and resilience

- Add conformance tests for Λ laws and an auditability trace matrix.
- Add Dishamory chaos tests: provider timeout, drift spike, ledger break, malformed input, USL outage, and operator interrupt.
- Add multi-tenant isolation tests and lane-boundary tests.
- Add trust-bundle parity checks using manifest digests.
- Add runbooks for fail-closed, drift surfacing, ledger recovery, and operator sovereignty.

Acceptance: CI fails on any unexpected continuation after a governance fault and publishes test artifacts plus CER/ledger exports.

## CI gates

Run, in order:

```text
go test ./...
go vet ./...
go test -race ./...
docker compose config
JSON/schema validation
Prometheus rule/config validation
conformance and chaos suites
```

## Handoff instructions for OpenCode

Implement in phases, keeping commits small and descriptive. Start with Phase 1 and Phase 2. Do not leave stubs that return unconditional success. If an external dependency is unavailable, provide a deterministic local substitute and document the boundary. Return the changed-file list, test output, and any intentionally blocked acceptance criteria.

## Definition of done

The runtime passes the CI gates, demonstrates an admitted and rejected execution, demonstrates operator interruption, verifies a ledger export, isolates two tenants, and shows the corresponding Prometheus/Grafana evidence.
