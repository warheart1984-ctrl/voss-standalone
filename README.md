# Voss Standalone Runtime

Provider-agnostic governed AI runtime with Voss Binding Λ.1-Λ.7.

## Quick start
```bash
docker compose up -d
```
Router on :8080, Prometheus :9090, Grafana :3000

## Add a model provider
1. Implement `Adapter` interface: `CapabilityRequest`, `Generate`, `Trace`
2. Register via `router.RegisterProvider("provider-id", adapter)`
3. Add tenant policy in `deploy/voss/config/tenants.production.json` with allowed providers and key_ref
4. USL Gate will enforce capability lattice, MLCA lane, and corrigibility before routing

No default provider. Routing is capability-first and governed.

## Workflows
`deploy/voss/workflow/` contains Governed Intake, OTEM Execution, External Suggestion Admission, Governed Training/Eval.

## Observability
Prometheus metrics: `voss_router_decision_duration_seconds`, `voss_router_admission_total`, `voss_router_provider_error_total`
Grafana dashboards: Binding Invariants, Auditability Traces, USL Gate Decisions

## Governed admission
`POST /route` accepts a `CapabilityRequest` (strict JSON, unknown fields rejected). Outcomes:
`ADMIT` (200), `DENY` (403), `QUARANTINE` (403), or `409` when the intent is interrupted/terminated.
Every decision is appended to the hash-chained ledger and exposed at `GET /ledger`, with chain integrity at
`GET /verify` and Prometheus-style metrics at `/metrics`.

## Operator sovereignty (Phase 3)
Authenticated APIs for interrupt, correction, and termination. Set `VOSS_OPERATOR_TOKEN`; when unset the
operator APIs fail closed (401).
```bash
# Interrupt: prevents provider invocation / wins during an in-flight request
curl -X POST localhost:8080/interrupt -H "Authorization: Bearer $VOSS_OPERATOR_TOKEN" -d '{"intent_id":"intent-1","operator_id":"op-admin"}'
# Correction: records operator directive and halts the intent
curl -X POST localhost:8080/correct -H "Authorization: Bearer $VOSS_OPERATOR_TOKEN" -d '{"intent_id":"intent-1","operator_id":"op-admin","directive":"re-plan training set"}'
# Termination: terminal. Subsequent attempts for the intent conflict forever.
curl -X POST localhost:8080/terminate -H "Authorization: Bearer $VOSS_OPERATOR_TOKEN" -d '{"intent_id":"intent-1","operator_id":"op-admin"}'
```
Execution state machine: `PENDING -> ADMITTED -> RUNNING`, and `HALTED` / `TERMINATED` are terminal.
Operator interrupts, corrections, and terminations take precedence at every gate (Λ.6) and are ledgered.

## Governance
Voss Binding Λ.1-Λ.7 enforced via GRE-1001 nine-stage pipeline, USL Gate, Immune Protocol, and unconditional corrigibility.
Ledger is cryptographically chained and continuity-hooked.
