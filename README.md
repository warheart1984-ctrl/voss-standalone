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

## Governance
Voss Binding Λ.1-Λ.7 enforced via GRE-1001 nine-stage pipeline, USL Gate, Immune Protocol, and unconditional corrigibility.
Ledger is cryptographically chained and continuity-hooked.
