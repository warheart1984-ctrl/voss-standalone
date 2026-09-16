# Runbook: Fail-Closed Governance

All governance faults fail closed: a request with unknown/malformed/unavailable
governance state is never admitted and never reaches a provider.

## Faults and denied outcomes

| Fault                          | Decision    | Rule ref          |
|--------------------------------|-------------|-------------------|
| Unknown tenant                 | DENY        | `Lambda.4`        |
| Lane mismatch                  | DENY        | `Lambda.4`        |
| Unbound sovereign operator     | DENY        | `Lambda.4`        |
| Capability not granted         | DENY        | `capability.lattice` |
| Tenant policy disallows        | DENY        | `tenant.capability_policy` |
| Empty/malformed payload        | QUARANTINE  | `immune.protocol` |
| Drift spike over threshold     | DENY        | `Lambda.5`        |
| Ledger chain integrity broken  | DENY        | `ledger.integrity` |
| Operator interrupt / terminate | DENY        | `Lambda.6` (HTTP 409) |
| Governance enforcement off     | DENY        | `Lambda.7`        |

## Symptom: a single request denied that should pass

1. Read `GET /ledger` and match the entry by `request_id`.
2. The `rule_ref` names the failing gate; the `stage_log` names the stage.
3. Fix the request side (tenant exists, lane aligns, sovereign operator matches)
   or the policy side, never the enforcement.
4. Denied paths never invoke the provider — confirm zero `generate` calls in
   metrics (`voss_router_provider_error_total`, absent executions).

## Symptom: provider errors increasing

Provider failures are governed DENYs with `provider.execution`. Verify the
queue/adapters are healthy upstream; Voss does not retry a denied transition.

## Key invariant

> Fail closed always. Surfaces as DENY/QUARANTINE with a named `rule_ref` and a
> ledger entry. Continuation after a governance fault is a bug.